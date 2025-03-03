package main

import (
	"bufio"
	"errors"
	"fmt"
	"gitlab.ozon.dev/qwestard/homework/internal/packaging"
	"os"
	"strings"

	"gitlab.ozon.dev/qwestard/homework/internal/handler"
	"gitlab.ozon.dev/qwestard/homework/internal/storage"
)

const storageFile = "orders.json"

func main() {
	st, err := storage.New(storageFile, packaging.NewPackagingService())
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

		parts := strings.Fields(line)

		if len(parts) < 1 {
			fmt.Printf("Неправильный формат ввода")
			return
		}
		cmd := parts[0]
		args := parts[1:]
		h := handler.New(st)

		err = h.Execute(cmd, args)
		if err != nil {
			if errors.Is(err, handler.ErrExit) {
				fmt.Println("Выход из приложения.")
				break
			}
			fmt.Printf("Неправильный формат ввода: %v\n", err)
		}
	}
}
