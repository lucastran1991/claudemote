package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/mac/claudemote/backend/internal/config"
	"github.com/mac/claudemote/backend/internal/database"
	"github.com/mac/claudemote/backend/internal/handler"
	"github.com/mac/claudemote/backend/internal/repository"
	"github.com/mac/claudemote/backend/internal/router"
	"github.com/mac/claudemote/backend/internal/service"
	"github.com/mac/claudemote/backend/internal/sse"
	"github.com/mac/claudemote/backend/internal/worker"
)

func main() {
	// 1. Load and validate configuration — panics on missing required vars.
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	// 2. Open SQLite with WAL mode + run embedded migrations.
	db, err := database.Connect(cfg.DBPath)
	if err != nil {
		log.Fatalf("database connect: %v", err)
	}
	if err := database.RunMigrations(db); err != nil {
		log.Fatalf("database migrate: %v", err)
	}
	log.Println("database ready:", cfg.DBPath)

	// 3. Wire dependency graph: repo → service → handler.
	userRepo := repository.NewUserRepository(db)
	jobRepo := repository.NewJobRepository(db)
	jobLogRepo := repository.NewJobLogRepository(db)

	authService := service.NewAuthService(userRepo, cfg.JWTSecret)
	jobService := service.NewJobService(jobRepo, jobLogRepo, cfg.ClaudeDefaultModel)

	// 4. Construct SSE hub and wire it into the worker pool so every LogWriter
	//    fans out live lines to connected stream subscribers.
	hub := sse.NewHub()
	pool := worker.New(cfg, db, jobRepo, jobLogRepo, hub)

	// Inject pool into service (avoids circular init: pool needs repos, service needs pool).
	jobService.SetPool(pool)

	// 5. Boot recovery: mark crashed-running jobs as failed, re-enqueue pending jobs.
	if err := pool.Recover(); err != nil {
		log.Fatalf("worker pool recovery: %v", err)
	}

	// 6. Start workers under a root context that is cancelled on SIGINT/SIGTERM.
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool.Start(rootCtx)

	// 7. Build Gin engine and start listening.
	authHandler := handler.NewAuthHandler(authService)
	jobHandler := handler.NewJobHandler(jobService)
	streamHandler := handler.NewStreamHandler(jobRepo, jobLogRepo, hub)

	r := router.Setup(authHandler, jobHandler, streamHandler, cfg.JWTSecret, cfg.CORSOrigin)

	log.Printf("server listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
