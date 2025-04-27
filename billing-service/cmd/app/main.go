package main

import (
	"log"

	"github.com/director74/dz8_shop/billing-service/config"
	"github.com/director74/dz8_shop/billing-service/internal/app"
)

func main() {
	// Загружаем конфигурацию
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatalf("Ошибка при загрузке конфигурации: %v", err)
	}

	billingApp, err := app.NewApp(cfg)
	if err != nil {
		log.Fatalf("Ошибка при создании приложения: %v", err)
	}

	// Запускаем приложение
	if err := billingApp.Run(); err != nil {
		log.Fatalf("Ошибка при запуске приложения: %v", err)
	}
}
