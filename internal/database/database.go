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
		&models.MeasurementUnit{}, &models.AgreementWorkshop{}, &models.AgreementWorkBlock{}, &models.AgreementWorkPosition{}, // новый код chat
		&models.WorkOrderBook{}, &models.WorkOrderDay{}, &models.WorkOrderShift{}, &models.WorkOrderEmployee{}, &models.WorkOrderTask{}, // новый код chat
		&models.WarehouseMaterial{}, &models.WarehouseReceipt{}, &models.WorkOrderMaterial{}, // новый код chat
		&models.StoredFile{},                           // новый код chat: реестр файлов
		&models.Notification{},                         // новый код chat: уведомления задач
		&models.SalaryApprovalBatch{},                  // новый код chat: идемпотентное применение выбора
		&models.SalaryPaymentDraft{},                   // --------- новый код chat: индивидуальные выплаты и согласования
		&models.SalaryDebt{}, &models.SalaryApproval{}, // новый код chat: долги и согласования
		&models.SalaryBonus{}, // новый код chat: премии за месяц
		&models.Agreement{},
		&models.AgreementContact{}, // новый код chat: контакты контрагента
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
		&models.ExpenseCategory{}, // новый код chat: общие категории расходов
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
	// --------- новый код chat: перенос назначений после обновления схемы
	if err := migrateTaskAssignees(DB); err != nil {
		panic(err)
	}
}

// --------- новый код chat: повторный запуск не возвращает снятых участников
func migrateTaskAssignees(db *gorm.DB) error {
	return db.Exec(`INSERT INTO task_assignees (task_id, user_id)
 SELECT tasks.id, tasks.to_user_id FROM tasks
 WHERE tasks.to_user_id > 0
 AND EXISTS (SELECT 1 FROM users WHERE users.id = tasks.to_user_id)
 AND NOT EXISTS (SELECT 1 FROM task_assignees WHERE task_assignees.task_id = tasks.id)`).Error
}

// --------- конец нового кода chat
