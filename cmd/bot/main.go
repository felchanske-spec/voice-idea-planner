package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/felchanske-spec/voice-idea-planner/internal/config"
	"github.com/felchanske-spec/voice-idea-planner/internal/health"
	"github.com/felchanske-spec/voice-idea-planner/internal/llm"
	"github.com/felchanske-spec/voice-idea-planner/internal/repository"
	"github.com/felchanske-spec/voice-idea-planner/internal/telegram"
	"github.com/felchanske-spec/voice-idea-planner/internal/usecase"
	"github.com/felchanske-spec/voice-idea-planner/internal/visualizer"
)

func main() {
	log.Println("[Main] Initialization voice-idea-planner backend...")

	shutdownCtx, stopSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignal()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[Main] Configuration initialization failed: %v", err)
	}
	log.Println("[Main] Configuratuin successfully loaded")

	pool, err := repository.InitPostgres(shutdownCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[Main] Postgres initialization failed: %v", err)
	}
	defer pool.Close()

	repo := repository.NewPostgresRepository(pool)
	llmClien := llm.NewGeminiClient(cfg.GeminiKey)
	vizer := visualizer.NewMermaidVisualizer()

	plannerUseCase := usecase.NewPlannerUseCase(repo, llmClien, vizer)

	tgClient, err := telegram.NewBotGateway(cfg.TelegramBotToken, plannerUseCase)
	if err != nil {
		log.Fatalf("[Main] Telegram client initialization failed: %v", err)
	}

	healthHandler := health.NewHealthHandler(pool)
	httpMux := http.NewServeMux()
	httpMux.HandleFunc("/health/liveness", healthHandler.Liveness)
	httpMux.HandleFunc("/health/readiness", healthHandler.Readiness)

	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: httpMux,
	}

	go func() {
		log.Printf("[Main] HTTP Health Server rinning on port :8080")
		if err := httpServer.ListenAndServe(); err != nil {
			log.Printf("[Main] HTTP Server error: %v", err)
		}
	}()

	go tgClient.Start(shutdownCtx)

	<-shutdownCtx.Done()
	log.Println("[Main] Shutdown signal received, Starting Graceful Shutdown...")

	shutdownhttpCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = httpServer.Shutdown(shutdownhttpCtx)

	log.Println("[Main] All active components finished. Service stopped safely")
}
