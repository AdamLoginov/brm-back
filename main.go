package main

import (
	"fmt"
	"log"
	"project/internal/app"
	"project/internal/config"
	"project/internal/database"
	"project/internal/mailworker"
	"project/internal/service"

	"net/http"
)

func main() {
	database.InitDB()
	config.LoadConfig()

	service.CreateBalanceForUser()

	mailworker.StartEmailMonitoring(database.DB)
	log.Println("Фоновый мониторинг входящей почты успешно запущен.")

	handler := app.NewRouter()
	fmt.Println("Сервер запущен!")
	err := http.ListenAndServe(":8080", handler)
	if err != nil {
		fmt.Println("Ошибка запуска сервера! ", err)
	}
}
