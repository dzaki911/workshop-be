package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"workshop-be/config"
	"workshop-be/infra/database"
	delivery "workshop-be/internal/delivery/http"
	"workshop-be/internal/repository/postgres"
	"workshop-be/internal/service"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.Open(ctx, cfg.DB.URL())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	userRepo := postgres.NewUserRepository(db)
	aspectRepo := postgres.NewAspectRepository(db)
	reportRepo := postgres.NewReportRepository(db)

	h := delivery.NewHandler(
		service.NewUserService(userRepo),
		service.NewAspectService(aspectRepo),
		service.NewReportService(reportRepo, userRepo, aspectRepo),
	)
	adminOnly := delivery.DevAdmin(cfg.AppEnv == "development", cfg.DevAdminEmail, userRepo)

	srv := &http.Server{
		Addr:              ":" + cfg.AppPort,
		Handler:           delivery.NewRouter(h, adminOnly),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("server berjalan di :%s (env=%s)", cfg.AppPort, cfg.AppEnv)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
