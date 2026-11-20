// --------- новый код chat: двухэтапный импорт общего журнала
package handlers

import (
	"io"
	"net/http"
	"project/internal/database"
	"project/internal/service"
)

func WorkOrderExcelHandler(w http.ResponseWriter, r *http.Request) {
	a := workAgreement(w, r)
	if a == 0 {
		return
	}
	bookID := workID(r, "bookId")
	if bookID == 0 {
		http.Error(w, "Не указана книга", 400)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Не удалось загрузить файл. Максимальный размер — 10 МБ", 400)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Выберите файл XLSX", 400)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Не удалось прочитать файл", 400)
		return
	}
	commit := r.PathValue("excelAction") == "confirm"
	if !commit && r.PathValue("excelAction") != "preview" {
		http.NotFound(w, r)
		return
	}
	report, err := service.ImportWorkOrderExcel(database.DB, a, bookID, data, r.FormValue("token"), commit)
	if err != nil {
		workError(w, err)
		return
	}
	workJSON(w, report)
}

// --------- конец нового кода chat
