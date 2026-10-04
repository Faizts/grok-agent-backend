package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/grok-agent/backend/internal/config"
	"github.com/grok-agent/backend/internal/db"
	"github.com/grok-agent/backend/internal/logger"
)

type Server struct {
	router *gin.Engine
	cfg    *config.Config
	server *http.Server
	pool   *pgxpool.Pool
}

func NewServer() (*Server, error) {
	cfg := config.Load()
	logger.Init(true)

	ctx := context.Background()

	pool, err := db.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}

	redisClient, err := db.NewRedis(cfg.RedisURL)
	if err != nil {
		logger.L.Warn("redis unavailable, continuing without it", zap.Error(err))
	}
	_ = redisClient

	s := &Server{
		cfg:    cfg,
		pool:   pool,
		router: gin.Default(),
	}

	setupRouter(s)
	s.server = &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: s.router,
	}

	return s, nil
}

func (s *Server) Run() error {
	logger.L.Info("starting server", zap.String("port", s.cfg.Port))

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.L.Info("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	s.pool.Close()
	logger.L.Info("server stopped")
	return nil
}
