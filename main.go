package main

import (
	"fmt"
	"log"
	"project/internal/app"
	"project/internal/config"
	"project/internal/database"
	"project/internal/mailworker"

	"net/http"
)

func main() {
	database.InitDB()
	config.LoadConfig()

	// database.DB.Create(&models.EmployeeCard{Name: "Миша"})
	// database.DB.Create((&models.User{Login: "admin", Password: "admin", EmployeeCardID: uint(1)}))

	// err := database.DB.Model(&models.Order{})

	mailworker.StartEmailMonitoring(database.DB)
	log.Println("Фоновый мониторинг входящей почты успешно запущен.")

	handler := app.NewRouter()
	fmt.Println("Сервер запущен!")
	err := http.ListenAndServe(":8080", handler)
	if err != nil {
		fmt.Println("Ошибка запуска сервера! ", err)
	}
}

// // 3. Create (Создание записи)
// database.DB.Create(&models.EmployeeCard{Name: "Миша"})
// database.DB.Create((&models.User{Login: "admin", Password: "admin", EmployeeCardID: uint(1)}))
// db.Create(&Product{Code: "D42", Price: 100})

// // 4. Read (Чтение записи)
// var product Product
// // Поиск первого продукта с кодом D42
// db.First(&product, "code = ?", "D42")
// fmt.Printf("Найден продукт: %s, Цена: %d\n", product.Code, product.Price)

// // 5. Update (Обновление)
// // Обновляем цену до 200
// db.Model(&product).Update("Price", 200)
// fmt.Println("Цена обновлена")

// // 6. Delete (Удаление)
// // GORM использует Soft Delete, если в структуре есть gorm.Model
// // db.Delete(&product, product.ID)
