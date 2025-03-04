package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gitlab.ozon.dev/qwestard/homework/internal/packaging"
	"gitlab.ozon.dev/qwestard/homework/internal/storage"
)

type Command struct {
	Name        string
	Description string
	Handler     func([]string) error
}

type Handler struct {
	st       *storage.OrderStorage
	ps       packaging.PackagingService
	commands []Command
}

var ErrExit = errors.New("exit")

func New(st *storage.OrderStorage, ps packaging.PackagingService) (*Handler, error) {
	h := &Handler{
		st: st,
		ps: ps,
	}
	h.initCommands()
	return h, nil
}

func (h *Handler) initCommands() {
	h.commands = []Command{
		{"help", "выводит справку", h.printHelp},
		{"exit", "завершает программу", h.handleExit},
		{"accept", "принимает заказ от курьера. Формат: accept <orderID> <userID> <deadline RFC3339> <weight> <baseCost> [<packaging>]", h.handleAccept},
		{"return_courier", "возвращает заказ курьеру", h.handleReturnCourier},
		{"deliver", "выдает заказы клиенту", h.handleDeliver},
		{"clientreturn", "принимает возврат от клиента", h.handleClientReturn},
		{"list", "выводит список заказов пользователя", h.handleList},
		{"returns", "выводит список возвратов с пагинацией", h.handleReturns},
		{"history", "выводит историю заказов", h.handleHistory},
		{"accept_from_courier", "принимает заказы из файла JSON", h.acceptOrdersFromCourier},
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
	fmt.Println("Доступные команды:")
	for _, c := range h.commands {
		fmt.Printf("  %s: %s\n", c.Name, c.Description)
	}
	return nil
}

func (h *Handler) handleExit([]string) error {
	return ErrExit
}

func (h *Handler) parseAcceptOrderRequest(args []string) (storage.AcceptOrderFromCourierRequest, error) {
	var req storage.AcceptOrderFromCourierRequest
	if len(args) != 5 && len(args) != 6 {
		return req, errors.New("формат: accept <orderID> <userID> <deadline RFC3339> <weight> <baseCost> [<packaging>]")
	}
	req.OrderID = args[0]
	req.RecipientID = args[1]
	deadline, err := time.Parse(time.RFC3339, args[2])
	if err != nil {
		return req, fmt.Errorf("ошибка разбора даты: %w", err)
	}
	req.Deadline = deadline
	req.Weight, err = strconv.ParseFloat(args[3], 64)
	if err != nil {
		return req, fmt.Errorf("ошибка разбора веса: %w", err)
	}
	req.BaseCost, err = strconv.ParseFloat(args[4], 64)
	if err != nil {
		return req, fmt.Errorf("ошибка разбора базовой стоимости: %w", err)
	}
	if len(args) == 6 {
		raw := strings.Split(args[5], "+")
		for _, s := range raw {
			pt := packaging.PackagingType(strings.ToLower(strings.TrimSpace(s)))
			req.Packaging = append(req.Packaging, pt)
		}
	}
	return req, nil
}

func (h *Handler) validatePackaging(req storage.AcceptOrderFromCourierRequest) error {
	if len(req.Packaging) == 0 {
		return nil
	}
	var mainCount, filmCount int
	for _, pt := range req.Packaging {
		pkg, err := h.ps.GetPackaging(pt)
		if err != nil {
			return err
		}
		if err := pkg.Validate(req.Weight); err != nil {
			return err
		}
		if pt == packaging.PackagingFilm {
			filmCount++
		} else {
			mainCount++
		}
	}
	if mainCount > 1 {
		return errors.New("недопустимо использовать более одной основной упаковки (не film)")
	}
	if mainCount == 1 && filmCount > 1 {
		return errors.New("к основной упаковке можно добавить не более одной пленки")
	}
	return nil
}

func (h *Handler) handleAccept(args []string) error {
	req, err := h.parseAcceptOrderRequest(args)
	if err != nil {
		return err
	}
	if err := h.validatePackaging(req); err != nil {
		return fmt.Errorf("ошибка проверки упаковки: %w", err)
	}
	err = h.st.AcceptOrderFromCourier(req)
	if err != nil {
		return err
	}
	if len(req.Packaging) > 0 {
		fmt.Printf("Заказ %s принят для пользователя %s (deadline=%s, упаковка=%v, вес=%.2f, базовая стоимость=%.2f)\n",
			req.OrderID, req.RecipientID, req.Deadline, req.Packaging, req.Weight, req.BaseCost)
	} else {
		fmt.Printf("Заказ %s принят для пользователя %s (deadline=%s)\n",
			req.OrderID, req.RecipientID, req.Deadline)
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
