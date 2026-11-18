// --------- новый код chat: перенос старых задач без создания уведомлений
package database

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"project/internal/models"
	"testing"
)

func TestTaskAssigneesMigration(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "migration.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	handle, _ := db.DB()
	defer handle.Close()
	if err = db.AutoMigrate(&models.User{}, &models.Task{}, &models.Notification{}); err != nil {
		t.Fatal(err)
	}
	user := models.User{Login: "legacy"}
	db.Create(&user)
	task := models.Task{Message: "Старая задача", ToUserID: user.ID, Status: true}
	db.Create(&task)
	for i := 0; i < 2; i++ {
		if err = migrateTaskAssignees(db); err != nil {
			t.Fatal(err)
		}
	}
	var got models.Task
	db.Preload("Assignees").First(&got, task.ID)
	if len(got.Assignees) != 1 || got.Assignees[0].ID != user.ID || got.CompletedByID != nil {
		t.Fatal("incorrect migration", got)
	}
	var count int64
	db.Model(&models.Notification{}).Count(&count)
	if count != 0 {
		t.Fatal("migration notified users")
	}
	db.Model(&task).Association("Assignees").Clear()
	db.Model(&task).Update("to_user_id", 0)
	if err = migrateTaskAssignees(db); err != nil {
		t.Fatal(err)
	}
	if db.Model(&task).Association("Assignees").Count() != 0 {
		t.Fatal("removed participant restored")
	}
}

// --------- конец нового кода chat
