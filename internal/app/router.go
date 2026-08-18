package app

import (
	"net/http"
	"project/internal/handlers"
	"project/internal/middleware"
)

func NewRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /employeecards", middleware.AuthMiddleware(handlers.GetAllEmployeeCardHandler))
	mux.HandleFunc("GET /employeescard/{id}/detail", middleware.AuthMiddleware(handlers.GetEmployeeCardDetailHandler))
	mux.HandleFunc("POST /employeescard/create", middleware.AuthMiddleware(handlers.CreateEmployeeCardHandler))
	mux.HandleFunc("POST /employeescard/{id}/document/create", middleware.AuthMiddleware(handlers.CreateEmployeeCardDocumentHandler))
	mux.HandleFunc("DELETE /employeescard/delete/{id}", middleware.AuthMiddleware(handlers.DeleteEmployeeCardHandler))

	mux.HandleFunc("GET /users/{id}", middleware.AuthMiddleware(handlers.GetUserHandler))
	mux.HandleFunc("GET /users", middleware.AuthMiddleware(handlers.GetAllUserHandler))

	mux.HandleFunc("DELETE /materials/delete/{id}", middleware.AuthMiddleware(handlers.DeleteMaterials))
	mux.HandleFunc("POST /materials/create", middleware.AuthMiddleware(handlers.CreateMaterial))
	mux.HandleFunc("POST /materials/upload/excel", middleware.AuthMiddleware(handlers.UploadMaterialsExcel))
	mux.HandleFunc("GET /materials", middleware.AuthMiddleware(handlers.GetAllMaterials))

	//Supplier Categoryes Handlers
	mux.HandleFunc("DELETE /suppliers/categoryes/{id}", middleware.AuthMiddleware(handlers.DeleteCategorySupplierHandler))
	mux.HandleFunc("POST /suppliers/categoryes/create", middleware.AuthMiddleware(handlers.CreateCategorySupplierHandler))
	mux.HandleFunc("GET /suppliers/categoryes", middleware.AuthMiddleware(handlers.GetAllCategorySupplierHandler))

	// Supplier Handlers
	mux.HandleFunc("DELETE /supplier/delete/{id}", middleware.AuthMiddleware(handlers.DeleteSupplierHandler))
	mux.HandleFunc("DELETE /supplier/manager/delete/{id}", middleware.AuthMiddleware(handlers.DeleteManagerCard))
	mux.HandleFunc("POST /supplier/{id}/manager/create", middleware.AuthMiddleware(handlers.CreateManagerCrad))
	mux.HandleFunc("POST /supplier/create", middleware.AuthMiddleware(handlers.CreateSupler))
	mux.HandleFunc("GET /suppliers/detail/{id}", middleware.AuthMiddleware(handlers.GetDetailSupplier))
	mux.HandleFunc("GET /suppliers", middleware.AuthMiddleware(handlers.GetAllSupler))

	//Order
	mux.HandleFunc("POST /order/create", middleware.AuthMiddleware(handlers.CreateOrderHandler))
	mux.HandleFunc("GET /order/{id}", middleware.AuthMiddleware(handlers.GetPaidOrderHandler))
	mux.HandleFunc("GET /agreements/{id}/orders", middleware.AuthMiddleware(handlers.GetAgreementOrderHandler))
	mux.HandleFunc("GET /agreements/orders/{id}/detail", middleware.AuthMiddleware(handlers.GetAgreementOrderDetailHandler))
	mux.HandleFunc("POST /agreements/orders/{id}/update", middleware.AuthMiddleware(handlers.UpdateAgreementOrder))

	mux.HandleFunc("POST /agreements/employee/{id}/update", middleware.AuthMiddleware(handlers.PutAgreementEmployeeCard))

	//Estimate
	mux.HandleFunc("POST /agreements/{id}/estimates/create", middleware.AuthMiddleware(handlers.CreateEstimateHandler))
	mux.HandleFunc("DELETE /agreements/estimates/{id}", middleware.AuthMiddleware(handlers.DeleteArgeementEstimateHandler))
	mux.HandleFunc("GET /agreements/{id}/estimates", middleware.AuthMiddleware(handlers.GetArgeementEstimatehandler))
	mux.HandleFunc("GET /estimates/{id}", middleware.AuthMiddleware(handlers.GetEstimateHandler))

	mux.HandleFunc("DELETE /agreements/delete/{id}", middleware.AuthMiddleware(handlers.DeleteAgreementHandler))
	mux.HandleFunc("POST /agreements/create", middleware.AuthMiddleware(handlers.CreateAgreementHandler))
	mux.HandleFunc("GET /agreements/{id}", middleware.AuthMiddleware(handlers.GetDetailAgreementHandler))
	mux.HandleFunc("GET /agreements", middleware.AuthMiddleware(handlers.GetAllAgreementHandler))

	// Expenses
	mux.HandleFunc("POST /agreements/{id}/expenses/create", middleware.AuthMiddleware(handlers.CreateExpensesHandler))
	mux.HandleFunc("GET /agreements/{id}/expenses", middleware.AuthMiddleware(handlers.GetAgreementExpensesHandler))
	mux.HandleFunc("DELETE /expenses/delete/{id}", middleware.AuthMiddleware(handlers.DeleteExpensesHandleer))

	// tools
	mux.HandleFunc("POST /tools/create", middleware.AuthMiddleware(handlers.CreateToolHandler))
	mux.HandleFunc("DELETE /tools/delete/{id}", middleware.AuthMiddleware(handlers.DeleteToolHandler))
	mux.HandleFunc("GET /agreements/{id}/tools/create", middleware.AuthMiddleware(handlers.GetAgreementAllToolHandler))

	// Блок диалогов
	mux.HandleFunc("POST /dialog/create", middleware.AuthMiddleware(handlers.CreateDialogHandler))
	mux.HandleFunc("GET /dialogs/{id}", middleware.AuthMiddleware(handlers.GetDetailDialogHandler))
	mux.HandleFunc("GET /dialogs", middleware.AuthMiddleware(handlers.GetAllDialogHandler))

	// Блок email
	mux.HandleFunc("POST /emails/create", middleware.AuthMiddleware(handlers.CreateEmailHandler))
	mux.HandleFunc("GET /emails/read/{id}", middleware.AuthMiddleware(handlers.ReadEmailhandler))
	mux.HandleFunc("GET /emails", middleware.AuthMiddleware(handlers.GetAllEmailsHandler))
	mux.HandleFunc("GET /emails/{id}", middleware.AuthMiddleware(handlers.GetDetailEmailHandler))

	// Блок Attachemt (файлы)
	mux.HandleFunc("GET /attachment/{id}", handlers.DownloadAttachmenthandler)

	mux.HandleFunc("POST /login", handlers.LoginHandler)

	return middleware.CorsMiddleware(mux)
}
