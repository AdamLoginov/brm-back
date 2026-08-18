package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"project/internal/database"
	"project/internal/models"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

func GetAllMaterials(w http.ResponseWriter, r *http.Request) {
	var materials []models.Materials
	if err := database.DB.Preload("Estimate").Order("name ASC").Find(&materials).Error; err != nil {
		http.Error(w, "Ошибка получения списка", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(materials)
}

func CreateMaterial(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name     string  `json:"name"`
		TypeUnit string  `json:"typeUnit"`
		Quantity float64 `json:"quantity"`
	}

	json.NewDecoder(r.Body).Decode(&input)

	if err := database.DB.Create(&models.Materials{
		Name:     input.Name,
		TypeUnit: input.TypeUnit,
		Quantity: input.Quantity,
	}).Error; err != nil {
		http.Error(w, "Не удалось создать материал", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Карточка материала создана"})
}

func UploadMaterialsExcel(w http.ResponseWriter, r *http.Request) {
	file, _, err := r.FormFile("ExcelFile")
	if err != nil {
		http.Error(w, "не удалось получить файл", http.StatusInternalServerError)
		return
	}
	defer file.Close()
	println("Файл получен")
	f, err := excelize.OpenReader(file)
	if err != nil {
		http.Error(w, "Не удалось прочитать файл", http.StatusBadRequest)
		return
	}

	defer f.Close()

	var materials []models.Materials

	rows, err := f.GetRows("Sheet1")
	if err != nil {
		http.Error(w, "Не удалось прочитать файл", http.StatusBadRequest)
		return
	}

	for _, row := range rows {
		if len(row) < 10 {
			continue
		}
		cellValue := row[1]

		if strings.Contains(cellValue, "ФССЦ") && !strings.Contains(cellValue, "ФССЦпг") {
			// idSmeta, _ := strconv.Atoi(strings.TrimSpace(row[0]))
			qty := parseExcelFloat(row[8])
			price := parseExcelFloat(row[9])

			materials = append(materials, models.Materials{
				IdSmeta:    strings.TrimSpace(row[0]),
				Name:       row[2],
				TypeUnit:   row[5],
				Quantity:   qty,
				PriceSmeta: price,
			})
		}
	}
	if len(materials) > 0 {
		if err := database.DB.CreateInBatches(&materials, 100).Error; err != nil {
			log.Printf("Ошибка при сохранении: %v", err)
		}
	}
}

func DeleteMaterials(w http.ResponseWriter, r *http.Request) {
	idMaterials := r.PathValue("id")
	if err := database.DB.Delete(&models.Materials{}, idMaterials).Error; err != nil {
		http.Error(w, "Не удалось удалить материал", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Материал успешно удален!"})
}

func parseExcelFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if strings.Contains(s, ",") && strings.Contains(s, ".") {
		s = strings.ReplaceAll(s, ",", "")
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}

	return val
}
