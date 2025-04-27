package main

import (
	"log"

	"github.com/director74/dz8_shop/payment-service/config"
	"github.com/director74/dz8_shop/payment-service/internal/app"
)

func main() {
	// Загрузка конфигурации
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatalf("Ошибка загрузки конфигурации: %v", err)
	}

	// Создание приложения
	application, err := app.NewApp(cfg)
	if err != nil {
		log.Fatalf("Ошибка создания приложения: %v", err)
	}

	// Запуск приложения
	if err := application.Run(); err != nil {
		log.Fatalf("Ошибка запуска приложения: %v", err)
	}
}
