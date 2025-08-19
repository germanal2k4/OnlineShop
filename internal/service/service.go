package service

import (
	"errors"

	"homework/internal/cache"
	"homework/internal/models"
	"homework/internal/repository"
)

type OrderService struct {
	repo         repository.Repository
	activeCache  *cache.ActiveOrdersCache
	historyCache *cache.HistoryCache
}

func NewOrderService(repo repository.Repository, activeCache *cache.ActiveOrdersCache, historyCache *cache.HistoryCache) *OrderService {
	return &OrderService{
		repo:         repo,
		activeCache:  activeCache,
		historyCache: historyCache,
	}
}

func (s *OrderService) RefreshActiveOrders() error {
	orders, err := s.repo.List("", 1000, "")
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
	s.activeCache.Mu.Lock()
	s.activeCache.Orders = newMap
	s.activeCache.Mu.Unlock()
	return nil
}

func (s *OrderService) GetOrderByID(id string) (*models.Order, error) {
	s.activeCache.Mu.RLock()
	order, ok := s.activeCache.Orders[id]
	s.activeCache.Mu.RUnlock()
	if ok {
		return order, nil
	}
	order, err := s.repo.GetID(id)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, errors.New("order not found")
	}
	go func(o *models.Order) {
		s.activeCache.Mu.Lock()
		s.activeCache.Orders[o.ID] = o
		s.activeCache.Mu.Unlock()
	}(order)
	return order, nil
}

func (s *OrderService) CreateOrder(order *models.Order) error {
	if err := s.repo.Create(order); err != nil {
		return err
	}
	s.activeCache.Mu.Lock()
	s.activeCache.Orders[order.ID] = order
	s.activeCache.Mu.Unlock()
	return nil
}

func (s *OrderService) UpdateOrder(order *models.Order) error {
	if err := s.repo.UpdateTx(order); err != nil {
		return err
	}
	s.activeCache.Mu.Lock()
	s.activeCache.Orders[order.ID] = order
	s.activeCache.Mu.Unlock()
	return nil
}

func (s *OrderService) DeleteOrder(id string) error {
	if err := s.repo.Delete(id); err != nil {
		return err
	}
	s.activeCache.Mu.Lock()
	delete(s.activeCache.Orders, id)
	s.activeCache.Mu.Unlock()
	return nil
}

func (s *OrderService) DeliverOrder(id string) error {
	if err := s.repo.Deliver(id); err != nil {
		return err
	}
	order, err := s.repo.GetID(id)
	if err != nil {
		return err
	}
	if order != nil {
		s.activeCache.Mu.Lock()
		s.activeCache.Orders[id] = order
		s.activeCache.Mu.Unlock()
	}
	return nil
}

func (s *OrderService) ClientReturnOrder(id string) error {
	if err := s.repo.ClientReturn(id); err != nil {
		return err
	}
	order, err := s.repo.GetID(id)
	if err != nil {
		return err
	}
	if order != nil {
		s.activeCache.Mu.Lock()
		s.activeCache.Orders[id] = order
		s.activeCache.Mu.Unlock()
	}
	return nil
}

func (s *OrderService) AcceptOrder(id string) error {
	if err := s.repo.AcceptOrder(id); err != nil {
		return err
	}
	order, err := s.repo.GetID(id)
	if err != nil {
		return err
	}
	if order != nil {
		s.activeCache.Mu.Lock()
		s.activeCache.Orders[id] = order
		s.activeCache.Mu.Unlock()
	}
	return nil
}

func (s *OrderService) CourierReturnOrder(id string) error {
	if err := s.repo.ReturnOrder(id); err != nil {
		return err
	}
	order, err := s.repo.GetID(id)
	if err != nil {
		return err
	}
	if order != nil {
		s.activeCache.Mu.Lock()
		s.activeCache.Orders[id] = order
		s.activeCache.Mu.Unlock()
	}
	return nil
}

func (s *OrderService) ListActiveOrders() ([]*models.Order, error) {
	s.activeCache.Mu.RLock()
	defer s.activeCache.Mu.RUnlock()
	orders := make([]*models.Order, 0, len(s.activeCache.Orders))
	for _, o := range s.activeCache.Orders {
		orders = append(orders, o)
	}
	return orders, nil
}

func (s *OrderService) ListHistoryOrders() ([]*models.Order, error) {
	return s.historyCache.Get(), nil
}

func (s *OrderService) ListReturns(offset, limit int64, recipientID string) ([]*models.Order, error) {
	historyOrders := s.historyCache.Get()
	var filtered []*models.Order
	for _, o := range historyOrders {
		if !o.ClientReturnAt.IsZero() {
			if recipientID != "" && o.RecipientID != recipientID {
				continue
			}
			filtered = append(filtered, o)
		}
	}
	if len(filtered) > 0 {
		start := offset
		if start > int64(len(filtered)) {
			start = int64(len(filtered))
		}
		end := start + limit
		if end > int64(len(filtered)) {
			end = int64(len(filtered))
		}
		return filtered[start:end], nil
	}
	return s.repo.GetReturns(offset, limit, recipientID)
}
