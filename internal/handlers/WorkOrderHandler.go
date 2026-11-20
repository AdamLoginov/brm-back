// --------- новый код chat: API книги и смен
package handlers

import (
	"net/http"
	"project/internal/database"
	"project/internal/models"
	"project/internal/service"
	"strconv"
)

func WorkOrderHandler(w http.ResponseWriter, r *http.Request) {
	a := workAgreement(w, r)
	if a == 0 {
		return
	}
	if r.PathValue("action") == "shifts" {
		id := workID(r, "shiftId")
		if (r.Method == "PATCH" || r.Method == "DELETE") && id == 0 {
			http.Error(w, "Не указана смена", 400)
			return
		}
		if r.Method == "DELETE" {
			version, _ := strconv.ParseInt(r.URL.Query().Get("version"), 10, 64)
			if err := service.DeleteWorkShift(database.DB, a, id, version); err != nil {
				workError(w, err)
				return
			}
			w.WriteHeader(204)
			return
		}
		var input service.WorkShiftInput
		if !workDecode(w, r, &input) {
			return
		}
		shift, warning, err := service.SaveWorkShift(database.DB, a, id, input)
		if err != nil {
			workError(w, err)
			return
		}
		workJSON(w, map[string]interface{}{"shift": shift, "warning": warning})
		return
	}
	if r.Method == "GET" {
		if r.PathValue("bookId") != "" {
			book, err := service.LoadWorkBook(database.DB, a, workID(r, "bookId"))
			if err != nil {
				workError(w, err)
				return
			}
			workJSON(w, book)
			return
		}
		rows := []models.WorkOrderBook{}
		if err := database.DB.Where("agreement_id = ?", a).Order("year DESC, month DESC").Find(&rows).Error; err != nil {
			workError(w, err)
			return
		}
		workJSON(w, rows)
		return
	}
	var input struct {
		Year  int `json:"year"`
		Month int `json:"month"`
	}
	if !workDecode(w, r, &input) {
		return
	}
	book, err := service.CreateWorkBook(database.DB, a, input.Year, input.Month)
	if err != nil {
		workError(w, err)
		return
	}
	workJSON(w, book)
}

// --------- конец нового кода chat
