package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"gitlab.ozon.dev/qwestard/homework/internal/storage"
)

type Handler struct {
	st *storage.OrderStorage
}

func New(st *storage.OrderStorage) *Handler {
	return &Handler{st: st}
}

func (h *Handler) Execute(cmd string, args []string) error {
	commands := map[string]func([]string){
		"help":                h.printHelp,
		"exit":                h.handleExit,
		"accept":              h.handleAccept,
		"return_courier":      h.handleReturnCourier,
		"deliver":             h.handleDeliver,
		"clientreturn":        h.handleClientReturn,
		"list":                h.handleList,
		"returns":             h.handleReturns,
		"history":             func(args []string) { h.handleHistory() },
		"accept_from_courier": h.acceptOrdersFromCourier,
	}

	if fn, ok := commands[cmd]; ok {
		fn(args)
	} else {
		return errors.New("неизвестная команда. Введите 'help' для справки")
	}
	return nil
}

func (h *Handler) printHelp(args []string) {
	fmt.Println(`Доступные команды:
  help
    - выводит справку
  exit
    - завершает программу
  accept <orderID> <userID> <deadline RFC3339>
    - принять заказ от курьера
  return_courier <orderID>
    - вернуть заказ курьеру (если заказ в состоянии accepted или clientreturn)
  deliver <userID> <orderID1> [orderID2 ...]
    - выдать заказы клиенту (перевод в delivered)
  clientreturn <userID> <orderID1> [orderID2 ...]
    - принять возврат от клиента (перевод в client_rtn)
  list <userID> [lastN=0] [onlyInPVZ=false]
    - список заказов пользователя
  returns [pageIndex=1] [pageSize=10]
    - список возвратов с пагинацией
  history
    - история заказов
  accept_from_courier <filename>
    - принять заказы из файла JSON (массив заказов)`)
}

func (h *Handler) handleExit(args []string) {
	fmt.Println("Выход из приложения.")
	os.Exit(0)
}
func (h *Handler) handleAccept(args []string) {
	if len(args) != 6 {
		fmt.Println("Формат: accept <orderID> <userID> <deadline RFC3339> <packaging> <weight> <baseCost>")
		return
	}
	orderID := args[0]
	userID := args[1]
	deadlineStr := args[2]
	packagingOption := args[3]
	weight, err := strconv.ParseFloat(args[4], 64)
	if err != nil {
		fmt.Printf("Ошибка разбора веса: %v\n", err)
		return
	}
	baseCost, err := strconv.ParseFloat(args[5], 64)
	if err != nil {
		fmt.Printf("Ошибка разбора базовой стоимости: %v\n", err)
		return
	}
	deadline, err := time.Parse(time.RFC3339, deadlineStr)
	if err != nil {
		fmt.Printf("Ошибка разбора даты: %v\n", err)
		return
	}
	err = h.st.AcceptOrderFromCourier(orderID, userID, deadline, packagingOption, weight, baseCost)
	if err != nil {
		fmt.Printf("Ошибка accept: %v\n", err)
		return
	}
	fmt.Printf("Заказ %s принят для пользователя %s (deadline=%s, упаковка=%s, вес=%.2f, стоимость=%.2f)\n",
		orderID, userID, deadline, packagingOption, weight, baseCost)
}

func (h *Handler) handleReturnCourier(args []string) {
	if len(args) != 1 {
		fmt.Println("Формат: return_courier <orderID>")
		return
	}
	orderID := args[0]
	err := h.st.ReturnOrderToCourier(orderID)
	if err != nil {
		fmt.Printf("Ошибка return_courier: %v\n", err)
		return
	}

	fmt.Printf("Заказ %s возвращён курьеру\n", orderID)

}

func (h *Handler) handleDeliver(args []string) {
	if len(args) < 2 {
		fmt.Println("Формат: deliver <userID> <orderID> [orderID2 ...]")
		return
	}
	userID := args[0]
	orderIDs := args[1:]
	err := h.st.DeliverOrReturnClientOrders(userID, orderIDs, "deliver")
	if err != nil {
		fmt.Printf("Ошибка deliver: %v\n", err)
	} else {
		fmt.Printf("Выдано %d заказ(ов) пользователю %s: %v\n", len(orderIDs), userID, orderIDs)
	}
}

func (h *Handler) handleClientReturn(args []string) {
	if len(args) < 2 {
		fmt.Println("Формат: clientreturn <userID> <orderID> [orderID2 ...]")
		return
	}
	userID := args[0]
	orderIDs := args[1:]
	err := h.st.DeliverOrReturnClientOrders(userID, orderIDs, "return")
	if err != nil {
		fmt.Printf("Ошибка clientreturn: %v\n", err)
		return
	}

	fmt.Printf("Принят возврат %d заказ(ов) от %s: %v\n", len(orderIDs), userID, orderIDs)

}

func (h *Handler) handleList(args []string) {
	if len(args) < 1 {
		fmt.Println("Формат: list <userID> [lastN=0] [onlyInPVZ=false]")
		return
	}
	userID := args[0]
	var lastN int
	var onlyInPVZ bool
	if len(args) >= 2 {
		if n, err := strconv.Atoi(args[1]); err == nil {
			lastN = n
		}
	}
	if len(args) >= 3 && args[2] == "true" {
		onlyInPVZ = true
	}
	orders, err := h.st.GetOrders(userID, lastN, onlyInPVZ)
	if err != nil {
		fmt.Printf("Ошибка GetOrders: %v\n", err)
		return
	}
	if len(orders) == 0 {
		fmt.Printf("У пользователя %s нет заказов\n", userID)
		return
	}
	fmt.Printf("Список заказов пользователя %s:\n", userID)
	for _, o := range orders {
		fmt.Printf("  ID=%s, State=%s, Deadline=%s\n",
			o.ID, o.CurrentState(), o.StorageDeadline.Format(time.RFC3339))
	}
}

func (h *Handler) handleReturns(args []string) {
	pageIndex := 1
	pageSize := 10
	if len(args) >= 1 {
		if n, err := strconv.Atoi(args[0]); err == nil {
			pageIndex = n
		}
	}
	if len(args) >= 2 {
		if n, err := strconv.Atoi(args[1]); err == nil {
			pageSize = n
		}
	}
	ret, err := h.st.GetReturns(pageIndex, pageSize)
	if err != nil {
		fmt.Printf("Ошибка GetReturns: %v\n", err)
		return
	}
	if len(ret) == 0 {
		fmt.Println("На данной странице возвратов нет.")
		return
	}
	fmt.Printf("Страница %d, возвраты:\n", pageIndex)
	for _, r := range ret {
		fmt.Printf("  ID=%s, State=%s\n", r.ID, r.CurrentState())
	}
}

func (h *Handler) handleHistory() {
	history, err := h.st.GetOrderHistory()
	if err != nil {
		fmt.Printf("Ошибка GetOrderHistory: %v\n", err)
		return
	}
	if len(history) == 0 {
		fmt.Println("История заказов пуста.")
		return
	}
	fmt.Println("История заказов (последние изменения — первыми):")
	for _, o := range history {
		fmt.Printf("  ID=%s, State=%s, LastChange=%s\n",
			o.ID, o.CurrentState(), o.LastStateChange.Format(time.RFC3339))
	}
}

func (h *Handler) acceptOrdersFromCourier(args []string) {
	if len(args) != 1 {
		fmt.Println("Формат: accept_from_courier <filename>")
		return
	}
	fileName := args[0]
	file, err := os.Open(fileName)
	if err != nil {
		fmt.Printf("Ошибка открытия файла %s: %v\n", fileName, err)
		return
	}
	defer file.Close()
	var orders []struct {
		ID              string  `json:"id"`
		RecipientID     string  `json:"recipient_id"`
		StorageDeadline string  `json:"storage_deadline"`
		Packaging       string  `json:"packaging"`
		Weight          float64 `json:"weight"`
		BaseCost        float64 `json:"base_cost"`
	}

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&orders); err != nil {
		fmt.Printf("Ошибка декодирования JSON: %v\n", err)
		return
	}

	for _, o := range orders {
		deadline, err := time.Parse(time.RFC3339, o.StorageDeadline)
		if err != nil {
			fmt.Printf("Неверный формат даты для заказа %s: %v\n", o.ID, err)
			continue
		}
		err = h.st.AcceptOrderFromCourier(o.ID, o.RecipientID, deadline, o.Packaging, o.Weight, o.BaseCost)
		if err != nil {
			fmt.Printf("Ошибка при принятии заказа %s: %v\n", o.ID, err)
			continue
		}
		fmt.Printf("Заказ %s принят для пользователя %s\n", o.ID, o.RecipientID)
	}
}
