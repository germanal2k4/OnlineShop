package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"gitlab.ozon.dev/qwestard/homework/internal/handler"
	"gitlab.ozon.dev/qwestard/homework/internal/packaging"
	"gitlab.ozon.dev/qwestard/homework/internal/storage"
)

const storageFile = "orders.json"

func main() {
	ps := packaging.NewPackagingService()

	st, err := storage.New(storageFile)
	if err != nil {
		fmt.Printf("Ошибка при создании хранилища: %v\n", err)
		os.Exit(1)
	}

	h, err := handler.New(st, ps)

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("\n> ")
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Ошибка чтения: %v\n", err)
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		cmd := parts[0]
		args := parts[1:]
		err = h.Execute(cmd, args)
		if err != nil {
			if errors.Is(err, handler.ErrExit) {
				fmt.Println("Выход из приложения.")
				break
			}
			fmt.Printf("Ошибка выполнения команды: %v\n", err)
		}
	}
}
