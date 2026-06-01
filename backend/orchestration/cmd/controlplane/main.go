package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/config"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/controlplane"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/httpapi"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/taskrun"
	"github.com/gregsemple2003/CodexDesktop/backend/orchestration/internal/temporalbackend"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	backend, err := temporalbackend.New(cfg)
	if err != nil {
		log.Fatalf("init temporal backend: %v", err)
	}
	defer func() {
		_ = backend.Close()
	}()

	service := controlplane.NewService(cfg.JobsRoot, backend)
	taskService := taskrun.NewService(cfg.WorktreeRoot, cfg.RunsRoot, backend)
	// Wire the multi-repo task-provider dequeue write so in-app Eject/Dequeue actually
	// writes Queue=Never to the CORRECT repo's task provider (Task-0016 BUG-0001). The
	// dashboard Service spans all registered repos, so the provider resolves each ejected
	// worktree's repo -> its task_provider slug via the registry and writes through the
	// gh provider; a repo with no GitHub provider configured is a safe no-op.
	taskService.SetDequeueProvider(temporalbackend.NewControlPlaneDequeueProvider(cfg.RegistryPath))

	worker, err := backend.StartWorker(cfg, taskService)
	if err != nil {
		log.Fatalf("start temporal worker: %v", err)
	}
	defer worker.Stop()

	startupCtx, startupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	_, err = service.Reconcile(startupCtx)
	startupCancel()
	if err != nil {
		log.Fatalf("startup reconcile: %v", err)
	}

	server := &http.Server{
		Addr:              cfg.BindAddress,
		Handler:           httpapi.NewMux(cfg, service, taskService, backend),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("codex orchestration control-plane listening on %s", cfg.BindAddress)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("listen: %v", err)
	}
}
