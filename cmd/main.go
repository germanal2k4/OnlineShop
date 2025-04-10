package main

import (
	"context"
	"gitlab.ozon.dev/qwestard/homework/internal/kafka"
	taskprocessor "gitlab.ozon.dev/qwestard/homework/internal/processor"
	"log"
	"os/signal"
	"syscall"
	"time"

	"gitlab.ozon.dev/qwestard/homework/internal/audit"
	"gitlab.ozon.dev/qwestard/homework/internal/cache"
	"gitlab.ozon.dev/qwestard/homework/internal/config"
	"gitlab.ozon.dev/qwestard/homework/internal/db"
	"gitlab.ozon.dev/qwestard/homework/internal/repository"
	"gitlab.ozon.dev/qwestard/homework/internal/server"
	"gitlab.ozon.dev/qwestard/homework/internal/service"
	"gitlab.ozon.dev/qwestard/homework/internal/wrapper"
)

func main() {
	cfg := config.LoadConfig()

	database, err := db.NewDB(cfg.DSN)
	if err != nil {
		log.Fatalf("Error connecting to db: %v", err)
	}
	defer database.Close()

	repo := repository.NewOrderRepository(database)

	taskRepo := repository.NewPostgresTaskRepository(database)

	prod, err := kafka.NewSaramaProducer(cfg.KafkaBrokers)
	if err != nil {
		log.Fatalf("Error creating Kafka producer: %v", err)
	}
	defer func() {
		if err := prod.Close(); err != nil {
			log.Printf("Error closing Kafka producer: %v", err)
		}
	}()

	processorConfigs := []audit.ProcessorConfig{
		{
			Processor:   &audit.DBProcessor{Db: database},
			BatchSize:   5,
			Timeout:     500 * time.Millisecond,
			ChannelSize: 50,
		},
		{
			Processor:   &audit.StdoutProcessor{Filter: cfg.FilterWord},
			BatchSize:   3,
			Timeout:     2 * time.Second,
			ChannelSize: 50,
		},
	}

	auditPool := audit.NewAuditWorkerPool(processorConfigs...)

	activeCache := cache.NewActiveOrdersCache()

	historyCache := cache.NewHistoryCache()
	if err := historyCache.Refresh(repo); err != nil {
		log.Fatalf("Error refreshing history cache: %v", err)
	}

	orderService := service.NewOrderService(repo, activeCache, historyCache)
	orderWrapper := wrapper.NewOrderWrapper(orderService)

	if err := orderWrapper.RefreshActiveOrders(); err != nil {
		log.Fatalf("Error refreshing active cache: %v", err)
	}
	srv := server.NewServer(orderWrapper, cfg, auditPool)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go historyCache.StartAutoRefresh(ctx, repo, 5*time.Minute)

	taskProc := taskprocessor.NewTaskProcessor(taskRepo, *prod, cfg.KafkaTopic, 1*time.Second, 10)
	go taskProc.Start(ctx)

	cont, cancelF := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancelF()

	go kafka.StartSaramaConsumer(cont, cfg.KafkaConfig, cfg.KafkaBrokers, cfg.KafkaGroupID, []string{cfg.KafkaTopic})

	go historyCache.StartAutoRefresh(ctx, repo, 5*time.Minute)

	if err := srv.Run(); err != nil {
		log.Fatalf("Server stopped with error: %v", err)
	}
}
