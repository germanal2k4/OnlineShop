package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"gitlab.ozon.dev/qwestard/homework/internal/packaging"
	"os"
	"sort"
	"strings"
	"time"

	"gitlab.ozon.dev/qwestard/homework/internal/models"
)

type OrderStorage struct {
	orders   map[string]*models.Order
	dataFile string
}

func New(dataFile string) (*OrderStorage, error) {
	st := &OrderStorage{
		orders:   make(map[string]*models.Order),
		dataFile: dataFile,
	}
	if err := st.loadFromFile(); err != nil {
		return st, err
	}
	return st, nil
}

func (st *OrderStorage) loadFromFile() error {
	file, err := os.OpenFile(st.dataFile, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	var orderList []*models.Order
	if err := json.NewDecoder(file).Decode(&orderList); err != nil {
		return fmt.Errorf("ошибка декодирования файла: %w", err)
	}

	st.orders = make(map[string]*models.Order, len(orderList))
	for _, o := range orderList {
		st.orders[o.ID] = o
	}
	return nil
}

func (st *OrderStorage) saveToFile() error {
	file, err := os.OpenFile(st.dataFile, os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	orderList := make([]*models.Order, 0, len(st.orders))
	for _, o := range st.orders {
		orderList = append(orderList, o)
	}

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	return enc.Encode(orderList)
}

func now() time.Time {
	return time.Now().UTC()
}

func (st *OrderStorage) deleteOrder(orderID string) error {
	delete(st.orders, orderID)
	return st.saveToFile()
}

func (st *OrderStorage) AcceptOrderFromCourier(orderID, recipientID string, deadline time.Time, packagingOption string, weight float64, baseCost float64) error {
	if _, exists := st.orders[orderID]; exists {
		return errors.New("заказ с таким ID уже существует (принят ранее)")
	}
	if deadline.Before(time.Now()) {
		return errors.New("срок хранения уже истёк, не можем принять заказ")
	}

	var totalPackagingCost float64
	packTypes := strings.Split(packagingOption, "+")
	for _, pStr := range packTypes {
		p, err := packaging.NewPackaging(pStr)
		if err != nil {
			return err
		}
		if err := p.Validate(weight); err != nil {
			return err
		}
		totalPackagingCost += p.Cost()
	}

	t := now()
	order := &models.Order{
		ID:              orderID,
		RecipientID:     recipientID,
		StorageDeadline: deadline,
		AcceptedAt:      t,
		LastStateChange: t,
		Weight:          weight,
		Cost:            baseCost + totalPackagingCost,
		Packaging:       packagingOption,
	}
	order.UpdateState(models.OrderStateAccepted)
	st.orders[orderID] = order
	if err := st.saveToFile(); err != nil {
		return fmt.Errorf("сбой при сохранении файла: %w", err)
	}
	return nil
}

func (st *OrderStorage) ReturnOrderToCourier(orderID string) error {
	order, exists := st.orders[orderID]
	if !exists {
		return errors.New("заказ не найден")
	}

	if order.CurrentState() == models.OrderStateAccepted {
		return st.deleteOrder(orderID)
	}

	if order.CurrentState() == models.OrderStateClientRtn {
		t := now()
		order.UpdateState(models.OrderStateReturned)
		order.ReturnedAt = t
		return st.saveToFile()
	}

	return errors.New("заказ находится в некорректном состоянии для возврата курьеру")
}

func (st *OrderStorage) validateOrdersForDelivery(userID string, orderIDs []string) ([]*models.Order, error) {
	validOrders := make([]*models.Order, 0, len(orderIDs))
	var invalidErrors []string

	for _, id := range orderIDs {
		o, ok := st.orders[id]
		if !ok {
			invalidErrors = append(invalidErrors, fmt.Sprintf("заказ %s не найден", id))
			continue
		}
		if o.RecipientID != userID {
			invalidErrors = append(invalidErrors, fmt.Sprintf("заказ %s принадлежит другому пользователю", id))
			continue
		}
		if o.CurrentState() != models.OrderStateAccepted {
			invalidErrors = append(invalidErrors, fmt.Sprintf("заказ %s не в состоянии 'accepted'", id))
			continue
		}
		if time.Now().After(o.StorageDeadline) {
			invalidErrors = append(invalidErrors, fmt.Sprintf("срок хранения заказа %s истёк", id))
			continue
		}
		validOrders = append(validOrders, o)
	}
	if len(invalidErrors) > 0 {
		return nil, errors.New("валидация доставки не пройдена: " + fmt.Sprintf("%v", invalidErrors))
	}
	return validOrders, nil
}

func validateReturnOrder(o *models.Order, userID string) error {
	if o.RecipientID != userID {
		return fmt.Errorf("заказ %s принадлежит другому пользователю", o.ID)
	}
	if o.CurrentState() != models.OrderStateDelivered {
		return fmt.Errorf("заказ %s не в состоянии 'delivered'", o.ID)
	}
	if o.DeliveredAt.IsZero() {
		return fmt.Errorf("заказ %s не имеет даты выдачи", o.ID)
	}
	if time.Since(o.DeliveredAt) > 48*time.Hour {
		return fmt.Errorf("с момента выдачи заказа %s прошло более 2 суток", o.ID)
	}
	return nil
}

func (st *OrderStorage) validateOrdersForReturn(userID string, orderIDs []string) ([]*models.Order, error) {
	var validOrders []*models.Order
	var invalidErrors []string

	for _, id := range orderIDs {
		o, ok := st.orders[id]
		if !ok {
			invalidErrors = append(invalidErrors, fmt.Sprintf("заказ %s не найден", id))
			continue
		}
		if err := validateReturnOrder(o, userID); err != nil {
			invalidErrors = append(invalidErrors, err.Error())
			continue
		}
		validOrders = append(validOrders, o)
	}
	if len(invalidErrors) > 0 {
		return nil, fmt.Errorf("валидация возврата не пройдена: %s", strings.Join(invalidErrors, "; "))
	}
	return validOrders, nil
}
func (st *OrderStorage) DeliverOrReturnClientOrders(userID string, orderIDs []string, action string) error {
	var ordersToProcess []*models.Order
	var err error

	switch action {
	case "deliver":
		ordersToProcess, err = st.validateOrdersForDelivery(userID, orderIDs)
	case "return":
		ordersToProcess, err = st.validateOrdersForReturn(userID, orderIDs)
	default:
		return errors.New("неизвестное действие (deliver или return)")
	}
	if err != nil {
		return err
	}

	t := now()
	switch action {
	case "deliver":
		for _, o := range ordersToProcess {
			o.UpdateState(models.OrderStateDelivered)
			o.DeliveredAt = t
		}
	case "return":
		for _, o := range ordersToProcess {
			o.UpdateState(models.OrderStateClientRtn)
			o.ClientReturnAt = t
		}
	}
	if err := st.saveToFile(); err != nil {
		return fmt.Errorf("сбой при сохранении файла: %w", err)
	}
	return nil
}

func (st *OrderStorage) GetOrders(userID string, lastN int, onlyInPVZ bool) ([]*models.Order, error) {
	result := make([]*models.Order, 0)
	for _, o := range st.orders {
		if o.RecipientID != userID {
			continue
		}
		include := true
		if onlyInPVZ && o.CurrentState() != models.OrderStateAccepted {
			include = false
		}
		if include {
			result = append(result, o)
		}
	}
	sortOrdersByLastChangeDesc(result)
	if lastN > 0 && len(result) > lastN {
		result = result[:lastN]
	}
	return result, nil
}

func (st *OrderStorage) GetReturns(pageIndex, pageSize int) ([]*models.Order, error) {
	if pageIndex < 1 {
		return nil, errors.New("pageIndex должен быть >= 1")
	}
	if pageSize < 1 {
		return nil, errors.New("pageSize должен быть >= 1")
	}

	var returns []*models.Order
	for _, o := range st.orders {
		if o.CurrentState() == models.OrderStateClientRtn {
			returns = append(returns, o)
		}
	}

	sortOrdersByLastChangeDesc(returns)

	start := (pageIndex - 1) * pageSize
	if start >= len(returns) {
		return []*models.Order{}, nil
	}
	end := start + pageSize
	if end > len(returns) {
		end = len(returns)
	}
	return returns[start:end], nil
}

func (st *OrderStorage) GetOrderHistory() ([]*models.Order, error) {
	all := make([]*models.Order, 0, len(st.orders))
	for _, o := range st.orders {
		all = append(all, o)
	}
	sortOrdersByLastChangeDesc(all)
	return all, nil
}

func sortOrdersByLastChangeDesc(orders []*models.Order) {
	sort.Slice(orders, func(i, j int) bool {
		return orders[i].LastStateChange.After(orders[j].LastStateChange)
	})
}
