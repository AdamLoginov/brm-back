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

// --------- новый код chat: несколько исполнителей, единое завершение и персональные уведомления
var ErrTaskForbidden = errors.New("Выполнить или вернуть задачу в работу может только автор или исполнитель")

func taskUsers(task models.Task) []uint {
	ids := []uint{}
	for _, user := range task.Assignees {
		ids = append(ids, user.ID)
	}
	if len(ids) == 0 && task.ToUserID != 0 {
		ids = append(ids, task.ToUserID)
	}
	return ids
}
func containsTaskUser(ids []uint, id uint) bool {
	for _, value := range ids {
		if value == id {
			return true
		}
	}
	return false
}
func resolveTaskUsers(tx *gorm.DB, input models.Task) ([]models.User, error) {
	ids := input.AssigneeIDs
	if ids == nil {
		ids = []uint{}
		if input.ToUserID != 0 {
			ids = append(ids, input.ToUserID)
		}
	}
	users := []models.User{}
	seen := map[uint]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		user, err := notificationActor(tx, id)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, nil
}
func saveTaskUsers(tx *gorm.DB, task *models.Task, users []models.User) error {
	if err := tx.Model(task).Association("Assignees").Replace(users); err != nil {
		return err
	}
	task.Assignees = users
	task.ToUserID = 0
	if len(users) > 0 {
		task.ToUserID = users[0].ID
	}
	return tx.Model(task).Update("to_user_id", task.ToUserID).Error
}
func CreateTaskWithNotifications(db *gorm.DB, actorID uint, input models.Task) (models.Task, error) {
	taskNotificationMutex.Lock()
	defer taskNotificationMutex.Unlock()
	task := models.Task{Message: input.Message, FromUserID: actorID, Priority: input.Priority}
	err := db.Transaction(func(tx *gorm.DB) error {
		actor, err := notificationActor(tx, actorID)
		if err != nil {
			return err
		}
		users, err := resolveTaskUsers(tx, input)
		if err != nil {
			return err
		}
		if err = tx.Omit("Assignees", "CompletedBy", "ToUser", "FromUser").Create(&task).Error; err != nil {
			return err
		}
		if err = saveTaskUsers(tx, &task, users); err != nil {
			return err
		}
		for _, user := range users {
			if err = notifyTask(tx, task, actor, user.ID, "assigned"); err != nil {
				return err
			}
		}
		return nil
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
		if err = tx.Preload("Assignees").First(&task, input.ID).Error; err != nil {
			return err
		}
		previousStatus := task.Status
		previousUsers := taskUsers(task)
		nextStatus := input.Status
		if completeOnly {
			nextStatus = true
		}
		// Проверка по прежнему составу: нельзя добавить себя и закрыть задачу одним запросом.
		if (completeOnly || previousStatus != nextStatus) && actorID != task.FromUserID && !containsTaskUser(previousUsers, actorID) {
			return ErrTaskForbidden
		}
		task.Status = nextStatus
		if !completeOnly {
			task.Message = input.Message
			task.Priority = input.Priority
			// Старый клиент может менять одиночное назначение; новые клиенты передают явный массив.
			if input.AssigneeIDs == nil && input.ToUserID == task.ToUserID {
				input.AssigneeIDs = previousUsers
			}
			users, err := resolveTaskUsers(tx, input)
			if err != nil {
				return err
			}
			if err = saveTaskUsers(tx, &task, users); err != nil {
				return err
			}
		}
		if !previousStatus && task.Status {
			now := time.Now().UTC()
			task.CompletedAt = &now
			task.CompletedByID = &actorID
		} else if !task.Status {
			task.CompletedAt = nil
			task.CompletedByID = nil
		}
		if err = tx.Model(&task).Select("Message", "Status", "Priority", "CompletedAt", "CompletedByID").Updates(&task).Error; err != nil {
			return err
		}
		recipients := taskUsers(task)
		for _, id := range recipients {
			kind := ""
			if previousStatus && !task.Status {
				kind = "reopened"
			} else if !containsTaskUser(previousUsers, id) {
				kind = "assigned"
			}
			if kind != "" {
				if err = notifyTask(tx, task, actor, id, kind); err != nil {
					return err
				}
			}
		}
		if !previousStatus && task.Status {
			recipients = append(recipients, task.FromUserID)
			seen := map[uint]bool{actorID: true}
			for _, id := range recipients {
				if seen[id] {
					continue
				}
				seen[id] = true
				if err = notifyTask(tx, task, actor, id, "completed"); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return task, err
}

// --------- конец нового кода chat
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
		if err := tx.Model(&task).Association("Assignees").Clear(); err != nil {
			return err
		} // новый код chat
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
