package main

import (
	"log"

	"github.com/director74/dz8_shop/order-service/config"
	"github.com/director74/dz8_shop/order-service/internal/app"
)

func main() {
	// Загружаем конфигурацию
	cfg, err := config.NewConfig()
	if err != nil {
		log.Fatalf("Ошибка при загрузке конфигурации: %v", err)
	}

	orderApp, err := app.NewApp(cfg)
	if err != nil {
		log.Fatalf("Ошибка при создании приложения: %v", err)
	}

	// Запускаем приложение
	if err := orderApp.Run(); err != nil {
		log.Fatalf("Ошибка при запуске приложения: %v", err)
	}
}
