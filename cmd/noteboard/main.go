// Команда noteboard: точка входа. Здесь только чтение конфига и запуск —
// вся сборка приложения живёт в internal/app.
package main

import (
	"log"

	"noteboard/internal/app"
	"noteboard/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if err := app.Run(cfg); err != nil {
		log.Fatalf("run: %v", err)
	}
}
