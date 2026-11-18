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
		&models.StoredFile{},                           // новый код chat: реестр файлов
		&models.Notification{},                         // новый код chat: уведомления задач
		&models.SalaryApprovalBatch{},                  // новый код chat: идемпотентное применение выбора
		&models.SalaryPaymentDraft{},                   // --------- новый код chat: индивидуальные выплаты и согласования
		&models.SalaryDebt{}, &models.SalaryApproval{}, // новый код chat: долги и согласования
		&models.SalaryBonus{}, // новый код chat: премии за месяц
		&models.Agreement{},
		&models.EmployeeCard{},
		&models.EmployeeSpecialization{}, // новый код chat: справочник специализаций
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
		&models.SalaryAccrual{},         // новый код chat: таблица начислений
		&models.SalaryAdvance{},         // новый код chat: таблица авансов
		&models.SalaryPenalty{},         // новый код chat: таблица штрафов
		&models.DailyAllowanceAccrual{}, // новый код chat: начисления суточных
		&models.DailyAllowancePayment{}, // новый код chat: история выдач суточных
		&models.Payment{},
		&models.Task{},
	)
}
