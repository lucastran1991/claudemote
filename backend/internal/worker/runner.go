package worker

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/mac/claudemote/backend/internal/config"
	"github.com/mac/claudemote/backend/internal/model"
	"github.com/mac/claudemote/backend/internal/repository"
)

// runJob is the core subprocess lifecycle executed by each worker goroutine.
// It transitions the job to running, spawns the claude subprocess, streams
// stdout/stderr through the log writer, then resolves the terminal status.
func runJob(
	parentCtx context.Context,
	jobID string,
	cfg *config.Config,
	jobRepo *repository.JobRepository,
	logRepo *repository.JobLogRepository,
	reg *Registry,
	hub SSEPublisher,
) {
	job, err := jobRepo.FindByID(jobID)
	if err != nil {
		log.Printf("runner: job %s not found: %v", jobID, err)
		return
	}
	if job.Status != "pending" {
		log.Printf("runner: job %s is already %s, skipping", jobID, job.Status)
		return
	}

	now := time.Now()
	job.Status = "running"
	job.StartedAt = &now
	if err := jobRepo.Update(job); err != nil {
		log.Printf("runner: could not mark job %s running: %v", jobID, err)
		return
	}
	log.Printf("runner: running jobID=%s", jobID)

	timeout := time.Duration(cfg.JobTimeoutMin) * time.Minute
	jobCtx, cancel := context.WithTimeout(parentCtx, timeout)
	reg.Set(jobID, cancel)
	defer func() {
		cancel()
		reg.Remove(jobID)
	}()

	cmd := buildCmd(jobCtx, cfg, job)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		finishJob(jobRepo, job, "failed", -1, fmt.Sprintf("stdout pipe: %v", err))
		return
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		finishJob(jobRepo, job, "failed", -1, fmt.Sprintf("stderr pipe: %v", err))
		return
	}

	if startErr := cmd.Start(); startErr != nil {
		log.Printf("runner: cmd.Start failed for job %s: %v", jobID, startErr)
		finishJob(jobRepo, job, "failed", -1, fmt.Sprintf("start error: %v", startErr))
		return
	}

	lw := newLogWriter(jobID, logRepo, hub)

	// Drain stderr concurrently; keep a tail for error summaries.
	var (
		stderrMu   sync.Mutex
		stderrTail strings.Builder
		stderrWG   sync.WaitGroup
	)
	stderrWG.Add(1)
	go func() {
		defer stderrWG.Done()
		s := bufio.NewScanner(stderrPipe)
		s.Buffer(make([]byte, 0, 1<<14), 4<<20)
		for s.Scan() {
			line := s.Text()
			lw.Append("stderr", line)
			stderrMu.Lock()
			stderrTail.WriteString(line)
			stderrTail.WriteByte('\n')
			stderrMu.Unlock()
		}
	}()

	parseRes, parseErr := ParseStream(stdoutPipe, lw)

	stderrWG.Wait()
	_ = cmd.Wait()

	// Flush all logs to DB before updating terminal job status — guarantees
	// log ordering is consistent and logs are visible before the job row flips.
	if flushErr := lw.Flush(); flushErr != nil {
		log.Printf("runner: log flush error for job %s: %v", jobID, flushErr)
	}

	exitCode := exitCodeOf(cmd)
	status, summary := resolveOutcome(jobCtx, parseRes, parseErr, exitCode, cfg.MaxCostPerJobUSD, stderrTail.String(), job)

	log.Printf("runner: job %s finished status=%s exitCode=%d", jobID, status, exitCode)
	finishJob(jobRepo, job, status, exitCode, summary)
}

// resolveOutcome maps parse results + context cancellation state to a terminal
// status string and display summary. Populates job summary fields from the
// result event when available so finishJob can persist them in one update.
func resolveOutcome(
	jobCtx context.Context,
	parseRes *ParseResult,
	parseErr error,
	exitCode int,
	maxCostUSD float64,
	stderrTail string,
	job *model.Job,
) (status, summary string) {
	switch jobCtx.Err() {
	case context.Canceled:
		return "cancelled", "cancelled"
	case context.DeadlineExceeded:
		return "failed", "timeout"
	}

	if parseErr != nil {
		return "failed", fmt.Sprintf("scan error: %v", parseErr)
	}

	if parseRes.Final != nil {
		re := parseRes.Final
		job.SessionID = re.SessionID
		job.DurationMs = re.DurationMs
		job.TotalCostUSD = re.TotalCostUSD
		job.NumTurns = re.NumTurns
		job.IsError = re.IsError
		job.StopReason = re.StopReason

		if re.TotalCostUSD > maxCostUSD {
			return "failed", "cost-cap-exceeded"
		}
		if re.IsError {
			return "failed", truncate(re.Result, maxSummaryLen)
		}
		return "done", truncate(re.Result, maxSummaryLen)
	}

	if exitCode == 0 {
		return "failed", "no-result-event"
	}
	return "failed", truncate(stderrTail, stderrTailLen)
}
