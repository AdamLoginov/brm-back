// --------- новый код chat: авторизованная выгрузка всех позиций выбранной книги
package handlers

import (
	"github.com/xuri/excelize/v2" // новый код chat
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
	// новый код chat: отдельный шаблон для выбранной позиции
	var f *excelize.File
	var err error
	if r.PathValue("statementMode") != "" { // новый код chat
		if r.PathValue("statementMode") != "all" {
			http.NotFound(w, r)
			return
		}
		data, err := service.BuildAllWorkOrderPositionStatements(database.DB, a, bookID) // новый код chat: архив отдельных ведомостей
		if err != nil {
			workError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="work-order-statements.zip"`)
		w.Header().Set("Cache-Control", "no-store")
		w.Write(data)
		return
	} else if r.PathValue("blockId") != "" { // новый код chat
		blockID := workID(r, "blockId")
		if blockID == 0 {
			http.Error(w, "Выберите блок", 400)
			return
		}
		f, err = service.BuildWorkOrderBlockStatement(database.DB, a, bookID, blockID)
	} else if r.PathValue("positionId") != "" {
		positionID := workID(r, "positionId")
		if positionID == 0 {
			http.Error(w, "Выберите конкретную позицию", 400)
			return
		}
		f, err = service.BuildWorkOrderPositionStatement(database.DB, a, bookID, positionID)
	} else {
		f, err = service.BuildWorkOrderPositionExcel(database.DB, a, bookID)
	}
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
