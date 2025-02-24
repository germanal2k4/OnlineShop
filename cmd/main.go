package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gitlab.ozon.dev/qwestard/homework/internal/storage"
)

func main() {
	const storageFile = "orders.json"
	st, err := storage.New(storageFile)
	if err != nil {
		fmt.Printf("Ошибка при создании хранилища: %v\n", err)
		return
	}

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("\n> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Ошибка чтения: %v\n", err)
			return
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		cmd := parts[0]
		args := parts[1:]

		switch cmd {
		case "help":
			handleHelp()
		case "exit":
			handleExit()
		case "accept":
			handleAccept(args, st)
		case "return_courier":
			handleReturnCourier(args, st)
		case "deliver":
			handleDeliver(args, st)
		case "clientreturn":
			handleClientReturn(args, st)
		case "list":
			handleList(args, st)
		case "returns":
			handleReturns(args, st)
		case "history":
			handleHistory(st)
		default:
			fmt.Println("Неизвестная команда. Введите 'help' для справки.")
		}
	}
}

func handleHelp() {
	printHelp()
}

func handleExit() {
	fmt.Println("Выход из приложения.")
	os.Exit(0)
}

func handleAccept(args []string, st *storage.OrderStorage) {
	if len(args) != 3 {
		fmt.Println("Формат: accept <orderID> <userID> <deadline in RFC3339>")
		return
	}
	orderID := args[0]
	userID := args[1]
	deadlineStr := args[2]
	deadline, err := time.Parse(time.RFC3339, deadlineStr)
	if err != nil {
		fmt.Printf("Ошибка разбора даты: %v\n", err)
		return
	}

	if err := st.AcceptOrderFromCourier(orderID, userID, deadline); err != nil {
		fmt.Printf("Ошибка accept: %v\n", err)
		return
	}
	fmt.Printf("Заказ %s принят для пользователя %s (deadline=%s)\n", orderID, userID, deadline)
}

func handleReturnCourier(args []string, st *storage.OrderStorage) {
	if len(args) != 1 {
		fmt.Println("Формат: return_courier <orderID>")
		return
	}
	orderID := args[0]
	if err := st.ReturnOrderToCourier(orderID); err != nil {
		fmt.Printf("Ошибка return_courier: %v\n", err)
		return
	}
	fmt.Printf("Заказ %s возвращён курьеру и обновлён/удалён\n", orderID)

}

func handleDeliver(args []string, st *storage.OrderStorage) {
	if len(args) < 2 {
		fmt.Println("Формат: deliver <userID> <orderID> [...дополнительные ID]")
		return
	}
	userID := args[0]
	orderIDs := args[1:]
	if err := st.DeliverOrReturnClientOrders(userID, orderIDs, "deliver"); err != nil {
		fmt.Printf("Ошибка deliver: %v\n", err)
		return
	}
	fmt.Printf("Выдано %d заказ(ов) пользователю %s: %v\n", len(orderIDs), userID, orderIDs)
}

func handleClientReturn(args []string, st *storage.OrderStorage) {
	if len(args) < 2 {
		fmt.Println("Формат: clientreturn <userID> <orderID> [...дополнительные ID]")
		return
	}
	userID := args[0]
	orderIDs := args[1:]
	if err := st.DeliverOrReturnClientOrders(userID, orderIDs, "return"); err != nil {
		fmt.Printf("Ошибка clientreturn: %v\n", err)
		return
	}
	fmt.Printf("Принят возврат %d заказ(ов) от %s: %v\n", len(orderIDs), userID, orderIDs)

}

func handleList(args []string, st *storage.OrderStorage) {
	if len(args) < 1 {
		fmt.Println("Формат: list <userID> [<lastN=0>] [<onlyInPVZ=false>]")
		return
	}
	userID := args[0]
	var lastN int
	var onlyInPVZ bool
	if len(args) >= 2 {
		if ln, err := strconv.Atoi(args[1]); err == nil {
			lastN = ln
		}
	}
	if len(args) >= 3 && args[2] == "true" {
		onlyInPVZ = true
	}
	orders, err := st.GetOrders(userID, lastN, onlyInPVZ)
	if err != nil {
		fmt.Printf("Ошибка GetOrders: %v\n", err)
		return
	}
	if len(orders) == 0 {
		fmt.Printf("У пользователя %s нет заказов (с учётом фильтров)\n", userID)
		return
	}
	fmt.Printf("Список заказов пользователя %s:\n", userID)
	for _, o := range orders {
		fmt.Printf("  ID=%s, State=%s, Deadline=%s\n", o.ID, o.CurrentState(), o.StorageDeadline.Format(time.RFC3339))
	}
}

func handleReturns(args []string, st *storage.OrderStorage) {
	pageI, pageS := 1, 10
	if len(args) >= 1 {
		if tmp, err := strconv.Atoi(args[0]); err == nil {
			pageI = tmp
		}
	}
	if len(args) >= 2 {
		if tmp, err := strconv.Atoi(args[1]); err == nil {
			pageS = tmp
		}
	}
	ret, err := st.GetReturns(pageI, pageS)
	if err != nil {
		fmt.Printf("Ошибка GetReturns: %v\n", err)
		return
	}
	if len(ret) == 0 {
		fmt.Println("На данной странице возвратов нет.")
		return
	}
	fmt.Printf("Страница %d, возвраты:\n", pageI)
	for _, r := range ret {
		fmt.Printf("  ID=%s, State=%s\n", r.ID, r.CurrentState())
	}
}

func handleHistory(st *storage.OrderStorage) {
	hist, err := st.GetOrderHistory()
	if err != nil {
		fmt.Printf("Ошибка GetOrderHistory: %v\n", err)
		return
	}
	if len(hist) == 0 {
		fmt.Println("История заказов пуста.")
		return
	}
	fmt.Println("История заказов (последние изменения — первыми):")
	for _, h := range hist {
		fmt.Printf("  ID=%s, State=%s, LastChange=%s\n", h.ID, h.CurrentState(), h.LastStateChange.Format(time.RFC3339))
	}
}

func printHelp() {
	fmt.Println(`Доступные команды:
  help
    - выводит эту справку

  exit
    - завершить программу

  accept <orderID> <userID> <deadline RFC3339>
    - принять заказ от курьера

  return_courier <orderID>
    - вернуть заказ курьеру, если срок хранения вышел и заказ ещё не выдавался

  deliver <userID> <orderID1> [orderID2 ...]
    - выдать заказы клиенту 

  clientreturn <userID> <orderID1> [orderID2 ...]
    - принять возврат от клиента

  list <userID> [lastN=0] [onlyInPVZ=false]
    - получить заказы пользователя, опционально: последние N и/или только в ПВЗ

  returns [pageIndex=1] [pageSize=10]
    - получить список возвратов постранично

  history
    - показать историю заказов
`)
}
