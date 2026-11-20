// --------- новый код chat: авторизованная выгрузка всех позиций выбранной книги
package handlers

import (
	"net/http"
	"project/internal/database"
	"project/internal/service"
)

func WorkOrderPositionExcelHandler(w http.ResponseWriter, r *http.Request) {
	a := workAgreement(w, r)
	if a == 0 {
		return
	}
	bookID := workID(r, "bookId")
	if bookID == 0 {
		http.Error(w, "Не указана книга", 400)
		return
	}
	f, err := service.BuildWorkOrderPositionExcel(database.DB, a, bookID)
	if err != nil {
		workError(w, err)
		return
	}
	defer f.Close()
	buffer, err := f.WriteToBuffer()
	if err != nil {
		workError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="work-order-positions.xlsx"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Write(buffer.Bytes())
}

// --------- конец нового кода chat
