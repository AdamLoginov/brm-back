package database

import (
	"project/internal/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDB() {
	var err error

	DB, err = gorm.Open(sqlite.Open("test.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		panic("не удалось подключиться к базе данных")
	}

	DB.AutoMigrate(
		&models.EmployeeCard{},
		&models.User{},
		&models.Materials{},
		&models.Suplers{},
		&models.ManagerCard{},
		&models.Agreement{},
		&models.Estimate{},
		&models.Order{},
		&models.OrderMaterial{},
		&models.Email{},
		&models.Dialog{},
		&models.Attachment{},
		&models.Expenses{},
		&models.Tool{},
		&models.CategoryEmployeeDocument{},
		&models.EmployeeDocuments{},
	)
}
