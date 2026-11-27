package service

// --------- новый код chat: уведомления и откат изменения вложений
import (
	"errors"
	"testing"

	"gorm.io/gorm"
	"project/internal/models"
)

func TestTaskAttachmentEditNotificationAndRollback(t *testing.T) {
	db, users := notificationFixture(t)
	if err := db.AutoMigrate(&models.TaskAttachment{}); err != nil {
		t.Fatal(err)
	}
	task, err := CreateTaskWithNotifications(db, users[0].ID, models.Task{Message: "Проверка", ToUserID: users[1].ID})
	if err != nil {
		t.Fatal(err)
	}
	appendFile := func(tx *gorm.DB, id uint) error {
		return tx.Create(&models.TaskAttachment{TaskID: id, Name: "Отчёт.pdf"}).Error
	}
	if _, err = UpdateTaskWithNotifications(db, users[0].ID, task, false, appendFile); err != nil {
		t.Fatal(err)
	}
	task.Message = "Новый текст"
	fail := func(*gorm.DB, uint) error { return errors.New("ошибка файла") }
	if _, err = UpdateTaskWithNotifications(db, users[0].ID, task, false, appendFile, fail); err == nil {
		t.Fatal("ожидалась ошибка")
	}
	var saved models.Task
	if err = db.Preload("Attachments").First(&saved, task.ID).Error; err != nil || len(saved.Attachments) != 1 || saved.Message != "Проверка" {
		t.Fatalf("неверное состояние задачи после отката: %+v, %v", saved, err)
	}
	var count int64
	if err = db.Model(&models.Notification{}).Where("recipient_id = ? AND kind = ?", users[1].ID, "updated").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("уведомлений об изменении: %d, %v", count, err)
	}
}
