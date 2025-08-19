package cache

import (
	"context"
	"sync"
	"time"

	"homework/internal/models"
	"homework/internal/repository"
)

type Cache interface {
	Refresh(repo repository.Repository) error
}

type ActiveOrdersCache struct {
	Mu     sync.RWMutex
	Orders map[string]*models.Order
}

func NewActiveOrdersCache() *ActiveOrdersCache {
	return &ActiveOrdersCache{
		Orders: make(map[string]*models.Order),
	}
}

type HistoryCache struct {
	mu     sync.RWMutex
	orders []*models.Order
}

func NewHistoryCache() *HistoryCache {
	return &HistoryCache{
		orders: make([]*models.Order, 0),
	}
}

func (c *HistoryCache) Refresh(repo repository.Repository) error {
	orders, err := repo.List("", 1000, "")
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.orders = orders
	c.mu.Unlock()
	return nil
}

func (c *HistoryCache) Get() []*models.Order {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.orders
}

func (c *HistoryCache) StartAutoRefresh(ctx context.Context, repo repository.Repository, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := c.Refresh(repo); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
