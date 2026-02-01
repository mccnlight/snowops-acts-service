package main

import (
	"fmt"
	"os"

	"github.com/nurpe/snowops-acts/internal/auth"
	"github.com/nurpe/snowops-acts/internal/config"
	"github.com/nurpe/snowops-acts/internal/db"
	"github.com/nurpe/snowops-acts/internal/excel"
	httphandler "github.com/nurpe/snowops-acts/internal/http"
	"github.com/nurpe/snowops-acts/internal/http/middleware"
	"github.com/nurpe/snowops-acts/internal/logger"
	"github.com/nurpe/snowops-acts/internal/repository"
	"github.com/nurpe/snowops-acts/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New(cfg.Environment)

	database, err := db.New(cfg, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect database")
	}

	reportRepo := repository.NewReportRepository(database)
	excelGenerator := excel.NewGenerator()

	actService := service.NewActService(reportRepo, excelGenerator, cfg)

	tokenParser := auth.NewParser(cfg.Auth.AccessSecret)
	handler := httphandler.NewHandler(actService, log)
	authMiddleware := middleware.Auth(tokenParser)
	router := httphandler.NewRouter(handler, authMiddleware, cfg.Environment)

	addr := fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port)
	log.Info().Str("addr", addr).Msg("starting acts service")

	if err := router.Run(addr); err != nil {
		log.Error().Err(err).Msg("server stopped")
		os.Exit(1)
	}
}
