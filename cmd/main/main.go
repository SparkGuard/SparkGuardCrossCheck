package main

import (
	"CodeBorrowing/internal/app"
	"CodeBorrowing/internal/config"
	"fmt"
)

func main() {
	// Чтение конфигураций.
	cfg, err := config.GetConfig()
	if err != nil {
		fmt.Println(err)
		return
	}

	// Инициализация приложения.
	application, err := app.Init(cfg)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer application.Close()

	//Запуск приложения.
	application.Run()
}
