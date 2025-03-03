package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"gitlab.ozon.dev/qwestard/homework/internal/packaging"
	"os"
	"strconv"
	"strings"
	"time"

	"gitlab.ozon.dev/qwestard/homework/internal/storage"
)

var ErrExit = errors.New("exit")

type Command struct {
	Name    string
	Handler func([]string) error
}

type Handler struct {
	st       *storage.OrderStorage
	commands []Command
}

func New(st *storage.OrderStorage) *Handler {
	h := &Handler{
		st: st,
	}
	h.initCommands()
	return h
}

func (h *Handler) initCommands() {
	h.commands = []Command{
		{"help", h.printHelp},
		{"exit", h.handleExit},
		{"accept", h.handleAccept},
		{"return_courier", h.handleReturnCourier},
		{"deliver", h.handleDeliver},
		{"clientreturn", h.handleClientReturn},
		{"list", h.handleList},
		{"returns", h.handleReturns},
		{"history", h.handleHistory},
		{"accept_from_courier", h.acceptOrdersFromCourier},
	}
}

func (h *Handler) Execute(cmd string, args []string) error {
	for _, c := range h.commands {
		if c.Name == cmd {
			return c.Handler(args)
		}
	}
	return errors.New("неизвестная команда. Введите 'help' для справки")
}

func (h *Handler) printHelp([]string) error {
	fmt.Println(`Доступные команды:
  help
    - выводит справку
  exit
    - завершает программу
  accept <orderID> <userID> <deadline RFC3339> <packaging> <weight> <baseCost>
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
	return nil
}

func (h *Handler) handleExit([]string) error {
	fmt.Println("Выход из приложения.")
	return ErrExit
}

func (h *Handler) handleAccept(args []string) error {
	if len(args) != 3 && len(args) != 6 {
		return errors.New("формат: accept <orderID> <userID> <deadline RFC3339> [<packaging> <weight> <baseCost>]")
	}
	orderID := args[0]
	userID := args[1]
	deadlineStr := args[2]
	deadline, err := time.Parse(time.RFC3339, deadlineStr)
	if err != nil {
		return fmt.Errorf("ошибка разбора даты: %w", err)
	}

	var packagingSlice []packaging.PackagingType
	var weight, baseCost float64
	if len(args) == 6 {
		raw := strings.Split(args[3], "+")
		for _, s := range raw {
			pt := packaging.PackagingType(strings.ToLower(strings.TrimSpace(s)))
			packagingSlice = append(packagingSlice, pt)
		}
		weight, err = strconv.ParseFloat(args[4], 64)
		if err != nil {
			return fmt.Errorf("ошибка разбора веса: %w", err)
		}
		baseCost, err = strconv.ParseFloat(args[5], 64)
		if err != nil {
			return fmt.Errorf("ошибка разбора базовой стоимости: %w", err)
		}
	}

	req := storage.AcceptOrderFromCourierRequest{
		OrderID:     orderID,
		RecipientID: userID,
		Deadline:    deadline,
		Packaging:   packagingSlice,
		Weight:      weight,
		BaseCost:    baseCost,
	}
	err = h.st.AcceptOrderFromCourier(req)
	if err != nil {
		return fmt.Errorf("ошибка accept: %w", err)
	}
	if len(packagingSlice) > 0 {
		fmt.Printf("Заказ %s принят для пользователя %s (deadline=%s, упаковка=%v, вес=%.2f, базовая стоимость=%.2f)\n",
			orderID, userID, deadline, packagingSlice, weight, baseCost)
	} else {
		fmt.Printf("Заказ %s принят для пользователя %s (deadline=%s)\n",
			orderID, userID, deadline)
	}
	return nil
}

func (h *Handler) handleReturnCourier(args []string) error {
	if len(args) != 1 {
		return errors.New("формат: return_courier <orderID>")
	}
	orderID := args[0]
	err := h.st.ReturnOrderToCourier(orderID)
	if err != nil {
		return fmt.Errorf("ошибка return_courier: %w", err)
	}
	fmt.Printf("Заказ %s возвращён курьеру\n", orderID)
	return nil
}

func (h *Handler) handleDeliver(args []string) error {
	if len(args) < 2 {
		return errors.New("формат: deliver <userID> <orderID> [orderID2 ...]")
	}
	userID := args[0]
	orderIDs := args[1:]
	err := h.st.DeliverOrReturnClientOrders(userID, orderIDs, "deliver")
	if err != nil {
		return fmt.Errorf("ошибка deliver: %w", err)
	}
	fmt.Printf("Выдано %d заказ(ов) пользователю %s: %v\n", len(orderIDs), userID, orderIDs)
	return nil
}

func (h *Handler) handleClientReturn(args []string) error {
	if len(args) < 2 {
		return errors.New("формат: clientreturn <userID> <orderID> [orderID2 ...]")
	}
	userID := args[0]
	orderIDs := args[1:]
	err := h.st.DeliverOrReturnClientOrders(userID, orderIDs, "return")
	if err != nil {
		return fmt.Errorf("ошибка clientreturn: %w", err)
	}
	fmt.Printf("Принят возврат %d заказ(ов) от %s: %v\n", len(orderIDs), userID, orderIDs)
	return nil
}

func (h *Handler) handleList(args []string) error {
	if len(args) < 1 {
		return errors.New("формат: list <userID> [lastN=0] [onlyInPVZ=false]")
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
		return fmt.Errorf("ошибка GetOrders: %w", err)
	}
	if len(orders) == 0 {
		fmt.Printf("У пользователя %s нет заказов\n", userID)
		return nil
	}
	fmt.Printf("Список заказов пользователя %s:\n", userID)
	for _, o := range orders {
		fmt.Printf("  ID=%s, State=%s, Deadline=%s\n",
			o.ID, o.CurrentState(), o.StorageDeadline.Format(time.RFC3339))
	}
	return nil
}

func (h *Handler) handleReturns(args []string) error {
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
		return fmt.Errorf("ошибка GetReturns: %w", err)
	}
	if len(ret) == 0 {
		fmt.Println("На данной странице возвратов нет.")
		return nil
	}
	fmt.Printf("Страница %d, возвраты:\n", pageIndex)
	for _, r := range ret {
		fmt.Printf("  ID=%s, State=%s\n", r.ID, r.CurrentState())
	}
	return nil
}

func (h *Handler) handleHistory([]string) error {
	history, err := h.st.GetOrderHistory()
	if err != nil {
		return fmt.Errorf("ошибка GetOrderHistory: %w", err)
	}
	if len(history) == 0 {
		fmt.Println("История заказов пуста.")
		return nil
	}
	fmt.Println("История заказов (последние изменения — первыми):")
	for _, o := range history {
		fmt.Printf("  ID=%s, State=%s, LastChange=%s\n",
			o.ID, o.CurrentState(), o.LastStateChange.Format(time.RFC3339))
	}
	return nil
}

func (h *Handler) acceptOrdersFromCourier(args []string) error {
	if len(args) != 1 {
		return errors.New("формат: accept_from_courier <filename>")
	}
	fileName := args[0]
	file, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("ошибка открытия файла %s: %w", fileName, err)
	}
	defer file.Close()

	var orders []struct {
		ID              string   `json:"id"`
		RecipientID     string   `json:"recipient_id"`
		StorageDeadline string   `json:"storage_deadline"`
		Packaging       []string `json:"packaging"`
		Weight          float64  `json:"weight"`
		BaseCost        float64  `json:"base_cost"`
	}

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&orders); err != nil {
		return fmt.Errorf("ошибка декодирования JSON: %w", err)
	}

	for _, o := range orders {
		deadline, err := time.Parse(time.RFC3339, o.StorageDeadline)
		if err != nil {
			fmt.Printf("Неверный формат даты для заказа %s: %v\n", o.ID, err)
			continue
		}
		var packagingSlice []packaging.PackagingType

		for _, s := range o.Packaging {
			pt := packaging.PackagingType(strings.ToLower(strings.TrimSpace(s)))
			packagingSlice = append(packagingSlice, pt)
		}
		req := storage.AcceptOrderFromCourierRequest{
			OrderID:     o.ID,
			RecipientID: o.RecipientID,
			Deadline:    deadline,
			Packaging:   packagingSlice,
			Weight:      o.Weight,
			BaseCost:    o.BaseCost,
		}
		err = h.st.AcceptOrderFromCourier(req)
		if err != nil {
			fmt.Printf("Ошибка при принятии заказа %s: %v\n", o.ID, err)
			continue
		}
		fmt.Printf("Заказ %s принят для пользователя %s\n", o.ID, o.RecipientID)
	}
	return nil
}
