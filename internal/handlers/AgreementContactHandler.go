// --------- новый код chat: контакты представителей контрагента внутри договора
package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"project/internal/database"
	"project/internal/models"
	"strconv"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"
)

type agreementContactInput struct {
	FullName string   `json:"full_name"`
	Position string   `json:"position"`
	Phones   []string `json:"phones"`
	Emails   []string `json:"emails"`
	Comment  string   `json:"comment"`
}

func cleanContactValues(values []string, emails bool) ([]string, error) {
	if len(values) > 30 {
		return nil, errors.New("Допустимо не более 30 телефонов или email")
	}
	result := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		limit := 100
		if emails {
			limit = 254
		}
		if utf8.RuneCountInString(value) > limit {
			return nil, errors.New("Слишком длинный телефон или email")
		}
		key := value
		if emails {
			address, err := mail.ParseAddress(value)
			if err != nil || address.Address != value || strings.ContainsAny(value, "\r\n") {
				return nil, errors.New("Укажите корректный email")
			}
			key = strings.ToLower(value)
		}
		if !seen[key] {
			result = append(result, value)
			seen[key] = true
		}
	}
	return result, nil
}

func AgreementContactHandler(w http.ResponseWriter, r *http.Request) {
	agreementID, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
	if err != nil || agreementID == 0 {
		http.Error(w, "Некорректный договор", 400)
		return
	}
	var agreement models.Agreement
	if err := database.DB.First(&agreement, agreementID).Error; err != nil {
		contactDBError(w, err, "Договор не найден")
		return
	}
	if r.Method == http.MethodGet {
		contacts := []models.AgreementContact{}
		if err := database.DB.Where("agreement_id = ?", agreementID).Order("full_name, id").Find(&contacts).Error; err != nil {
			contactDBError(w, err, "Контакты не найдены")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(contacts)
		return
	}
	var contact models.AgreementContact
	if r.Method != http.MethodPost {
		id, err := strconv.ParseUint(r.PathValue("contactId"), 10, 32)
		if err != nil || id == 0 {
			http.Error(w, "Некорректный контакт", 400)
			return
		}
		if err := database.DB.Where("id = ? AND agreement_id = ?", id, agreementID).First(&contact).Error; err != nil {
			contactDBError(w, err, "Контакт не найден в договоре")
			return
		}
	}
	if r.Method == http.MethodDelete {
		if err := database.DB.Delete(&contact).Error; err != nil {
			contactDBError(w, err, "Контакт не найден")
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost && r.Method != http.MethodPatch {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var input agreementContactInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		http.Error(w, "Некорректные данные контакта", 400)
		return
	}
	input.FullName = strings.Join(strings.Fields(input.FullName), " ")
	input.Position, input.Comment = strings.TrimSpace(input.Position), strings.TrimSpace(input.Comment)
	if input.FullName == "" || utf8.RuneCountInString(input.FullName) > 200 {
		http.Error(w, "Укажите ФИО от 1 до 200 символов", 400)
		return
	}
	if utf8.RuneCountInString(input.Position) > 200 || utf8.RuneCountInString(input.Comment) > 5000 {
		http.Error(w, "Должность — до 200 символов, комментарий — до 5000", 400)
		return
	}
	input.Phones, err = cleanContactValues(input.Phones, false)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	input.Emails, err = cleanContactValues(input.Emails, true)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	contact.AgreementID = uint(agreementID)
	contact.FullName, contact.Position, contact.Comment = input.FullName, input.Position, input.Comment
	contact.Phones, contact.Emails = input.Phones, input.Emails
	if r.Method == http.MethodPost {
		err = database.DB.Create(&contact).Error
	} else {
		// Select сохраняет также пустые поля и списки; Updates не создаёт удалённый контакт заново.
		result := database.DB.Model(&models.AgreementContact{}).Where("id = ? AND agreement_id = ?", contact.ID, agreementID).
			Select("FullName", "Position", "Phones", "Emails", "Comment").Updates(&contact)
		err = result.Error
		if err == nil && result.RowsAffected == 0 {
			http.Error(w, "Контакт не найден", 404)
			return
		}
	}
	if err != nil {
		contactDBError(w, err, "Контакт не найден")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(contact)
}

func contactDBError(w http.ResponseWriter, err error, missing string) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		http.Error(w, missing, 404)
	} else {
		http.Error(w, "Не удалось обработать контакты", 500)
	}
}

// --------- конец нового кода chat
