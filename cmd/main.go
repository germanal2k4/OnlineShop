package main

import (
	"log"

	"gitlab.ozon.dev/qwestard/homework/internal/config"
	"gitlab.ozon.dev/qwestard/homework/internal/db"
	"gitlab.ozon.dev/qwestard/homework/internal/repository"
	"gitlab.ozon.dev/qwestard/homework/internal/server"
)

func main() {
	cfg := config.LoadConfig()

	database, err := db.NewDB(cfg.DSN)
	if err != nil {
		log.Fatalf("Error in connection to db: %v", err)
	}
	defer database.Close()

	repo := repository.NewOrderRepository(database)

	srv := server.NewServer(repo, cfg)

	if err := srv.Run(); err != nil {
		log.Fatalf("Server stopped: %v", err)
	}
}
