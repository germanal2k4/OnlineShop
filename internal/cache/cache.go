package cache

import (
	"context"
	"sync"
	"time"

	"gitlab.ozon.dev/qwestard/homework/internal/models"
	"gitlab.ozon.dev/qwestard/homework/internal/repository"
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

func (c *ActiveOrdersCache) Refresh(repo repository.Repository) error {
	orders, err := repo.List("", 1000, "")
	if err != nil {
		return err
	}
	newMap := make(map[string]*models.Order)
	for _, o := range orders {
		state := o.CurrentState()
		if state == models.OrderStateAccepted || state == models.OrderStateDelivered {
			newMap[o.ID] = o
		}
	}
	c.Mu.Lock()
	c.Orders = newMap
	c.Mu.Unlock()
	return nil
}

func (c *ActiveOrdersCache) Get() map[string]*models.Order {
	c.Mu.RLock()
	defer c.Mu.RUnlock()
	result := make(map[string]*models.Order, len(c.Orders))
	for k, v := range c.Orders {
		result[k] = v
	}
	return result
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
