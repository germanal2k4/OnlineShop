package main

import (
	"log"
	"os"

	"gitlab.ozon.dev/qwestard/homework/internal/config"
	"gitlab.ozon.dev/qwestard/homework/internal/db"
	"gitlab.ozon.dev/qwestard/homework/internal/repository"
	"gitlab.ozon.dev/qwestard/homework/internal/server"
)

func main() {
	cfg := config.LoadConfig()
	migrationsDir := "migrations"
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		log.Fatalf("Нет папки с миграциями: %s", migrationsDir)
	}

	database, err := db.NewDB(cfg.DSN, migrationsDir)
	if err != nil {
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}
	defer database.Close()

	repo := repository.NewOrderRepository(database)

	srv := server.NewServer(repo, cfg.Username, cfg.Password, cfg.Addr())

	if err := srv.Run(); err != nil {
		log.Fatalf("Сервер упал: %v", err)
	}
}
