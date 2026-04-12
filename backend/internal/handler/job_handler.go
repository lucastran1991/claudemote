package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mac/claudemote/backend/internal/service"
	"github.com/mac/claudemote/backend/pkg/response"
)

// EnqueueRequest is the payload for POST /api/jobs.
type EnqueueRequest struct {
	// Command is the free-text prompt sent to Claude Code.
	// Cap at 16 000 bytes (16 KB) — ample for any real prompt, prevents disk-fill DoS.
	Command string `json:"command" binding:"required,max=16000"`
	Model   string `json:"model"` // optional; defaults to config value
}

// JobHandler handles job CRUD HTTP requests.
// Phase 01: handlers wire through service/repo but return real DB data.
// Phase 02 will add worker pool invocation inside JobService.Enqueue.
type JobHandler struct {
	jobService *service.JobService
}

// NewJobHandler creates a JobHandler wired to the given JobService.
func NewJobHandler(jobService *service.JobService) *JobHandler {
	return &JobHandler{jobService: jobService}
}

// List handles GET /api/jobs — returns all jobs ordered by created_at DESC.
func (h *JobHandler) List(c *gin.Context) {
	jobs, err := h.jobService.List()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to list jobs")
		return
	}
	response.OK(c, jobs)
}

// Enqueue handles POST /api/jobs — creates a new pending job and dispatches it
// to the worker pool. Returns 202 Accepted immediately; the job runs asynchronously.
func (h *JobHandler) Enqueue(c *gin.Context) {
	var req EnqueueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	job, err := h.jobService.Enqueue(req.Command, req.Model)
	if err != nil {
		if errors.Is(err, service.ErrQueueFull) {
			response.Error(c, http.StatusServiceUnavailable, "worker queue is full, try again later")
			return
		}
		response.Error(c, http.StatusInternalServerError, "failed to enqueue job")
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"data": job})
}

// Get handles GET /api/jobs/:id — returns a single job by uuid.
func (h *JobHandler) Get(c *gin.Context) {
	id := c.Param("id")

	job, err := h.jobService.Get(id)
	if err != nil {
		if errors.Is(err, service.ErrJobNotFound) {
			response.Error(c, http.StatusNotFound, "job not found")
			return
		}
		response.Error(c, http.StatusInternalServerError, "failed to get job")
		return
	}

	response.OK(c, job)
}

// Cancel handles POST /api/jobs/:id/cancel — cancels a pending or running job.
func (h *JobHandler) Cancel(c *gin.Context) {
	id := c.Param("id")

	if err := h.jobService.Cancel(id); err != nil {
		if errors.Is(err, service.ErrJobNotFound) {
			response.Error(c, http.StatusNotFound, "job not found")
			return
		}
		if errors.Is(err, service.ErrJobNotCancellable) {
			response.Error(c, http.StatusConflict, "job is already finished")
			return
		}
		response.Error(c, http.StatusInternalServerError, "failed to cancel job")
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
