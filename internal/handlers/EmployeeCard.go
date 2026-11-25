package handlers

import (
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"log"
	"net/http"
	"os"
	"project/internal/database"
	"project/internal/middleware"
	"project/internal/models"
	"project/internal/service" // новый код chat
	"strconv"
)

func GetEmployeeCardDetailHandler(w http.ResponseWriter, r *http.Request) {
	var employeeCardId = r.PathValue("id")
	var employeeCard models.EmployeeCard

	if err := database.DB.Preload("Specializations").Preload("EmployeeDocuments.CategoryEmployeeDocument").Preload("Agreement").First(&employeeCard, employeeCardId).Error; err != nil {
		msg := "[Error] Ошибка при получении данных"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(employeeCard)
}

func GetAllEmployeeCardAgreementHandler(w http.ResponseWriter, r *http.Request) {
	var agreementID = r.PathValue("id")
	var employeeCards []models.EmployeeCard
	// --------- новый код chat: расширенный список только для выбора в табель
	query := database.DB.Where("agreement_id = ?", agreementID)
	if r.URL.Query().Get("timesheet_candidates") == "1" {
		query = database.DB.Where("(agreement_id = ? AND status = ?) OR (status = ? AND dismissal_date <> '')", agreementID, false, true)
	}
	// --------- конец нового кода chat
	if err := query.Find(&employeeCards).Error; err != nil {
		msg := "[Error] Ошибка при поиске"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(employeeCards)
}

func GetAllEmployeeCardHandler(w http.ResponseWriter, r *http.Request) {
	var employeeCard []models.EmployeeCard
	// новый код chat: документы для проверки заполненности загружаются только по запросу
	query := database.DB.Preload("Specializations").Preload("Agreement")
	if r.URL.Query().Get("include_documents") == "true" {
		query = query.Preload("EmployeeDocuments.CategoryEmployeeDocument")
	}
	if err := query.Find(&employeeCard).Error; err != nil {
		msg := "[Error] Ошибка получения карточек сотрудников"
		log.Println(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(employeeCard)
}

func CreateEmployeeCardHandler(w http.ResponseWriter, r *http.Request) {
	var employeeCard models.EmployeeCard

	// --------- новый код chat: проверка типов и новых полей карточки
	if err := json.NewDecoder(r.Body).Decode(&employeeCard); err != nil {
		http.Error(w, "Некорректные данные карточки сотрудника", http.StatusBadRequest)
		return
	}
	if !validateEmployeeSizes(w, &employeeCard) { // новый код chat: проверка роста
		return
	}
	if !validateEmployeeEmployment(w, &employeeCard) {
		return
	}
	// --------- конец нового кода chat
	if err := saveEmployeeProfile(&employeeCard, ""); err != nil {
		msg := "[Error] Не удалось создать карточку сотрудника"
		log.Println(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Карточка товара успешно создана!"})
}

func UpdateEmployeeCardHandler(w http.ResponseWriter, r *http.Request) {
	var employeeCardID = r.PathValue("id")
	var employeeCard models.EmployeeCard

	if err := json.NewDecoder(r.Body).Decode(&employeeCard); err != nil {
		msg := "[Error] Ошибка при чтении данных"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	if !validateEmployeeSizes(w, &employeeCard) { // новый код chat: проверка роста
		return
	}
	if !validateEmployeeEmployment(w, &employeeCard) {
		return
	} // новый код chat: проверка месячной зарплаты при изменении
	if err := saveEmployeeProfile(&employeeCard, employeeCardID); err != nil {
		msg := "[Error] Ошибка при обнавлении данных"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Данные на сотрудника успешно обновлены"})
}

func GetEmployeeCardDocumentDetailHandler(w http.ResponseWriter, r *http.Request) {
	var documentId = r.PathValue("id")
	var document models.EmployeeDocuments

	if err := database.DB.First(&document, documentId).Error; err != nil {
		http.Error(w, "Ошибка при поиске файла в БД", http.StatusInternalServerError)
		return
	}

	// --------- новый код chat: относительный ключ или старый путь
	filePath, err := service.FileLocation(database.DB, document.StoredFileID, document.FilePath)
	if err != nil {
		http.Error(w, "Не удалось найти файл", 404)
		return
	}
	if _, err := os.Stat(filePath); err != nil {
		http.Error(w, "Физический файл отсутствует на сервере", http.StatusNotFound)
		return
	}

	service.SetFileResponseHeaders(w, document.FileName, document.MimeType, false) // новый код chat: просмотр безопасных типов и скачивание остальных

	http.ServeFile(w, r, filePath) // новый код chat
}

// --------- новый код chat: загрузка через общее хранилище
func CreateEmployeeCardDocumentHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, service.MaxStoredFileSize+(1<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		http.Error(w, "Некорректная форма или превышен размер файла (64 МБ)", http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Выберите файл", 400)
		return
	}
	defer file.Close()
	employeeID, err := strconv.ParseUint(r.PathValue("id"), 10, 32)
	if err != nil || employeeID == 0 {
		http.Error(w, "Некорректный сотрудник", 400)
		return
	}
	categoryID, err := strconv.ParseUint(r.FormValue("category_employee_document_id"), 10, 32)
	if err != nil || categoryID == 0 {
		http.Error(w, "Выберите категорию", 400)
		return
	}
	service.FileStorageMutex.Lock()
	defer service.FileStorageMutex.Unlock()
	batch := &service.FileStorageBatch{}
	defer batch.Close()
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		var employee models.EmployeeCard
		if err := tx.First(&employee, employeeID).Error; err != nil {
			return err
		}
		var category models.CategoryEmployeeDocument
		if err := tx.First(&category, categoryID).Error; err != nil {
			return err
		}
		_, err := service.SaveEmployeeDocument(tx, batch, employee, category, header.Filename, file, middleware.CurrentUserID(r))
		return err
	})
	if err != nil {
		log.Printf("[UPLOAD ERROR] %v", err)
		status := http.StatusInternalServerError
		if errors.Is(err, gorm.ErrRecordNotFound) {
			status = http.StatusNotFound
		}
		if errors.Is(err, service.ErrFileTooLarge) {
			status = http.StatusRequestEntityTooLarge
		}
		http.Error(w, "Не удалось сохранить документ", status)
		return
	}
	batch.Commit()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Документ на сотрудника успешно добавлен"})
}

// --------- конец нового кода chat

func DeleteEmployeeCardDocumentHandler(w http.ResponseWriter, r *http.Request) {
	var documentId = r.PathValue("id")

	if err := database.DB.Delete(&models.EmployeeDocuments{}, documentId).Error; err != nil {
		msg := "Ошибка при удалении данных с сервера"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Документ сотрудника успешно удален!"})
}

func PutAgreementEmployeeCard(w http.ResponseWriter, r *http.Request) {
	idEmployeeCard := r.PathValue("id")
	// AgreementID := r.PathValue("id_agreement")
	var input struct {
		AgreementID uint `json:"agreement_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		log.Println("[ERROR] Ошибка в получении данных ")
		http.Error(w, "[ERROR] Ошибка в получении данных ", http.StatusBadRequest)
		return
	}
	// --------- новый код chat: нельзя заново закрепить уволенного сотрудника
	employeeProfileMutex.Lock()
	defer employeeProfileMutex.Unlock()
	var employee models.EmployeeCard
	if database.DB.First(&employee, "id = ?", idEmployeeCard).Error != nil {
		http.Error(w, "Сотрудник не найден", http.StatusNotFound)
		return
	}
	if employee.Status && input.AgreementID != 0 {
		http.Error(w, "Уволенного сотрудника нельзя закрепить за договором", http.StatusBadRequest)
		return
	}
	// --------- новый код chat: перевод и снятие ответственного выполняются вместе
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if employee.AgreementID != input.AgreementID {
			if err := tx.Model(&models.Tool{}).Where("employee_card_id = ?", employee.ID).Update("employee_card_id", nil).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.Ppe{}).Where("employee_card_id = ?", employee.ID).Update("employee_card_id", nil).Error; err != nil { // новый код chat: СИЗ открепляется при переводе
				return err
			}
		}
		return tx.Model(&employee).Update("agreement_id", input.AgreementID).Error
	}); err != nil {
		http.Error(w, "Не удалось изменить договор сотрудника", http.StatusInternalServerError)
		return
	}
	// --------- конец нового кода chat

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Карточка сотрудника успешно обновлена!"})
}

func DeleteEmployeeCardHandler(w http.ResponseWriter, r *http.Request) {
	idEmployeeCard := r.PathValue("id")

	if err := database.DB.Delete(&models.EmployeeCard{}, idEmployeeCard).Error; err != nil {
		msg := "Не удалось удалить карточку сотрудника"
		log.Println(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Карточка сотрудника успешно удалена!"})
}

func GetAllCategoryEmployeeDocumentHandler(w http.ResponseWriter, r *http.Request) {
	var categoryEmployeeDocument []models.CategoryEmployeeDocument

	if err := database.DB.Find(&categoryEmployeeDocument).Error; err != nil {
		msg := "[Error] Ошибка при поиске категорий"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(categoryEmployeeDocument)
}

func CreateCategoryEmployeeDocumentHandler(w http.ResponseWriter, r *http.Request) {
	var categoryEmployeeDocument models.CategoryEmployeeDocument

	if err := json.NewDecoder(r.Body).Decode(&categoryEmployeeDocument); err != nil {
		msg := "[Error] Ошибка при чтении данных"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	if err := database.DB.Create(&categoryEmployeeDocument).Error; err != nil {
		msg := "[Error] Ошибка при создании категории"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(categoryEmployeeDocument)
}

func DeleteCategoryEmployeeDocumentHandler(w http.ResponseWriter, r *http.Request) {
	var categoryID = r.PathValue("id")

	if err := database.DB.Delete(&models.CategoryEmployeeDocument{}, categoryID).Error; err != nil {
		msg := "[Error] Ошибка при удалении файла"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Категория успешно удалена!"})
}
