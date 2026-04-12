package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"cloud-agent-monitor/internal/storage"
	"cloud-agent-monitor/pkg/config"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type App struct {
	config *config.Config
	server *gin.Engine
	db     *gorm.DB
}

func (a *App) Run() error {
	go func() {
		fmt.Printf("platform-api listening on %s\n", a.config.Server.Addr)
		if err := a.server.Run(a.config.Server.Addr); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server exit: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("shutting down server...")
	return nil
}

func (a *App) HealthCheck(ctx context.Context) error {
	return storage.Ping(ctx, a.db)
}
