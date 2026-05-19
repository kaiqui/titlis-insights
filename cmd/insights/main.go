package main

import (
	"context"
	"fmt"
	nethttp "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/titlis/insights/internal/config"
	thttp "github.com/titlis/insights/internal/http"
	"github.com/titlis/insights/internal/observability"
	"github.com/titlis/insights/internal/recommend"
	"github.com/titlis/insights/internal/repo"
	"github.com/titlis/insights/internal/source"
	"github.com/titlis/insights/internal/source/datadog"
	"github.com/titlis/insights/internal/source/memory"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	log := observability.NewLogger(cfg.LogLevel, cfg.LogFormat)
	log.Info("starting", "service", "titlis-insights", "env", cfg.AppEnv, "port", cfg.Port)

	credProvider := datadog.StaticCredentialProvider{Site: cfg.DatadogSite}
	var src source.MetricsSource
	if cfg.UseStubSource {
		src = memory.NewSource()
		log.Info("using memory metrics source")
	} else {
		src = datadog.NewClient(credProvider)
		log.Info("using datadog metrics source", "site", cfg.DatadogSite)
	}
	var templates repo.TemplateRepo = repo.NewMemoryTemplateRepo()
	var logRepo repo.RecommendationLogRepo = repo.NoopRecommendationLog{}
	if cfg.DatabaseURL != "" {
		db, dbErr := pgxpool.New(context.Background(), cfg.DatabaseURL)
		if dbErr != nil {
			log.Error("postgres connect failed; using memory repos", "error", dbErr.Error())
		} else {
			templates = repo.NewPGTemplateRepo(db)
			logRepo = repo.NewPGRecommendationLog(db)
			log.Info("using postgres repos")
		}
	}
	rec := recommend.NewRecommender(src, templates, recommend.DefaultOptions())
	handlers := thttp.NewHandlers(rec, templates, logRepo)
	if !cfg.UseStubSource {
		handlers.Prober = datadog.NewClient(credProvider)
	}
	router := thttp.NewRouter(handlers, cfg.InternalSecret, log)

	srv := &nethttp.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != nethttp.ErrServerClosed {
			errCh <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case s := <-stop:
		log.Info("shutdown signal received", "signal", s.String())
	case err := <-errCh:
		log.Error("server error", "error", err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}
