package wrapper

import (
	"gitlab.ozon.dev/qwestard/homework/internal/models"
	"gitlab.ozon.dev/qwestard/homework/internal/service"
)

type OrderWrapper struct {
	orderService *service.OrderService
}

func NewOrderWrapper(svc *service.OrderService) *OrderWrapper {
	return &OrderWrapper{
		orderService: svc,
	}
}

func (w *OrderWrapper) GetOrderByID(id string) (*models.Order, error) {
	return w.orderService.GetOrderByID(id)
}

func (w *OrderWrapper) CreateOrder(order *models.Order) error {
	return w.orderService.CreateOrder(order)
}

func (w *OrderWrapper) UpdateOrder(order *models.Order) error {
	return w.orderService.UpdateOrder(order)
}

func (w *OrderWrapper) DeleteOrder(id string) error {
	return w.orderService.DeleteOrder(id)
}

func (w *OrderWrapper) DeliverOrder(id string) error {
	return w.orderService.DeliverOrder(id)
}

func (w *OrderWrapper) ClientReturnOrder(id string) error {
	return w.orderService.ClientReturnOrder(id)
}

func (w *OrderWrapper) AcceptOrder(id string) error {
	return w.orderService.AcceptOrder(id)
}

func (w *OrderWrapper) CourierReturnOrder(id string) error {
	return w.orderService.CourierReturnOrder(id)
}

func (w *OrderWrapper) RefreshActiveOrders() error {
	return w.orderService.RefreshActiveOrders()
}

func (w *OrderWrapper) ListActiveOrders() ([]*models.Order, error) {
	return w.orderService.ListActiveOrders()
}

func (w *OrderWrapper) ListHistoryOrders() ([]*models.Order, error) {
	return w.orderService.ListHistoryOrders()
}

func (w *OrderWrapper) GetReturns(offset, limit int64, recipientID string) ([]*models.Order, error) {
	orders, err := w.orderService.ListReturns(offset, limit, recipientID)
	if err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		go func() {
			err := w.orderService.RefreshActiveOrders()
			if err != nil {
				return
			}
		}()
	}
	return orders, nil
}
