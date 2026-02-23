package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/piotr-halas/decodo-workflow/internal/agent"
	"github.com/piotr-halas/decodo-workflow/internal/api"
	"github.com/piotr-halas/decodo-workflow/internal/config"
	"github.com/piotr-halas/decodo-workflow/internal/db"
	"github.com/piotr-halas/decodo-workflow/internal/integrations/jira"
	"github.com/piotr-halas/decodo-workflow/internal/workflow"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	var portFlag int
	flag.IntVar(&portFlag, "port", 0, "Server port (overrides SERVER_PORT env var)")
	flag.Parse()

	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Override port if provided via CLI flag
	if portFlag != 0 {
		cfg.ServerPort = portFlag
	}

	database, err := db.New(cfg.DBPath)
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	projects, err := database.GetProjects()
	if err != nil {
		logger.Error("failed to load projects", "error", err)
		os.Exit(1)
	}
	for _, p := range projects {
		if err := agent.SetupTargetRepo(p.RepoPath, logger); err != nil {
			logger.Warn("failed to setup agent definitions for project", "project", p.Name, "error", err)
		}
	}

	runner := agent.NewRunner(cfg.OpencodeBin, cfg.QwenBin, logger)
	engine := workflow.NewEngine(database, runner, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.JiraEnabled {
		jiraClient := jira.NewClient(cfg.JiraURL, cfg.JiraEmail, cfg.JiraToken, logger)
		go syncLoop(ctx, cfg, database, jiraClient, logger)
		syncOnce(database, jiraClient, logger)
		logger.Info("Jira sync enabled", "interval", cfg.JiraSyncInterval)
	} else {
		logger.Info("Jira sync disabled - use POST /api/projects/{id}/tickets to add tickets manually")
	}

	stop := make(chan struct{})
	go engine.RunLoop(stop)

	router := api.NewRouter(database, engine)
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.ServerPort),
		Handler: router,
	}

	go func() {
		logger.Info("starting server", "port", cfg.ServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down...")
	close(stop)
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	server.Shutdown(shutdownCtx)
}

func syncLoop(ctx context.Context, cfg *config.Config, database *db.DB, jiraClient *jira.Client, logger *slog.Logger) {
	ticker := time.NewTicker(cfg.JiraSyncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			syncOnce(database, jiraClient, logger)
		}
	}
}

func syncOnce(database *db.DB, jiraClient *jira.Client, logger *slog.Logger) {
	logger.Info("syncing tickets from Jira...")

	tickets, err := jiraClient.FetchMyTickets()
	if err != nil {
		logger.Error("failed to fetch tickets from Jira", "error", err)
		return
	}

	for i := range tickets {
		tickets[i].Source = "jira"
		if err := database.UpsertTicket(&tickets[i]); err != nil {
			logger.Error("failed to upsert ticket", "key", tickets[i].JiraKey, "error", err)
			continue
		}
	}

	logger.Info("sync complete", "tickets", len(tickets))
}
