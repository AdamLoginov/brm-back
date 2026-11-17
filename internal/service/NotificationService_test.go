// --------- новый код chat: события задач, отсутствие дублей, транзакции и личное прочтение
package service

import (
	"errors"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"project/internal/models"
	"sync"
	"testing"
)

func notificationFixture(t *testing.T) (*gorm.DB, []models.User) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "notifications.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn, _ := db.DB(); conn.Close() })
	if err = db.AutoMigrate(&models.User{}, &models.Task{}, &models.Notification{}); err != nil {
		t.Fatal(err)
	}
	users := []models.User{{Login: "author"}, {Login: "executor"}, {Login: "other"}}
	if err = db.Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	return db, users
}
func TestTaskNotificationLifecycle(t *testing.T) {
	db, u := notificationFixture(t)
	task, err := CreateTaskWithNotifications(db, u[0].ID, models.Task{Message: "Проверить табель", ToUserID: u[1].ID, FromUserID: u[2].ID, Status: true})
	if err != nil {
		t.Fatal(err)
	}
	if task.FromUserID != u[0].ID || task.Status {
		t.Fatal("untrusted creator/status")
	}
	assertKinds := func(user uint, kinds ...string) {
		t.Helper()
		list, e := ListNotifications(db, user, 1)
		if e != nil {
			t.Fatal(e)
		}
		if len(list.Items) != len(kinds) {
			t.Fatalf("notifications for %d: %+v", user, list.Items)
		}
		for i, k := range kinds {
			if list.Items[i].Kind != k {
				t.Fatal("kind", list.Items[i])
			}
		}
	}
	assertKinds(u[1].ID, "assigned")
	assertKinds(u[0].ID)
	// Повторное сохранение текста не создаёт уведомления о назначении.
	task.Message = "Проверить табель и часы"
	task, err = UpdateTaskWithNotifications(db, u[0].ID, task, false)
	if err != nil {
		t.Fatal(err)
	}
	assertKinds(u[1].ID, "assigned")
	task.ToUserID = u[2].ID
	task, err = UpdateTaskWithNotifications(db, u[0].ID, task, false)
	if err != nil {
		t.Fatal(err)
	}
	assertKinds(u[2].ID, "assigned")
	// Завершить может любой пользователь, в уведомлении фиксируется фактический автор действия.
	task, err = UpdateTaskWithNotifications(db, u[1].ID, task, true)
	if err != nil {
		t.Fatal(err)
	}
	list, err := ListNotifications(db, u[0].ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if list.Items[0].ActorID != u[1].ID || list.Items[0].ActorName != "executor" || list.Items[0].TaskMessage != task.Message {
		t.Fatal("actor/snapshot")
	}
	_, err = UpdateTaskWithNotifications(db, u[2].ID, task, true)
	if err != nil {
		t.Fatal(err)
	}
	assertKinds(u[0].ID, "completed")
	task.Status = false
	task, err = UpdateTaskWithNotifications(db, u[0].ID, task, false)
	if err != nil {
		t.Fatal(err)
	}
	assertKinds(u[2].ID, "reopened", "assigned")
	task.Status = true
	task, err = UpdateTaskWithNotifications(db, u[2].ID, task, false)
	if err != nil {
		t.Fatal(err)
	}
	assertKinds(u[0].ID, "completed", "completed")
	// Получение ленты ничего не прочитывает.
	list, _ = ListNotifications(db, u[0].ID, 1)
	if list.Unread != 2 {
		t.Fatal("GET marked read")
	}
	if err = ReadNotifications(db, u[1].ID, list.Items[0].ID, false); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("read another user", err)
	}
	if err = ReadNotifications(db, u[0].ID, list.Items[0].ID, false); err != nil {
		t.Fatal(err)
	}
	list, _ = ListNotifications(db, u[0].ID, 1)
	if list.Unread != 1 || list.Items[0].ReadAt == nil {
		t.Fatal("individual read")
	}
	if err = ReadNotifications(db, u[0].ID, 0, true); err != nil {
		t.Fatal(err)
	}
	list, _ = ListNotifications(db, u[0].ID, 1)
	if list.Unread != 0 {
		t.Fatal("read all")
	}
	list, _ = ListNotifications(db, u[2].ID, 1)
	if list.Unread != 2 {
		t.Fatal("read affected other user")
	}
	if err = DeleteTaskWithNotifications(db, u[1].ID, task.ID); err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Unscoped().Model(&models.Notification{}).Where("task_id = ?", task.ID).Count(&count)
	if count != 0 {
		t.Fatal("deleted task notifications remain")
	}
}
func TestTaskSelfNotificationAndConcurrentCompletion(t *testing.T) {
	db, u := notificationFixture(t)
	task, err := CreateTaskWithNotifications(db, u[0].ID, models.Task{Message: "Самому себе", ToUserID: u[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := UpdateTaskWithNotifications(db, u[0].ID, task, true); errs <- e }()
	}
	wg.Wait()
	close(errs)
	for err = range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	list, err := ListNotifications(db, u[0].ID, 1)
	if err != nil || list.Total != 2 || list.Items[0].Kind != "completed" || list.Items[1].Kind != "assigned" {
		t.Fatal("self/duplicate", list, err)
	}
}
func TestTaskNotificationTransactionRollback(t *testing.T) {
	db, u := notificationFixture(t)
	failure := errors.New("test notification write failed")
	if err := db.Callback().Create().Before("gorm:create").Register("fail_notification", func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "notifications" {
			tx.AddError(failure)
		}
	}); err != nil {
		t.Fatal(err)
	}
	_, err := CreateTaskWithNotifications(db, u[0].ID, models.Task{ToUserID: u[1].ID})
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	var count int64
	db.Model(&models.Task{}).Count(&count)
	if count != 0 {
		t.Fatal("task created without notification")
	}
	db.Callback().Create().Remove("fail_notification")
	// Старые задачи не получают уведомления при чтении или обычном редактировании.
	task := models.Task{Message: "Старая", FromUserID: u[0].ID, ToUserID: u[1].ID}
	db.Create(&task)
	if _, err = UpdateTaskWithNotifications(db, u[0].ID, task, false); err != nil {
		t.Fatal(err)
	}
	db.Model(&models.Notification{}).Count(&count)
	if count != 0 {
		t.Fatal("backfill")
	}
}

// --------- конец нового кода chat
