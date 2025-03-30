package main

import (
	"gitlab.ozon.dev/qwestard/homework/internal/audit"
	"gitlab.ozon.dev/qwestard/homework/internal/cache"
	"gitlab.ozon.dev/qwestard/homework/internal/config"
	"gitlab.ozon.dev/qwestard/homework/internal/db"
	"gitlab.ozon.dev/qwestard/homework/internal/repository"
	"gitlab.ozon.dev/qwestard/homework/internal/server"
	"log"
	"time"
)

func main() {
	cfg := config.LoadConfig()

	database, err := db.NewDB(cfg.DSN)
	if err != nil {
		log.Fatalf("Error in connection to db: %v", err)
	}
	defer database.Close()

	repo := repository.NewOrderRepository(database)

	poolConfig := audit.AuditPoolConfig{
		BatchSize:   5,
		Timeout:     500 * time.Millisecond,
		ChannelSize: 50,
		Worker:      2,
	}
	auditPool := audit.NewAuditWorkerPool(poolConfig, &audit.StdoutProcessor{Filter: cfg.FilterWord}, &audit.DBProcessor{Db: database})

	activeCache := cache.NewActiveOrdersCache()

	if err := activeCache.Refresh(repo); err != nil {
		log.Fatalf("Error refreshing active cache: %v", err)
	}

	historyCache := cache.NewHistoryCache()
	if err := historyCache.Refresh(repo); err != nil {
		log.Fatalf("Error refreshing history cache: %v", err)
	}
	stopCh := make(chan struct{})

	defer close(stopCh)

	historyCache.StartAutoRefresh(repo, 5*time.Minute, stopCh)

	srv := server.NewServer(repo, cfg, auditPool, activeCache, historyCache)

	if err := srv.Run(); err != nil {
		log.Fatalf("Server stopped with error: %v", err)
	}
}
