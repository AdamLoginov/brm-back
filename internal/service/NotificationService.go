// --------- новый код chat: задача и уведомления меняются одной транзакцией
package service

import (
	"errors"
	"gorm.io/gorm"
	"project/internal/models"
	"sync"
	"time"
)

var taskNotificationMutex sync.Mutex
var ErrTaskUser = errors.New("Пользователь задачи не найден")

func notificationActor(tx *gorm.DB, id uint) (models.User, error) {
	var u models.User
	if id == 0 {
		return u, ErrTaskUser
	}
	if err := tx.Select("id", "login").First(&u, id).Error; err != nil {
		return u, ErrTaskUser
	}
	return u, nil
}
func taskRecipient(tx *gorm.DB, id uint) error {
	if id == 0 {
		return nil
	}
	_, err := notificationActor(tx, id)
	return err
}
func notifyTask(tx *gorm.DB, t models.Task, actor models.User, recipient uint, kind string) error {
	if recipient == 0 {
		return nil
	}
	return tx.Create(&models.Notification{RecipientID: recipient, TaskID: t.ID, ActorID: actor.ID, ActorName: actor.Login, Kind: kind, TaskMessage: t.Message}).Error
}
func CreateTaskWithNotifications(db *gorm.DB, actorID uint, input models.Task) (models.Task, error) {
	taskNotificationMutex.Lock()
	defer taskNotificationMutex.Unlock()
	task := models.Task{Message: input.Message, ToUserID: input.ToUserID, FromUserID: actorID, Priority: input.Priority, Status: false}
	err := db.Transaction(func(tx *gorm.DB) error {
		actor, err := notificationActor(tx, actorID)
		if err != nil {
			return err
		}
		if err = taskRecipient(tx, task.ToUserID); err != nil {
			return err
		}
		if err = tx.Create(&task).Error; err != nil {
			return err
		}
		return notifyTask(tx, task, actor, task.ToUserID, "assigned")
	})
	return task, err
}
func UpdateTaskWithNotifications(db *gorm.DB, actorID uint, input models.Task, completeOnly bool) (models.Task, error) {
	taskNotificationMutex.Lock()
	defer taskNotificationMutex.Unlock()
	var task models.Task
	err := db.Transaction(func(tx *gorm.DB) error {
		actor, err := notificationActor(tx, actorID)
		if err != nil {
			return err
		}
		if input.ID == 0 {
			return gorm.ErrRecordNotFound
		}
		if err = tx.First(&task, input.ID).Error; err != nil {
			return err
		}
		previousStatus, previousRecipient := task.Status, task.ToUserID
		if completeOnly {
			task.Status = true
		} else {
			task.Message = input.Message
			task.Status = input.Status
			task.Priority = input.Priority
			task.ToUserID = input.ToUserID
		}
		if err = taskRecipient(tx, task.ToUserID); err != nil {
			return err
		}
		if err = tx.Model(&task).Select("Message", "Status", "Priority", "ToUserID").Updates(&task).Error; err != nil {
			return err
		}
		// Возврат с переназначением даёт новому исполнителю одно уведомление о возврате.
		if previousStatus && !task.Status {
			if err = notifyTask(tx, task, actor, task.ToUserID, "reopened"); err != nil {
				return err
			}
		} else if previousRecipient != task.ToUserID {
			if err = notifyTask(tx, task, actor, task.ToUserID, "assigned"); err != nil {
				return err
			}
		}
		if !previousStatus && task.Status {
			return notifyTask(tx, task, actor, task.FromUserID, "completed")
		}
		return nil
	})
	return task, err
}
func DeleteTaskWithNotifications(db *gorm.DB, actorID, id uint) error {
	taskNotificationMutex.Lock()
	defer taskNotificationMutex.Unlock()
	return db.Transaction(func(tx *gorm.DB) error {
		if _, err := notificationActor(tx, actorID); err != nil {
			return err
		}
		var task models.Task
		if id == 0 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.First(&task, id).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("task_id = ?", id).Delete(&models.Notification{}).Error; err != nil {
			return err
		}
		return tx.Delete(&task).Error
	})
}

type NotificationList struct {
	Items    []models.Notification `json:"items"`
	Unread   int64                 `json:"unread"`
	Total    int64                 `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
}

func ListNotifications(db *gorm.DB, userID uint, page int) (NotificationList, error) {
	result := NotificationList{Items: []models.Notification{}, Page: page, PageSize: 50}
	err := db.Transaction(func(tx *gorm.DB) error {
		if _, err := notificationActor(tx, userID); err != nil {
			return err
		}
		query := func() *gorm.DB { return tx.Model(&models.Notification{}).Where("recipient_id = ?", userID) }
		if err := query().Count(&result.Total).Error; err != nil {
			return err
		}
		if err := query().Where("read_at IS NULL").Count(&result.Unread).Error; err != nil {
			return err
		}
		last := int((result.Total + 49) / 50)
		if last < 1 {
			last = 1
		}
		if result.Page > last {
			result.Page = last
		}
		if result.Page < 1 {
			result.Page = 1
		}
		return query().Order("id DESC").Offset((result.Page - 1) * 50).Limit(50).Find(&result.Items).Error
	})
	return result, err
}
func ReadNotifications(db *gorm.DB, userID, id uint, all bool) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if _, err := notificationActor(tx, userID); err != nil {
			return err
		}
		query := tx.Model(&models.Notification{}).Where("recipient_id = ?", userID)
		if !all {
			var n models.Notification
			if id == 0 {
				return gorm.ErrRecordNotFound
			}
			if err := query.Where("id = ?", id).First(&n).Error; err != nil {
				return err
			}
			query = tx.Model(&models.Notification{}).Where("recipient_id = ? AND id = ?", userID, id)
		}
		return query.Where("read_at IS NULL").Update("read_at", time.Now()).Error
	})
}

// --------- конец нового кода chat
