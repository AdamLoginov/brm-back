// --------- новый код chat: разовый переход к общему списку алиментов сотрудника
package database

import (
	"gorm.io/gorm"
	"project/internal/models"
)

func migrateEmployeeAlimony(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if tx.Migrator().HasTable("salary_alimonies") && tx.Migrator().HasColumn("salary_alimonies", "time_sheet_month_id") {
			if err := tx.Migrator().DropTable("salary_alimonies"); err != nil {
				return err
			}
		}
		return tx.AutoMigrate(&models.SalaryAlimony{}, &models.SalaryAlimonyEmployeeHistory{}) // новый код chat: доступ к истории после перевода
	})
}

// --------- конец нового кода chat
