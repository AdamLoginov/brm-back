package mailworker

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"project/internal/database"
	"project/internal/models"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/mail"
	"gorm.io/gorm"
)

const (
	imapHost     = "imap.mail.ru:993"
	imapUser     = "mysnikov2000@mail.ru"
	imapPassword = "ZrJfFfb8BcOV5MFdoAuA" // Твой пароль приложения
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

			var section imap.BodySectionName
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

				rawText, attchments := parseEmailContent(msg.GetBody(&section))

				newEmail := models.Email{
					EmailTo:     emailTo,
					EmailFrom:   emailFrom,
					Title:       msg.Envelope.Subject,
					Message:     strings.TrimSpace(rawText),
					DialogID:    findDialog(emailFrom),
					Attachments: attchments,
				}

				if err := db.Create(&newEmail).Error; err != nil {
					log.Printf("[DB ERROR] Не удалось сохранить письмо: %v", err)
					continue
				}

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

	if err := c.Login(imapUser, imapPassword); err != nil {
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

func parseEmailContent(r io.Reader) (string, []models.Attachment) {
	if r == nil {
		return "", nil
	}

	mailReader, err := mail.CreateReader(r)
	if err != nil {
		log.Printf("[IMAP ERROR] Ошибка создания Mail Reader: %v", err)
		return "", nil
	}

	var bodyText string
	var attachments []models.Attachment

	for {
		part, err := mailReader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("[IMAP WARNING] Ошибка чтения части письма: %v", err)
			break
		}

		switch header := part.Header.(type) {

		case *mail.InlineHeader:
			contentType, _, _ := header.ContentType()
			if contentType == "text/plain" && bodyText == "" {
				bytesBody, err := io.ReadAll(part.Body)
				if err == nil {
					bodyText = string(bytesBody)
				}
			}
		case *mail.AttachmentHeader:
			filename, err := header.Filename()
			if err != nil || filename == "" {
				continue // Пропускаем файл, если не удалось прочитать имя
			}

			contentType, _, _ := header.ContentType()
			uniqueFileName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), filename)
			filePath := filepath.Join("./uploads", uniqueFileName)
			dst, err := os.Create(filePath)
			if err != nil {
				log.Printf("[IMAP ERROR] Не удалось создать файл на диске: %v", err)
				continue
			}
			writtenSize, err := io.Copy(dst, part.Body)
			dst.Close() // Обязательно закрываем файл сразу после записи!
			if err != nil {
				log.Printf("[IMAP ERROR] Ошибка при записи вложения: %v", err)
				_ = os.Remove(filePath) // Удаляем битый файл
				continue
			}
			attachment := models.Attachment{
				FileName: filename,
				FilePath: filePath,
				FileSize: writtenSize,
				MimeType: contentType,
			}

			attachments = append(attachments, attachment)
		}

	}

	return bodyText, attachments
}
