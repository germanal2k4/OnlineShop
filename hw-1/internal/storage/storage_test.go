package storage_test

import (
	"os"
	"testing"
	"time"

	"hw-1/internal/models"
	"hw-1/internal/storage"

	"github.com/stretchr/testify/assert"
)

// решил написать небольшое юнит тестирование
const testFile = "test_orders.json"

// setupStorage создаёт тестовое хранилище в памяти
func setupStorage(t *testing.T) *storage.OrderStorage {
	_ = os.Remove(testFile) // Удаляем тестовый файл перед началом тестов
	st, err := storage.NewOrderStorage(testFile)
	assert.NoError(t, err)
	return st
}

// TestAcceptOrder проверяет приём заказа от курьера
func TestAcceptOrder(t *testing.T) {
	st := setupStorage(t)

	orderID := "order123"
	userID := "user42"
	deadline := time.Now().Add(24 * time.Hour)

	err := st.AcceptOrderFromCourier(orderID, userID, deadline)
	assert.NoError(t, err, "Приём заказа не должен выдавать ошибку")

	orders, err := st.GetOrders(userID, 0, false)
	assert.NoError(t, err)
	assert.Len(t, orders, 1)
	assert.Equal(t, orderID, orders[0].ID)
	assert.Equal(t, models.OrderStateAccepted, orders[0].State)
}

// TestDeliverOrder проверяет выдачу заказа клиенту
func TestDeliverOrder(t *testing.T) {
	st := setupStorage(t)

	orderID := "order789"
	userID := "user88"
	deadline := time.Now().Add(24 * time.Hour)

	_ = st.AcceptOrderFromCourier(orderID, userID, deadline)

	err := st.DeliverOrReturnClientOrders(userID, []string{orderID}, "deliver")
	assert.NoError(t, err, "Выдача заказа должна проходить без ошибок")

	orders, err := st.GetOrders(userID, 0, false)
	assert.NoError(t, err)
	assert.Len(t, orders, 1)
	assert.Equal(t, models.OrderStateDelivered, orders[0].State)
}

// TestClientReturn проверяет возврат заказа от клиента (в течение 48 часов)
func TestClientReturn(t *testing.T) {
	st := setupStorage(t)

	orderID := "order987"
	userID := "user99"
	deadline := time.Now().Add(24 * time.Hour)

	_ = st.AcceptOrderFromCourier(orderID, userID, deadline)

	_ = st.DeliverOrReturnClientOrders(userID, []string{orderID}, "deliver")

	err := st.DeliverOrReturnClientOrders(userID, []string{orderID}, "return")
	assert.NoError(t, err, "Клиент должен иметь возможность вернуть заказ в течение 48 часов")

	orders, err := st.GetOrders(userID, 0, false)
	assert.NoError(t, err)
	assert.Len(t, orders, 1)
	assert.Equal(t, models.OrderStateClientRtn, orders[0].State)
}

// TestGetOrders проверяет получение списка заказов пользователя
func TestGetOrders(t *testing.T) {
	st := setupStorage(t)

	_ = st.AcceptOrderFromCourier("order1", "user100", time.Now().Add(24*time.Hour))
	_ = st.AcceptOrderFromCourier("order2", "user100", time.Now().Add(24*time.Hour))

	orders, err := st.GetOrders("user100", 0, false)
	assert.NoError(t, err)
	assert.Len(t, orders, 2)
}

// TestGetReturns проверяет, что возвраты корректно выводятся с пагинацией
func TestGetReturns(t *testing.T) {
	st := setupStorage(t)

	_ = st.AcceptOrderFromCourier("orderA", "userA", time.Now().Add(24*time.Hour))
	_ = st.AcceptOrderFromCourier("orderB", "userA", time.Now().Add(24*time.Hour))
	_ = st.DeliverOrReturnClientOrders("userA", []string{"orderA"}, "deliver")
	_ = st.DeliverOrReturnClientOrders("userA", []string{"orderB"}, "deliver")

	_ = st.DeliverOrReturnClientOrders("userA", []string{"orderA"}, "return")

	returns, err := st.GetReturns(1, 10)
	assert.NoError(t, err)
	assert.Len(t, returns, 1)
	assert.Equal(t, "orderA", returns[0].ID)
}

// TestOrderHistory проверяет, что история заказов корректно обновляется
func TestOrderHistory(t *testing.T) {
	st := setupStorage(t)

	_ = st.AcceptOrderFromCourier("order100", "userX", time.Now().Add(24*time.Hour))
	_ = st.AcceptOrderFromCourier("order200", "userX", time.Now().Add(24*time.Hour))
	_ = st.DeliverOrReturnClientOrders("userX", []string{"order100"}, "deliver")

	history, err := st.GetOrderHistory()
	assert.NoError(t, err)
	assert.Len(t, history, 2)
	assert.Equal(t, "order100", history[0].ID, "Последний изменённый заказ должен быть первым в истории")
}
