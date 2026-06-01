package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/config"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/controlplane"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/queue"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/taskrun"
)

// QueueDrainController starts/stops the O3 queue-drain consumer workflow (A3.4).
// The Temporal backend satisfies it; it is an interface so the mux does not depend
// on the backend package directly.
type QueueDrainController interface {
	StartQueueDrainConsumer(ctx context.Context, config queue.QueueDrainConfig) (string, error)
	StopQueueDrainConsumer(ctx context.Context) error
}

func NewMux(cfg config.Config, service *controlplane.Service, taskService *taskrun.Service, consumer QueueDrainController) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		handleHealth(w, r, cfg, service)
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		handleHealth(w, r, cfg, service)
	})
	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		handleJobsList(w, r, "/jobs", service)
	})
	mux.HandleFunc("/api/v1/jobs", func(w http.ResponseWriter, r *http.Request) {
		handleJobsList(w, r, "/api/v1/jobs", service)
	})
	mux.HandleFunc("/jobs/", func(w http.ResponseWriter, r *http.Request) {
		handleJobDetail(w, r, "/jobs/", service)
	})
	mux.HandleFunc("/api/v1/jobs/", func(w http.ResponseWriter, r *http.Request) {
		handleJobAPIRoute(w, r, service)
	})
	mux.HandleFunc("/api/v1/tasks", func(w http.ResponseWriter, r *http.Request) {
		handleTasksList(w, r, taskService)
	})
	mux.HandleFunc("/api/v1/worktrees", func(w http.ResponseWriter, r *http.Request) {
		handleWorktreesList(w, r, taskService)
	})
	mux.HandleFunc("/api/v1/worktrees/", func(w http.ResponseWriter, r *http.Request) {
		handleWorktreeAPIRoute(w, r, taskService)
	})
	mux.HandleFunc("/api/v1/repos", func(w http.ResponseWriter, r *http.Request) {
		handleReposList(w, r, cfg)
	})
	mux.HandleFunc("/api/v1/queue/consumer/", func(w http.ResponseWriter, r *http.Request) {
		handleQueueConsumerRoute(w, r, cfg, consumer)
	})
	mux.HandleFunc("/api/v1/tasks/", func(w http.ResponseWriter, r *http.Request) {
		handleTaskAPIRoute(w, r, taskService)
	})
	mux.HandleFunc("/api/v1/task-runs/", func(w http.ResponseWriter, r *http.Request) {
		handleTaskRunDetail(w, r, taskService)
	})
	mux.HandleFunc("/webhooks/", func(w http.ResponseWriter, r *http.Request) {
		handleWebhookRoute(w, r, "/webhooks/", service)
	})
	mux.HandleFunc("/api/v1/webhooks/", func(w http.ResponseWriter, r *http.Request) {
		handleWebhookRoute(w, r, "/api/v1/webhooks/", service)
	})
	mux.HandleFunc("/runs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		jobID := strings.TrimSpace(r.URL.Query().Get("job_id"))
		if jobID == "" {
			writeJSONError(w, http.StatusBadRequest, errors.New("job_id query parameter is required"))
			return
		}
		ctx, cancel := contextWithTimeout(r, 15*time.Second)
		defer cancel()
		runs, err := service.Runs(ctx, jobID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.NotFound(w, r)
				return
			}
			writeJSONError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"job_id": jobID, "runs": runs})
	})
	mux.HandleFunc("/sync", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		ctx, cancel := contextWithTimeout(r, 30*time.Second)
		defer cancel()
		report, err := service.Reconcile(ctx)
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, report)
	})
	return mux
}

func handleTasksList(w http.ResponseWriter, r *http.Request, taskService *taskrun.Service) {
	if r.URL.Path != "/api/v1/tasks" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := contextWithTimeout(r, 15*time.Second)
	defer cancel()
	tasks, err := taskService.ListTasks(ctx)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": tasks})
}

// handleWorktreesList serves GET /api/v1/worktrees: the FULL worktree pool (Task-0016)
// — every pool member, idle and allocated, each with a stable worktree_id + status;
// allocated entries carry the bound { repo, issue/Task, worktree path, agent session id,
// session transcript path, run/gate state } read live from the per-run workflow. It
// SUPPLIES the raw fields needed to construct a VSCodium link to a bound session but
// never emits a vscodium:// link itself (the orchestrator boundary). Active non-pool
// owned-lane worktrees are merged in (ListFullPool), so the existing REG-008 parked-lane
// read keeps working through the dispatch-path transition.
func handleWorktreesList(w http.ResponseWriter, r *http.Request, taskService *taskrun.Service) {
	if r.URL.Path != "/api/v1/worktrees" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	worktrees, err := taskService.ListFullPool()
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"worktrees": worktrees})
}

// handleReposList serves GET /api/v1/repos: one entry per registered repo (id +
// local_root + task_provider_repo) read from the central registry at the configured
// OBSIDIAN_REGISTRY_PATH, with NO queue_workers in the response (Task-0016 removes it as
// an admission cap). It is the registry-sourced repo list the UI repo filter consumes.
func handleReposList(w http.ResponseWriter, r *http.Request, cfg config.Config) {
	if r.URL.Path != "/api/v1/repos" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	repos, err := taskrun.ListRepos(cfg.RegistryPath)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"repos": repos})
}

// handleWorktreeAPIRoute is the method/path-guarded sub-router for the worktree-pool
// lifecycle operations under /api/v1/worktrees/* (Task-0016), mirroring the
// handleTaskAPIRoute pattern: 405 on the wrong method, 404 on an unknown sub-path. It
// hosts create + destroy in PASS-0002; assign / eject / dequeue are added in later
// passes on the same sub-router so all pool operations share one consistent guard.
func handleWorktreeAPIRoute(w http.ResponseWriter, r *http.Request, taskService *taskrun.Service) {
	action := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/worktrees/"), "/")
	switch action {
	case "create":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Repo string `json:"repo"`
		}
		if err := decodeJSONBody(r, &body); err != nil {
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		created, err := taskService.CreatePoolWorktree(body.Repo)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusCreated, created)
	case "assign":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			TaskID     string `json:"task_id"`
			Repo       string `json:"repo"`
			WorktreeID string `json:"worktree_id"`
		}
		if err := decodeJSONBody(r, &body); err != nil {
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		ctx, cancel := contextWithTimeout(r, 30*time.Second)
		defer cancel()
		run, err := taskService.AssignTaskToPoolWorktree(ctx, body.TaskID, body.Repo, body.WorktreeID)
		if err != nil {
			if errors.Is(err, taskrun.ErrNoIdleWorktree) {
				writeJSONError(w, http.StatusConflict, err)
				return
			}
			if strings.Contains(err.Error(), "not found") {
				http.NotFound(w, r)
				return
			}
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusAccepted, run)
	case "destroy":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			WorktreeID string `json:"worktree_id"`
		}
		if err := decodeJSONBody(r, &body); err != nil {
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		if err := taskService.DestroyPoolWorktree(body.WorktreeID); err != nil {
			if errors.Is(err, taskrun.ErrPoolWorktreeAllocated) {
				writeJSONError(w, http.StatusConflict, err)
				return
			}
			if errors.Is(err, taskrun.ErrPoolWorktreeNotFound) {
				http.NotFound(w, r)
				return
			}
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "destroyed", "worktree_id": body.WorktreeID})
	case "dequeue":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Repo   string `json:"repo"`
			TaskID string `json:"task_id"`
		}
		if err := decodeJSONBody(r, &body); err != nil {
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		// Standalone dequeue: take the task out of the queue (Queue -> Never) WITHOUT
		// ejecting; it does NOT close the issue (the issue stays open).
		if err := taskService.DequeueTask(body.Repo, body.TaskID); err != nil {
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "dequeued", "task_id": body.TaskID})
	default:
		http.NotFound(w, r)
	}
}

// decodeJSONBody decodes an optional JSON request body into v. A nil/empty body is
// tolerated (the handlers all have safe zero-value defaults), matching the existing
// consumer-start route's lenient body handling.
func decodeJSONBody(r *http.Request, v any) error {
	if r.Body == nil {
		return nil
	}
	if err := json.NewDecoder(r.Body).Decode(v); err != nil && err.Error() != "EOF" {
		return err
	}
	return nil
}

// handleQueueConsumerRoute starts/stops the O3 queue-drain consumer (A3.4),
// following the POST dispatch-route pattern. POST /api/v1/queue/consumer/start
// starts the singleton consumer workflow against the configured provider repo;
// POST /api/v1/queue/consumer/stop signals it to exit. An optional JSON body on
// start ({"repo":..., "poll_interval_seconds":...}) overrides the configured repo
// / default ~2-min cadence.
func handleQueueConsumerRoute(w http.ResponseWriter, r *http.Request, cfg config.Config, consumer QueueDrainController) {
	if consumer == nil {
		writeJSONError(w, http.StatusServiceUnavailable, errors.New("queue-drain consumer is not configured"))
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	action := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/queue/consumer/"), "/")
	ctx, cancel := contextWithTimeout(r, 30*time.Second)
	defer cancel()

	switch action {
	case "start":
		config := queue.QueueDrainConfig{Repo: cfg.QueueDrainRepo}
		var body struct {
			Repo                string `json:"repo"`
			PollIntervalSeconds int    `json:"poll_interval_seconds"`
		}
		if r.Body != nil {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err.Error() != "EOF" {
				writeJSONError(w, http.StatusBadRequest, err)
				return
			}
		}
		if strings.TrimSpace(body.Repo) != "" {
			config.Repo = strings.TrimSpace(body.Repo)
		}
		if body.PollIntervalSeconds > 0 {
			config.PollInterval = time.Duration(body.PollIntervalSeconds) * time.Second
		}
		workflowID, err := consumer.StartQueueDrainConsumer(ctx, config)
		if err != nil {
			writeJSONError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"status": "started", "workflow_id": workflowID, "repo": config.Repo})
	case "stop":
		if err := consumer.StopQueueDrainConsumer(ctx); err != nil {
			writeJSONError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"status": "stopped"})
	default:
		http.NotFound(w, r)
	}
}

func handleTaskAPIRoute(w http.ResponseWriter, r *http.Request, taskService *taskrun.Service) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/tasks/")
	if trimmed == "" {
		http.NotFound(w, r)
		return
	}

	if strings.HasSuffix(trimmed, "/dispatch") {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		taskID := strings.TrimSuffix(trimmed, "/dispatch")
		taskID = strings.TrimSuffix(taskID, "/")
		if taskID == "" || strings.Contains(taskID, "/") {
			http.NotFound(w, r)
			return
		}
		ctx, cancel := contextWithTimeout(r, 30*time.Second)
		defer cancel()
		run, err := taskService.Dispatch(ctx, taskID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.NotFound(w, r)
				return
			}
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusAccepted, run)
		return
	}
	if strings.HasSuffix(trimmed, "/dispatch-workload-failure-exercise") {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		taskID := strings.TrimSuffix(trimmed, "/dispatch-workload-failure-exercise")
		taskID = strings.TrimSuffix(taskID, "/")
		if taskID == "" || strings.Contains(taskID, "/") {
			http.NotFound(w, r)
			return
		}
		ctx, cancel := contextWithTimeout(r, 30*time.Second)
		defer cancel()
		run, err := taskService.DispatchWorkloadFailureExercise(ctx, taskID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.NotFound(w, r)
				return
			}
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusAccepted, run)
		return
	}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	taskID := strings.TrimSuffix(trimmed, "/")
	if taskID == "" || strings.Contains(taskID, "/") {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := contextWithTimeout(r, 15*time.Second)
	defer cancel()
	task, err := taskService.Task(ctx, taskID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.NotFound(w, r)
			return
		}
		writeJSONError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func handleTaskRunDetail(w http.ResponseWriter, r *http.Request, taskService *taskrun.Service) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/task-runs/")
	if trimmed == "" {
		http.NotFound(w, r)
		return
	}

	if strings.HasSuffix(trimmed, "/state") {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		runID := strings.TrimSuffix(trimmed, "/state")
		runID = strings.TrimSuffix(runID, "/")
		if runID == "" || strings.Contains(runID, "/") {
			http.NotFound(w, r)
			return
		}
		var update taskrun.TaskRunUpdate
		if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		ctx, cancel := contextWithTimeout(r, 30*time.Second)
		defer cancel()
		run, err := taskService.UpdateRun(ctx, runID, update)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.NotFound(w, r)
				return
			}
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusAccepted, run)
		return
	}
	if strings.HasSuffix(trimmed, "/poke") {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		runID := strings.TrimSuffix(trimmed, "/poke")
		runID = strings.TrimSuffix(runID, "/")
		if runID == "" || strings.Contains(runID, "/") {
			http.NotFound(w, r)
			return
		}
		ctx, cancel := contextWithTimeout(r, 30*time.Second)
		defer cancel()
		run, err := taskService.PokeRun(ctx, runID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.NotFound(w, r)
				return
			}
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusAccepted, run)
		return
	}
	if strings.HasSuffix(trimmed, "/interrupt") {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		runID := strings.TrimSuffix(trimmed, "/interrupt")
		runID = strings.TrimSuffix(runID, "/")
		if runID == "" || strings.Contains(runID, "/") {
			http.NotFound(w, r)
			return
		}
		ctx, cancel := contextWithTimeout(r, 30*time.Second)
		defer cancel()
		run, err := taskService.InterruptRun(ctx, runID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.NotFound(w, r)
				return
			}
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusAccepted, run)
		return
	}
	if strings.HasSuffix(trimmed, "/retry-cleanup") {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		runID := strings.TrimSuffix(trimmed, "/retry-cleanup")
		runID = strings.TrimSuffix(runID, "/")
		if runID == "" || strings.Contains(runID, "/") {
			http.NotFound(w, r)
			return
		}
		ctx, cancel := contextWithTimeout(r, 30*time.Second)
		defer cancel()
		run, err := taskService.RetryCleanupRun(ctx, runID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.NotFound(w, r)
				return
			}
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusAccepted, run)
		return
	}
	if strings.HasSuffix(trimmed, "/retry-workload") {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		runID := strings.TrimSuffix(trimmed, "/retry-workload")
		runID = strings.TrimSuffix(runID, "/")
		if runID == "" || strings.Contains(runID, "/") {
			http.NotFound(w, r)
			return
		}
		ctx, cancel := contextWithTimeout(r, 30*time.Second)
		defer cancel()
		run, err := taskService.RetryWorkloadRun(ctx, runID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.NotFound(w, r)
				return
			}
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusAccepted, run)
		return
	}
	if strings.HasSuffix(trimmed, "/resolve-interrupt-review") {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		runID := strings.TrimSuffix(trimmed, "/resolve-interrupt-review")
		runID = strings.TrimSuffix(runID, "/")
		if runID == "" || strings.Contains(runID, "/") {
			http.NotFound(w, r)
			return
		}
		var resolution taskrun.InterruptReviewResolution
		if err := json.NewDecoder(r.Body).Decode(&resolution); err != nil {
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		ctx, cancel := contextWithTimeout(r, 30*time.Second)
		defer cancel()
		run, err := taskService.ResolveInterruptReview(ctx, runID, resolution)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.NotFound(w, r)
				return
			}
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusAccepted, run)
		return
	}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	runID := strings.TrimSuffix(trimmed, "/")
	if runID == "" || strings.Contains(runID, "/") {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := contextWithTimeout(r, 15*time.Second)
	defer cancel()
	run, err := taskService.Run(ctx, runID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.NotFound(w, r)
			return
		}
		writeJSONError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func handleJobsList(w http.ResponseWriter, r *http.Request, exactPath string, service *controlplane.Service) {
	if r.URL.Path != exactPath {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := contextWithTimeout(r, 15*time.Second)
	defer cancel()
	state, err := service.Snapshot(ctx)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": state.Jobs, "last_sync": state.LastSync, "generated_at": state.GeneratedAt})
}

func handleJobDetail(w http.ResponseWriter, r *http.Request, prefix string, service *controlplane.Service) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	jobID := strings.TrimPrefix(r.URL.Path, prefix)
	if jobID == "" || strings.Contains(jobID, "/") {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := contextWithTimeout(r, 15*time.Second)
	defer cancel()
	job, err := service.Job(ctx, jobID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.NotFound(w, r)
			return
		}
		writeJSONError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func handleJobAPIRoute(w http.ResponseWriter, r *http.Request, service *controlplane.Service) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/jobs/")
	if trimmed == "" {
		http.NotFound(w, r)
		return
	}

	if strings.HasSuffix(trimmed, "/runs") {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		jobID := strings.TrimSuffix(trimmed, "/runs")
		jobID = strings.TrimSuffix(jobID, "/")
		if jobID == "" || strings.Contains(jobID, "/") {
			http.NotFound(w, r)
			return
		}
		ctx, cancel := contextWithTimeout(r, 15*time.Second)
		defer cancel()
		runs, err := service.Runs(ctx, jobID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.NotFound(w, r)
				return
			}
			writeJSONError(w, http.StatusBadGateway, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"job_id": jobID, "runs": runs})
		return
	}

	if strings.HasSuffix(trimmed, "/run") {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		jobID := strings.TrimSuffix(trimmed, "/run")
		jobID = strings.TrimSuffix(jobID, "/")
		if jobID == "" || strings.Contains(jobID, "/") {
			http.NotFound(w, r)
			return
		}
		ctx, cancel := contextWithTimeout(r, 30*time.Second)
		defer cancel()
		started, err := service.RunNow(ctx, jobID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.NotFound(w, r)
				return
			}
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusAccepted, started)
		return
	}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	handleJobDetail(w, r, "/api/v1/jobs/", service)
}

func handleWebhookRoute(w http.ResponseWriter, r *http.Request, prefix string, service *controlplane.Service) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	webhookPath := strings.TrimPrefix(r.URL.Path, prefix)
	webhookPath = strings.Trim(webhookPath, "/")
	if webhookPath == "" {
		http.NotFound(w, r)
		return
	}

	ctx, cancel := contextWithTimeout(r, 30*time.Second)
	defer cancel()
	started, err := service.TriggerWebhook(ctx, webhookPath)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.NotFound(w, r)
			return
		}
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusAccepted, started)
}

func handleHealth(w http.ResponseWriter, r *http.Request, cfg config.Config, service *controlplane.Service) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := contextWithTimeout(r, 15*time.Second)
	defer cancel()
	state, err := service.Snapshot(ctx)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"status":           "degraded",
			"jobs_root":        cfg.JobsRoot,
			"namespace":        cfg.Namespace,
			"task_queue":       cfg.TaskQueue,
			"temporal_address": cfg.TemporalAddress,
			"error":            err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"jobs_root":        cfg.JobsRoot,
		"namespace":        cfg.Namespace,
		"task_queue":       cfg.TaskQueue,
		"temporal_address": cfg.TemporalAddress,
		"last_sync":        state.LastSync,
		"job_count":        len(state.Jobs),
		"generated_at":     state.GeneratedAt,
	})
}

func contextWithTimeout(r *http.Request, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), timeout)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeJSONError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
