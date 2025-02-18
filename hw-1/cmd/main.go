package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"hw-1/internal/storage"
)

func main() {
	// Тут я создаю хранилище если все плохо вывожу ошибочку
	st, err := storage.NewOrderStorage("orders.json")
	if err != nil {
		fmt.Printf("Ошибка при создании хранилища: %v\n", err)
		return
	}

	reader := bufio.NewReader(os.Stdin)

	for {
		// добавил > чтобы было понятнее как в консольке линуха
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
		// тут все просто нашли exit вышли из приложения
		if line == "exit" {
			fmt.Println("Выход из приложения.")
			return
		}

		parts := strings.Fields(line)
		cmd := parts[0]
		// если все-таки не выход парсим то, что получили
		switch cmd {
		// help выводим помощь
		case "help":
			printHelp()
		case "accept":
			// если не 4 слова, то отказываем в принятии заказа
			if len(parts) != 4 {
				fmt.Println("Формат: accept <orderID> <userID> <deadline in RFC3339>")
				continue
			}
			orderID := parts[1]
			userID := parts[2]
			deadlineStr := parts[3]
			// проверяем формат даты
			deadline, err := time.Parse(time.RFC3339, deadlineStr)
			if err != nil {
				fmt.Printf("Ошибка разбора даты: %v\n", err)
				continue
			}

			err = st.AcceptOrderFromCourier(orderID, userID, deadline)
			if err != nil {
				fmt.Printf("Ошибка accept: %v\n", err)
			} else {
				fmt.Printf("Заказ %s принят для пользователя %s (deadline=%s)\n", orderID, userID, deadline)
			}
		// возврат курьеру
		case "return_courier":
			if len(parts) != 2 {
				fmt.Println("Формат: return_courier <orderID>")
				continue
			}
			orderID := parts[1]
			// внутренняя логика опишу ее внутри метода
			err := st.ReturnOrderToCourier(orderID)
			if err != nil {
				fmt.Printf("Ошибка return_courier: %v\n", err)
			} else {
				fmt.Printf("Заказ %s возвращён курьеру и удалён\n", orderID)
			}
		// проверка на доставку
		case "deliver":
			if len(parts) < 3 {
				fmt.Println("Формат: deliver <userID> <orderID> [...дополнительные ID]")
				continue
			}
			userID := parts[1]
			orderIDs := parts[2:]
			// также внутренняя логика
			err := st.DeliverOrReturnClientOrders(userID, orderIDs, "deliver")
			if err != nil {
				fmt.Printf("Ошибка deliver: %v\n", err)
			} else {
				fmt.Printf("Выдано %d заказ(ов) пользователю %s: %v\n", len(orderIDs), userID, orderIDs)
			}
		// возврат заказа
		case "clientreturn":
			if len(parts) < 3 {
				fmt.Println("Формат: clientreturn <userID> <orderID> [...дополнительные ID]")
				continue
			}
			userID := parts[1]
			orderIDs := parts[2:]
			err := st.DeliverOrReturnClientOrders(userID, orderIDs, "return")
			if err != nil {
				fmt.Printf("Ошибка clientreturn: %v\n", err)
			} else {
				fmt.Printf("Принят возврат %d заказ(ов) от %s: %v\n", len(orderIDs), userID, orderIDs)
			}

		case "list":
			if len(parts) < 2 {
				fmt.Println("Формат: list <userID> [<lastN=0>] [<onlyInPVZ=false>]")
				continue
			}
			userID := parts[1]
			var lastN int
			// устал от обычного нейминга если надо переименую
			var onlyInPVZ bool

			if len(parts) >= 3 {
				ln, err := strconv.Atoi(parts[2])
				if err == nil {
					lastN = ln
				}
			}
			if len(parts) >= 4 {
				if parts[3] == "true" {
					onlyInPVZ = true
				}
			}

			orders, err := st.GetOrders(userID, lastN, onlyInPVZ)
			if err != nil {
				fmt.Printf("Ошибка GetOrders: %v\n", err)
				continue
			}
			if len(orders) == 0 {
				fmt.Printf("У пользователя %s нет заказов (с учётом фильтров)\n", userID)
				continue
			}
			fmt.Printf("Список заказов пользователя %s:\n", userID)
			for _, o := range orders {
				fmt.Printf("  ID=%s, State=%s, Deadline=%s\n",
					o.ID, o.State, o.StorageDeadline.Format(time.RFC3339))
			}

		case "returns":
			// устанавливаем размер страниц
			pageI := 1
			pageS := 10
			if len(parts) >= 2 {
				// смотрим на возвраты
				if tmp, err := strconv.Atoi(parts[1]); err == nil {
					pageI = tmp
				}
			}
			if len(parts) >= 3 {
				if tmp, err := strconv.Atoi(parts[2]); err == nil {
					pageS = tmp
				}
			}
			// вызываем метод для правильного вывода
			ret, err := st.GetReturns(pageI, pageS)
			if err != nil {
				fmt.Printf("Ошибка GetReturns: %v\n", err)
				continue
			}
			if len(ret) == 0 {
				fmt.Println("На данной странице возвратов нет.")
				continue
			}
			fmt.Printf("Страница %d, возвраты:\n", pageI)
			for _, r := range ret {
				fmt.Printf("  ID=%s, State=%s\n", r.ID, r.State)
			}

		case "history":
			hist, err := st.GetOrderHistory()
			if err != nil {
				fmt.Printf("Ошибка GetOrderHistory: %v\n", err)
				continue
			}
			if len(hist) == 0 {
				fmt.Println("История заказов пуста.")
				continue
			}
			fmt.Println("История заказов (последние изменения — первыми):")
			for _, h := range hist {
				fmt.Printf("  ID=%s, State=%s, LastChange=%s\n",
					h.ID, h.State, h.LastStateChange.Format(time.RFC3339))
			}

		default:
			fmt.Println("Неизвестная команда. Введите 'help' для справки.")
		}
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
