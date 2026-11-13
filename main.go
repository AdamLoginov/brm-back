package main

import (
	"fmt"
	"log"
	"os"
	"project/internal/app"
	"project/internal/config"
	"project/internal/database"
	"project/internal/mailworker"
	"project/internal/service"

	"net/http"
)

func main() {
	config.LoadConfig()
	database.InitDB()
	if err := service.MigratePasswords(); err != nil {
		log.Fatal(err)
	}

	service.CreateBalanceForUser()

	if os.Getenv("MAIL_ENABLED") == "true" {
		mailworker.StartEmailMonitoring(database.DB)
	}

	handler := app.NewRouter()
	fmt.Println("Сервер запущен!")
	address := os.Getenv("HTTP_ADDR")
	if address == "" {
		address = "127.0.0.1:8080"
	}
	err := http.ListenAndServe(address, handler)
	if err != nil {
		fmt.Println("Ошибка запуска сервера! ", err)
	}
}
