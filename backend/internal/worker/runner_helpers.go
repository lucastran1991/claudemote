package worker

import (
	"context"
	"log"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/mac/claudemote/backend/internal/config"
	"github.com/mac/claudemote/backend/internal/model"
	"github.com/mac/claudemote/backend/internal/repository"
)

const (
	maxSummaryLen   = 2000
	stderrTailLen   = 500
	costCapExitCode = 137 // sentinel exit code reported when cost cap is exceeded
)

// buildCmd constructs the claude subprocess with a safe environment and its
// own process group (so SIGTERM reaches all child processes on cancellation).
func buildCmd(jobCtx context.Context, cfg *config.Config, job *model.Job) *exec.Cmd {
	cmd := exec.CommandContext(jobCtx,
		cfg.ClaudeBin,
		"-p", job.Command,
		// --verbose is required when combining -p with --output-format=stream-json;
		// without it claude 2.1.x exits 1 with "requires --verbose" before producing
		// any output.
		"--verbose",
		"--output-format", "stream-json",
		"--permission-mode", cfg.ClaudePermissionMode,
		"--model", job.Model,
	)
	cmd.Dir = cfg.WorkDir
	// Own process group so the cancel signal propagates to all child processes.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// Override CommandContext's default SIGKILL with SIGTERM to the entire
	// process group; WaitDelay gives 5 s for clean shutdown before force-kill.
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}
	cmd.WaitDelay = 5 * time.Second
	cmd.Env = safeEnv()
	return cmd
}

// finishJob writes the terminal state back to the DB.
// DurationMs is computed from StartedAt only when StartedAt is set.
func finishJob(
	jobRepo *repository.JobRepository,
	job *model.Job,
	status string,
	exitCode int,
	summary string,
) {
	now := time.Now()
	job.Status = status
	job.ExitCode = &exitCode
	job.Summary = summary
	job.FinishedAt = &now
	if job.StartedAt != nil {
		job.DurationMs = int(now.Sub(*job.StartedAt).Milliseconds())
	}
	if err := jobRepo.Update(job); err != nil {
		log.Printf("runner: failed to persist final state for job %s: %v", job.ID, err)
	}
}

// safeEnv builds a filtered environment for the claude subprocess.
// Only PATH, HOME, ANTHROPIC_API_KEY and any CLAUDE_* vars are forwarded,
// preventing JWT_SECRET, DB_PATH, ADMIN_* etc. from leaking into Claude.
func safeEnv() []string {
	var allowed []string
	for _, kv := range os.Environ() {
		key := kv
		if idx := strings.Index(kv, "="); idx >= 0 {
			key = kv[:idx]
		}
		if isAllowedEnvKey(key) {
			allowed = append(allowed, kv)
		}
	}
	return allowed
}

func isAllowedEnvKey(key string) bool {
	switch key {
	case "PATH", "HOME", "ANTHROPIC_API_KEY":
		return true
	}
	return strings.HasPrefix(key, "CLAUDE_")
}

// exitCodeOf extracts the process exit code; returns -1 if unavailable.
func exitCodeOf(cmd *exec.Cmd) int {
	if cmd.ProcessState != nil {
		return cmd.ProcessState.ExitCode()
	}
	return -1
}

// truncate shortens s to at most n bytes (byte-level cut, safe for DB storage).
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
