package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"hw-1/internal/models"
)

// OrderStorage использую мьютекс так мапа не конкурентная предпологая дальнейшее развитие программы думаю
// что она будет предпологать, что к файлу-базе будут стучаться несколько горутин и чтобы гипотетически не словить панику
// юзаю мьютекс
type OrderStorage struct {
	mu       sync.Mutex
	orders   map[string]*models.Order
	dataFile string
}

// NewOrderStorage метод для создания хранилища
func NewOrderStorage(dataFile string) (*OrderStorage, error) {
	st := &OrderStorage{
		orders:   make(map[string]*models.Order),
		dataFile: dataFile,
	}
	// пытаемся подгрузить данные
	if err := st.loadFromFile(); err != nil {
		// если не выходит то создаем файл заново
		_, err = os.Create(dataFile)
		if err != nil {
			return st, err
		}
	}
	return st, nil
}

// loadFromFile метод сгружает все данные из имеющегося файла в мапу если этого не поулчается сделать , то вернет ошибку
func (st *OrderStorage) loadFromFile() error {
	file, err := os.Open(st.dataFile)
	if err != nil {
		return err
	}
	defer file.Close()

	var orderList []*models.Order
	if err := json.NewDecoder(file).Decode(&orderList); err != nil {
		return err
	}

	st.orders = make(map[string]*models.Order, len(orderList))
	for _, o := range orderList {
		st.orders[o.ID] = o
	}
	return nil
}

// saveToFile сохраняет данные в наш файл с хранением всей информации
func (st *OrderStorage) saveToFile() error {
	file, err := os.Create(st.dataFile)
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

// now добавлен для инкапсуляции кода
func now() time.Time {
	return time.Now().UTC()
}

// updateOrderState обновляю положение заказа
func (st *OrderStorage) updateOrderState(o *models.Order, newState models.OrderState) {
	o.State = newState
	o.LastStateChange = now()
}

// AcceptOrderFromCourier функция для принятия заказа от курьера
func (st *OrderStorage) AcceptOrderFromCourier(orderID, recipientID string, deadline time.Time) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	if _, exists := st.orders[orderID]; exists {
		return errors.New("заказ с таким ID уже существует (принят ранее)")
	}
	if deadline.Before(time.Now()) {
		return errors.New("срок хранения уже истёк, не можем принять заказ")
	}
	// формирую заказ
	t := now()
	order := &models.Order{
		ID:              orderID,
		RecipientID:     recipientID,
		StorageDeadline: deadline,
		State:           models.OrderStateAccepted,
		AcceptedAt:      &t,
		LastStateChange: t,
	}
	// записываю в файл
	st.orders[orderID] = order
	if err := st.saveToFile(); err != nil {
		return fmt.Errorf("сбой при сохранении файла: %w", err)
	}
	return nil
}

// ReturnOrderToCourier функция для возврата заказа курьеру
func (st *OrderStorage) ReturnOrderToCourier(orderID string) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	order, exists := st.orders[orderID]
	if !exists {
		return errors.New("заказ не найден")
	}
	if order.State != models.OrderStateAccepted {
		return errors.New("заказ должен быть в состоянии 'accepted', чтобы вернуть курьеру")
	}
	if time.Now().Before(order.StorageDeadline) {
		return errors.New("срок хранения ещё не вышел, вернуть заказ курьеру нельзя")
	}
	// обновляю статус заказа
	t := now()
	st.updateOrderState(order, models.OrderStateReturned)
	order.ReturnedAt = &t
	// удаляю из мапы
	delete(st.orders, orderID)
	// сохраняю изменения в файл
	if err := st.saveToFile(); err != nil {
		return fmt.Errorf("сбой при сохранении файла: %w", err)
	}
	return nil
}

// DeliverOrReturnClientOrders доставить или вернуть заказ клиенту
func (st *OrderStorage) DeliverOrReturnClientOrders(userID string, orderIDs []string, action string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	// ищу заказ
	for _, id := range orderIDs {
		o, ok := st.orders[id]
		if !ok {
			return fmt.Errorf("заказ %s не найден", id)
		}
		if o.RecipientID != userID {
			return fmt.Errorf("заказ %s принадлежит другому пользователю", id)
		}
	}

	switch action {
	// если надо доставить
	case "deliver":
		for _, id := range orderIDs {
			o := st.orders[id]
			// проверяю доставлен ли он
			if o.State != models.OrderStateAccepted {
				return fmt.Errorf("заказ %s не в состоянии 'accepted', нельзя выдать", o.ID)
			}
			// проверяю не вышел ли срок
			if time.Now().After(o.StorageDeadline) {
				return fmt.Errorf("срок хранения заказа %s истёк, нельзя выдать", o.ID)
			}
			// если все ок обновляю
			t := now()
			st.updateOrderState(o, models.OrderStateDelivered)
			o.DeliveredAt = &t
		}
	case "return":
		for _, id := range orderIDs {
			o := st.orders[id]
			// проверяю нет ли дефектов
			if o.State != models.OrderStateDelivered {
				return fmt.Errorf("заказ %s не в состоянии 'delivered', нельзя вернуть", o.ID)
			}
			if o.DeliveredAt == nil {
				return fmt.Errorf("заказ %s не имеет даты выдачи", o.ID)
			}
			// прошло ли более двух суток
			if time.Since(*o.DeliveredAt) > 48*time.Hour {
				return fmt.Errorf("с момента выдачи заказа %s прошло более 2 суток, возврат невозможен", o.ID)
			}
			t := now()
			st.updateOrderState(o, models.OrderStateClientRtn)
			o.ClientReturnAt = &t
		}
	default:
		return errors.New("неизвестное действие (deliver или return)")
	}
	// сохраняю изменения в файл
	if err := st.saveToFile(); err != nil {
		return fmt.Errorf("сбой при сохранении файла: %w", err)
	}
	return nil
}

// GetOrders функция для получения списка заказов
func (st *OrderStorage) GetOrders(userID string, lastN int, onlyInPVZ bool) ([]*models.Order, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	var result []*models.Order
	for _, o := range st.orders {
		if o.RecipientID == userID {
			if onlyInPVZ {
				if o.State == models.OrderStateAccepted {
					result = append(result, o)
				}
			} else {
				result = append(result, o)
			}
		}
	}
	// сортирую и вывожу
	sortOrdersByLastChangeDesc(result)
	if lastN > 0 && len(result) > lastN {
		result = result[:lastN]
	}
	return result, nil
}

// GetReturns список возвратов
func (st *OrderStorage) GetReturns(pageIndex, pageSize int) ([]*models.Order, error) {
	if pageIndex < 1 {
		return nil, errors.New("pageIndex должен быть >= 1")
	}
	if pageSize < 1 {
		return nil, errors.New("pageSize должен быть >= 1")
	}
	// реализуется та же логика что и в предыдщей функции
	st.mu.Lock()
	defer st.mu.Unlock()

	var returns []*models.Order
	for _, o := range st.orders {
		if o.State == models.OrderStateClientRtn {
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

// GetOrderHistory возвращаем историю
func (st *OrderStorage) GetOrderHistory() ([]*models.Order, error) {
	st.mu.Lock()
	defer st.mu.Unlock()

	all := make([]*models.Order, 0, len(st.orders))
	for _, o := range st.orders {
		all = append(all, o)
	}
	sortOrdersByLastChangeDesc(all)
	return all, nil
}

// Сортировка по последнему измнению опять же добавил для инкапсуляции
func sortOrdersByLastChangeDesc(orders []*models.Order) {
	sort.Slice(orders, func(i, j int) bool {
		return orders[i].LastStateChange.After(orders[j].LastStateChange)
	})
}
