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
		Logger: logger.Default.LogMode(logger.Warn),
	})

	if err != nil {
		panic("не удалось подключиться к базе данных")
	}

	DB.AutoMigrate(
		&models.Agreement{},
		&models.EmployeeCard{},
		&models.EmployeeBalance{},
		&models.EmployeeDocuments{},
		&models.CategoryEmployeeDocument{},
		&models.User{},
		&models.Materials{},
		&models.Suplers{},
		&models.ManagerCard{},
		&models.Estimate{},
		&models.Order{},
		&models.OrderMaterial{},
		&models.Email{},
		&models.Dialog{},
		&models.Attachment{},
		&models.Expenses{},
		&models.Tool{},
		&models.TimeSheet{},
		&models.TimeSheetMonth{},
		&models.Payment{},
		&models.Task{},
	)
}
