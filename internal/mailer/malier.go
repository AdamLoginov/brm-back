package mailer

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"project/internal/models" // Замени на свой путь

	"github.com/jordan-wright/email"
)

// Константы для настройки почты
const (
	smtpHost     = "smtp.mail.ru"
	smtpPort     = "465"
	smtpUser     = "mysnikov2000@mail.ru"
	smtpPassword = "ZrJfFfb8BcOV5MFdoAuA"
)

func SendOrderEmail(msg models.Email, flagCompanyCard bool) error {
	address := smtpHost + ":" + smtpPort

	messageBytes, err := buildMessage(msg, flagCompanyCard)
	if err != nil {
		return fmt.Errorf("ошибка формирования сообщения: %w", err)
	}

	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
		ServerName:         smtpHost,
	}

	conn, err := tls.Dial("tcp", address, tlsConfig)
	if err != nil {
		return fmt.Errorf("ошибка TLS подключения: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, smtpHost)
	if err != nil {
		return fmt.Errorf("ошибка SMTP клиента: %w", err)
	}
	defer client.Quit()

	auth := smtp.PlainAuth("", smtpUser, smtpPassword, smtpHost)
	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("ошибка авторизации: %w", err)
	}

	if err = client.Mail(smtpUser); err != nil {
		return err
	}
	if err = client.Rcpt(msg.EmailTo); err != nil {
		return err
	}

	w, err := client.Data()
	if err != nil {
		return err
	}
	defer w.Close()

	_, err = w.Write(messageBytes)
	if err != nil {
		return err
	}

	return nil
}

func buildMessage(msg models.Email, flag bool) ([]byte, error) {
	e := email.NewEmail()
	e.From = smtpUser
	e.To = []string{msg.EmailTo}
	e.Subject = msg.Title
	e.Text = []byte(msg.Message) // Просто приводим твою message string к байтам

	if msg.InReplyTo != "" {
		e.Headers.Set("Reply-To", msg.InReplyTo)
	}

	if flag {
		if _, err := e.AttachFile("./uploads/company_card.pdf"); err != nil {
			log.Fatalf("[Ошибка] При отправке файла карточки компании")
			return nil, err
		}
	}

	if len(msg.Attachments) > 0 {
		for _, att := range msg.Attachments {
			if att.FilePath == "" {
				continue
			}

			if _, err := e.AttachFile(att.FilePath); err != nil {
				log.Fatalf("[Ошибка] При отправке файла id=%d произошла ошибка", att.ID)
				return nil, err
			}
		}
	}
	return e.Bytes()
}
