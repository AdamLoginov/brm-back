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

func GetArgeementEstimatehandler(w http.ResponseWriter, r *http.Request) {
	var estimate []models.Estimate
	var idAgreement = r.PathValue("id")

	var idAgreementInt, _ = strconv.Atoi(idAgreement)

	if err := database.DB.Where("agreement_id = ?", idAgreementInt).Find(&estimate).Error; err != nil {
		http.Error(w, "Ошибка поиска сметы", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(estimate)
}

func GetEstimateHandler(w http.ResponseWriter, r *http.Request) {
	var estimateId = r.PathValue("id")
	var estimate models.Estimate

	if err := database.DB.Preload("Materials.OrderMaterial.Order").First(&estimate, estimateId).Error; err != nil {
		msg := "[Error] Ошибка при получении данных сметы"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(estimate)
}

func DeleteArgeementEstimateHandler(w http.ResponseWriter, r *http.Request) {
	var estimateID = r.PathValue("id")

	if err := database.DB.Delete(&models.Estimate{}, estimateID).Error; err != nil {
		http.Error(w, "Не удалось удалить смету", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Смета успешна удалена!"})
}

func CreateEstimateHandler(w http.ResponseWriter, r *http.Request) {
	var estimate models.Estimate
	var idAgreement = r.PathValue("id")
	var nameEstimate = r.FormValue("name")

	idAgreementInt, _ := strconv.Atoi(idAgreement)

	estimate = models.Estimate{
		Name:        nameEstimate,
		AgreementID: uint(idAgreementInt),
	}

	if err := database.DB.Create(&estimate).Error; err != nil {
		http.Error(w, "Не удалось создать смету", http.StatusBadRequest)
		return
	}

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

	sheetList := f.GetSheetList()
	if len(sheetList) == 0 {
		http.Error(w, "В Excel файле нет ни одного листа", http.StatusBadRequest)
		return
	}

	// 2. Берем имя самого первого листа (не важно, как он называется: "Sheet1", "Лист1" или "Смета")
	firstSheetName := sheetList[0]

	// 3. Читаем строки из автоматически определенного листа
	rows, err := f.GetRows(firstSheetName)
	if err != nil {
		http.Error(w, "Не удалось прочитать строки из листа: "+firstSheetName, http.StatusBadRequest)
		return
	}

	flag := false
	for _, row := range rows {
		if len(row) < 10 {
			continue
		}

		if strings.Contains(row[1], "Обоснование") {
			flag = true
		}

		if !flag || len(row[2]) < 2 || strings.Contains(row[2], "Наименование работ") {
			continue
		}

		var rowEstimate map[string]int

		if r.FormValue("type") == "12" {
			rowEstimate = map[string]int{
				"idSmeta":   0,
				"cellValue": 1,
				"name":      2,
				"typeUnit":  5,
				"quantity":  8,
				"price":     9,
			}
		} else {
			rowEstimate = map[string]int{
				"idSmeta":   0,
				"cellValue": 1,
				"name":      2,
				"typeUnit":  5,
				"quantity":  6,
				"price":     7,
			}
		}

		cellValue := row[rowEstimate["cellValue"]]

		// if strings.Contains(cellValue, "ФССЦ") && !strings.Contains(cellValue, "ФССЦпг") {
		if strings.TrimSpace(cellValue) != "" && !strings.Contains(cellValue, "ФЕР") && strings.TrimSpace(row[rowEstimate["idSmeta"]]) != "" && !strings.Contains(cellValue, "ФССЦпг") {
			qty := parseExcelFloat(row[rowEstimate["quantity"]])
			price := parseExcelFloat(row[rowEstimate["price"]])

			materials = append(materials, models.Materials{
				IdSmeta:    strings.TrimSpace(row[rowEstimate["idSmeta"]]),
				Name:       row[rowEstimate["name"]],
				TypeUnit:   row[rowEstimate["typeUnit"]],
				Quantity:   qty,
				PriceSmeta: price,
				EstimateID: estimate.ID,
			})
		}
	}

	if len(materials) > 0 {
		if err := database.DB.CreateInBatches(&materials, 100).Error; err != nil {
			log.Printf("Ошибка при сохранении: %v", err)
		}
	}
}
