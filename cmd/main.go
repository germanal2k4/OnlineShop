package main

import (
	"gitlab.ozon.dev/qwestard/homework/internal/audit"
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
	auditPool := audit.NewAuditWorkerPool(5, 500*time.Millisecond, &audit.StdoutProcessor{Filter: cfg.FilterWord})

	srv := server.NewServer(repo, cfg, auditPool)

	if err := srv.Run(); err != nil {
		log.Fatalf("Server stopped with error: %v", err)
	}
}
