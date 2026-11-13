package service

import (
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"project/internal/database"
	"project/internal/models"
	"testing"
)

func TestPasswordMigrationPreservesCredentialsAndIsIdempotent(t *testing.T) {
	old := database.DB
	t.Cleanup(func() { database.DB = old })
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	database.DB = db
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatal(err)
	}
	user := models.User{Login: "migration-test", Password: "existing-password"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	if err := MigratePasswords(); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&user, user.ID).Error; err != nil {
		t.Fatal(err)
	}
	firstHash := user.Password
	if err := bcrypt.CompareHashAndPassword([]byte(firstHash), []byte("existing-password")); err != nil {
		t.Fatal(err)
	}
	if err := MigratePasswords(); err != nil {
		t.Fatal(err)
	}
	if err := db.First(&user, user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if user.Password != firstHash {
		t.Fatal("password was hashed twice")
	}
}
