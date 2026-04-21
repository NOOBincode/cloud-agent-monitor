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
	topoapp "cloud-agent-monitor/internal/topology/application"
	"cloud-agent-monitor/pkg/config"
	"cloud-agent-monitor/pkg/infra"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type App struct {
	config      *config.Config
	server      *gin.Engine
	db          *gorm.DB
	queue       *infra.Queue
	topoSvc     *topoapp.TopologyService
	impactCache *topoapp.ImpactCacheService
}

func (a *App) Run() error {
	ctx := context.Background()

	if a.queue != nil {
		go func() {
			if err := a.queue.Start(); err != nil {
				log.Printf("queue start failed: %v", err)
			}
		}()
	}

	if a.topoSvc != nil {
		if err := a.topoSvc.Start(ctx); err != nil {
			log.Printf("topology service start failed: %v", err)
		}
	}

	if a.impactCache != nil {
		if err := a.impactCache.Start(ctx); err != nil {
			log.Printf("impact cache service start failed: %v", err)
		}
	}

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

	if a.impactCache != nil {
		a.impactCache.Stop()
	}
	if a.topoSvc != nil {
		a.topoSvc.Stop()
	}
	if a.queue != nil {
		a.queue.Stop()
	}

	return nil
}

func (a *App) HealthCheck(ctx context.Context) error {
	return storage.Ping(ctx, a.db)
}
