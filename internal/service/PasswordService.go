package service

import (
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"project/internal/database"
	"project/internal/models"
)

// MigratePasswords preserves existing credentials while removing plaintext storage.
func MigratePasswords() error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		var users []models.User
		if err := tx.Unscoped().Find(&users).Error; err != nil {
			return err
		}
		for _, user := range users {
			if _, err := bcrypt.Cost([]byte(user.Password)); err == nil {
				continue
			}
			hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
			if err != nil {
				return err
			}
			if err := tx.Unscoped().Model(&models.User{}).Where("id = ?", user.ID).Update("password", string(hash)).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
