package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"project/internal/database"
	"project/internal/models"
	"strconv"
	"time"
)

func GetEmployeeCardDetailHandler(w http.ResponseWriter, r *http.Request) {
	var employeeCardId = r.PathValue("id")
	var employeeCard models.EmployeeCard

	if err := database.DB.Preload("EmployeeDocuments.CategoryEmployeeDocument").Preload("Agreement").First(&employeeCard, employeeCardId).Error; err != nil {
		msg := "[Error] Ошибка при получении данных"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(employeeCard)
}

func GetAllEmployeeCardHandler(w http.ResponseWriter, r *http.Request) {
	var employeeCard []models.EmployeeCard
	if err := database.DB.Find(&employeeCard).Error; err != nil {
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

	json.NewDecoder(r.Body).Decode(&employeeCard)
	if err := database.DB.Create(&employeeCard).Error; err != nil {
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

	if err := database.DB.Model(&models.EmployeeCard{}).Where("id = ?", employeeCardID).Select("*").
		Omit("id", "created_at", "updated_at", "agreement", "employee_documents").Updates(employeeCard).Error; err != nil {
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

	if _, err := os.Stat(document.FilePath); os.IsNotExist(err) {
		http.Error(w, "Физический файл отсутствует на сервере", http.StatusNotFound)
		return
	}

	if document.MimeType != "" {
		w.Header().Set("Content-Type", document.MimeType)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"%s\"", document.FileName))
	http.ServeFile(w, r, document.FilePath)
}

func CreateEmployeeCardDocumentHandler(w http.ResponseWriter, r *http.Request) {
	var employeeCardID = r.PathValue("id")

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Ошибка при парсинге формы", http.StatusBadRequest)
		return
	}

	file, handler, err := r.FormFile("file")
	if err == nil {
		uniqueFileName := fmt.Sprintf("%s_%d", handler.Filename, time.Now().Unix())
		filePath := filepath.Join("./uploads/documents", uniqueFileName)
		dst, err := os.Create(filePath)
		if err != nil {
			log.Printf("[UPLOAD ERROR] Не удалось создать файл на диске: %v", err)
			http.Error(w, "Ошибка сохранения файла", http.StatusInternalServerError)
			return
		}
		defer dst.Close()

		if _, err := io.Copy(dst, file); err != nil {
			http.Error(w, "Ошибка при записи файла", http.StatusInternalServerError)
			return
		}

		_, _ = file.Seek(0, 0)
		buffer := make([]byte, 512)
		_, _ = file.Read(buffer)

		// Go сам проанализирует байты и вернет точный MIME-тип (например "application/pdf")
		mimeType := http.DetectContentType(buffer)

		employeeCardID_uint, err := strconv.ParseUint(employeeCardID, 10, 32)
		categoryID, _ := strconv.ParseUint(r.FormValue("category_employee_document_id"), 10, 32)

		employeeDocument := models.EmployeeDocuments{
			// DocumentName:               r.FormValue("document_name"),
			FileName:                   handler.Filename,
			FilePath:                   filePath,
			FileSize:                   handler.Size,
			MimeType:                   mimeType,
			EmployeeCardID:             uint(employeeCardID_uint),
			CategoryEmployeeDocumentID: uint(categoryID),
		}

		if err := database.DB.Create(&employeeDocument).Error; err != nil {
			msg := "[Error] Ошибка при сохранении данных документа"
			log.Println(msg)
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Документ на сотрудника успешно добавлен"})
}

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
	log.Println("------------------------")
	log.Println(idEmployeeCard, input.AgreementID)
	if err := database.DB.Model(&models.EmployeeCard{}).Where("id = ?", idEmployeeCard).Update("agreement_id", input.AgreementID); err != nil {
		log.Println("[ERROR] Ошибка при внесении данных в базу ")
		http.Error(w, "[ERROR] Ошибка при внесении данных в базу ", http.StatusBadRequest)
		return
	}

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
