package mailworker

import (
	"fmt"
	"io"
	"log"
	"os"
	"project/internal/database"
	"project/internal/models"
	"project/internal/service"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
	"gorm.io/gorm"
)

const (
	imapHost = "imap.mail.ru:993"
	imapUser = "mysnikov2000@mail.ru"
)

func StartEmailMonitoring(db *gorm.DB) {
	ticker := time.NewTicker(60 * time.Second) // поменять на 60 секунд

	go func() {
		for range ticker.C {
			log.Printf("[Письма] Начало поиска писем: ")
			connection, err := connectIMAP()
			if err != nil {
				log.Printf("[IMAP ERROR] Не удалось установить сессию: %v", err)
				continue
			}

			criteria := imap.NewSearchCriteria()
			criteria.WithoutFlags = []string{imap.SeenFlag}
			ids, err := connection.Search(criteria)
			if err != nil || len(ids) == 0 {
				connection.Logout()
				continue
			}

			seqset := new(imap.SeqSet)
			seqset.AddNum(ids...)

			section := imap.BodySectionName{Peek: true} // новый код chat: отметка прочитано только после сохранения
			items := []imap.FetchItem{imap.FetchEnvelope, section.FetchItem()}

			messages := make(chan *imap.Message, 10)
			done := make(chan error, 1)

			go func() {
				done <- connection.Fetch(seqset, items, messages)
			}()

			for msg := range messages {
				emailTo := ""
				if len(msg.Envelope.To) > 0 {
					emailTo = msg.Envelope.To[0].Address()
				}

				emailFrom := ""
				if len(msg.Envelope.From) > 0 {
					emailFrom = msg.Envelope.From[0].Address()
				}

				// --------- новый код chat: письмо и файлы сохраняются вместе
				newEmail := models.Email{EmailTo: emailTo, EmailFrom: emailFrom, Title: msg.Envelope.Subject, DialogID: findDialog(emailFrom)}
				if err := saveIncomingEmail(db, msg.GetBody(&section), &newEmail); err != nil {
					log.Printf("[DB ERROR] Не удалось сохранить письмо: %v", err)
					continue
				}
				// --------- конец нового кода chat

				singleSeq := new(imap.SeqSet)
				singleSeq.AddNum(msg.SeqNum)
				connection.Store(singleSeq, imap.AddFlags, []interface{}{imap.SeenFlag}, nil)
				log.Printf("[Пришло писмьо №%d] : ", newEmail.ID)
			}
			<-done
			connection.Logout()
		}
	}()
}

func connectIMAP() (*client.Client, error) {
	c, err := client.DialTLS(imapHost, nil)
	if err != nil {
		return nil, fmt.Errorf("подключение: %w", err)
	}

	if err := c.Login(imapUser, os.Getenv("MAIL_PASSWORD")); err != nil {
		c.Logout()
		return nil, fmt.Errorf("авторизация: %w", err)
	}

	if _, err := c.Select("INBOX", false); err != nil {
		c.Logout()
		return nil, fmt.Errorf("выбор INBOX: %w", err)
	}

	return c, nil
}

func findDialog(emailFrom string) uint {
	var dialog models.Dialog
	if err := database.DB.Where("email_to = ?", emailFrom).First(&dialog).Error; err != nil {
		log.Printf("Не могу найти диалог с %s", emailFrom)
		return 0
	}
	return dialog.ID
}

// --------- новый код chat: единое хранилище вложений входящей почты
func saveIncomingEmail(db *gorm.DB, reader io.Reader, email *models.Email) error {
	service.FileStorageMutex.Lock()
	defer service.FileStorageMutex.Unlock()
	batch := &service.FileStorageBatch{}
	defer batch.Close()
	err := db.Transaction(func(tx *gorm.DB) error {
		body, attachments, err := parseEmailContent(reader, tx, batch)
		if err != nil {
			return err
		}
		email.Message = strings.TrimSpace(body)
		email.Attachments = attachments
		return tx.Create(email).Error
	})
	if err == nil {
		batch.Commit()
	}
	return err
}
func parseEmailContent(r io.Reader, tx *gorm.DB, batch *service.FileStorageBatch) (string, []models.Attachment, error) {
	if r == nil {
		return "", nil, fmt.Errorf("пустое письмо")
	}
	reader, err := mail.CreateReader(r)
	if err != nil {
		return "", nil, err
	}
	defer reader.Close()
	var body string
	var attachments []models.Attachment
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", nil, err
		}
		switch header := part.Header.(type) {
		case *mail.InlineHeader:
			contentType, _, _ := header.ContentType()
			if contentType == "text/plain" && body == "" {
				data, err := io.ReadAll(io.LimitReader(part.Body, (4<<20)+1))
				if err != nil {
					return "", nil, err
				}
				if len(data) > 4<<20 {
					return "", nil, fmt.Errorf("текст письма превышает 4 МБ")
				}
				body = string(data)
			}
		case *mail.AttachmentHeader:
			filename, err := header.Filename()
			if err != nil || filename == "" {
				return "", nil, fmt.Errorf("не удалось прочитать имя вложения")
			}
			attachment, err := service.SaveMailAttachment(tx, batch, filename, part.Body, "incoming", 0)
			if err != nil {
				return "", nil, err
			}
			attachments = append(attachments, attachment)
		}
	}
	return body, attachments, nil
}

// --------- конец нового кода chat
