package app

import (
	"net/http"
	"project/internal/handlers"
	"project/internal/middleware"
)

func NewRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/employeecards", middleware.AuthMiddleware(handlers.GetAllEmployeeCardHandler))
	mux.HandleFunc("GET /api/employeescard/{id}/detail", middleware.AuthMiddleware(handlers.GetEmployeeCardDetailHandler))
	mux.HandleFunc("GET /api/employeescard/document/{id}/detail", handlers.GetEmployeeCardDocumentDetailHandler)
	mux.HandleFunc("GET /api/employeescard/document/category", middleware.AuthMiddleware(handlers.GetAllCategoryEmployeeDocumentHandler))
	mux.HandleFunc("POST /api/employeescard/document/category/create", middleware.AuthMiddleware(handlers.CreateCategoryEmployeeDocumentHandler))
	mux.HandleFunc("POST /api/employeescard/create", middleware.AuthMiddleware(handlers.CreateEmployeeCardHandler))
	mux.HandleFunc("POST /api/employeescard/{id}/document/create", middleware.AuthMiddleware(handlers.CreateEmployeeCardDocumentHandler))
	mux.HandleFunc("POST /api/employeescard/{id}/update", middleware.AuthMiddleware(handlers.UpdateEmployeeCardHandler))
	mux.HandleFunc("DELETE /api/employeescard/document/{id}/delete", middleware.AuthMiddleware(handlers.DeleteEmployeeCardDocumentHandler))
	mux.HandleFunc("DELETE /api/employeescard/document/category/{id}/delete", middleware.AuthMiddleware(handlers.DeleteCategoryEmployeeDocumentHandler))
	mux.HandleFunc("DELETE /api/employeescard/delete/{id}", middleware.AuthMiddleware(handlers.DeleteEmployeeCardHandler))

	mux.HandleFunc("GET /api/users/{id}", middleware.AuthMiddleware(handlers.GetUserHandler))
	mux.HandleFunc("GET /api/users", middleware.AuthMiddleware(handlers.GetAllUserHandler))
	mux.HandleFunc("POST /api/users/update/{id}", middleware.AuthMiddleware(handlers.UpdateUserHandler))
	mux.HandleFunc("POST /api/users/create", middleware.AuthMiddleware(handlers.CreateUserHandler))
	mux.HandleFunc("DELETE /api/users/delete/{id}", middleware.AuthMiddleware(handlers.DeleteUserHandler))

	mux.HandleFunc("DELETE /api/materials/delete/{id}", middleware.AuthMiddleware(handlers.DeleteMaterials))
	mux.HandleFunc("POST /api/materials/create", middleware.AuthMiddleware(handlers.CreateMaterial))
	mux.HandleFunc("POST /api/materials/upload/excel", middleware.AuthMiddleware(handlers.UploadMaterialsExcel))
	mux.HandleFunc("GET /api/materials", middleware.AuthMiddleware(handlers.GetAllMaterials))

	//Supplier Categoryes Handlers
	mux.HandleFunc("DELETE /api/suppliers/categoryes/{id}", middleware.AuthMiddleware(handlers.DeleteCategorySupplierHandler))
	mux.HandleFunc("POST /api/suppliers/categoryes/create", middleware.AuthMiddleware(handlers.CreateCategorySupplierHandler))
	mux.HandleFunc("GET /api/suppliers/categoryes", middleware.AuthMiddleware(handlers.GetAllCategorySupplierHandler))

	// Supplier Handlers
	mux.HandleFunc("DELETE /api/supplier/delete/{id}", middleware.AuthMiddleware(handlers.DeleteSupplierHandler))
	mux.HandleFunc("DELETE /api/supplier/manager/delete/{id}", middleware.AuthMiddleware(handlers.DeleteManagerCard))
	mux.HandleFunc("POST /api/supplier/{id}/manager/create", middleware.AuthMiddleware(handlers.CreateManagerCrad))
	mux.HandleFunc("POST /api/supplier/create", middleware.AuthMiddleware(handlers.CreateSupler))
	mux.HandleFunc("GET /api/suppliers/detail/{id}", middleware.AuthMiddleware(handlers.GetDetailSupplier))
	mux.HandleFunc("GET /api/suppliers", middleware.AuthMiddleware(handlers.GetAllSupler))

	//Order
	mux.HandleFunc("POST /api/order/create", middleware.AuthMiddleware(handlers.CreateOrderHandler))
	mux.HandleFunc("GET /api/order/{id}", middleware.AuthMiddleware(handlers.GetPaidOrderHandler))
	mux.HandleFunc("GET /api/agreements/{id}/orders", middleware.AuthMiddleware(handlers.GetAgreementOrderHandler))
	mux.HandleFunc("GET /api/agreements/orders/{id}/detail", middleware.AuthMiddleware(handlers.GetAgreementOrderDetailHandler))
	mux.HandleFunc("POST /api/agreements/orders/{id}/update", middleware.AuthMiddleware(handlers.UpdateAgreementOrder))

	mux.HandleFunc("POST /api/agreements/employee/{id}/update", middleware.AuthMiddleware(handlers.PutAgreementEmployeeCard))
	mux.HandleFunc("GET /api/agreements/{id}/employee", middleware.AuthMiddleware(handlers.GetAllEmployeeCardAgreementHandler))

	mux.HandleFunc("GET /api/agreements/{id}/timesheet", middleware.AuthMiddleware(handlers.GetAllAgreementTimeSheethandler))
	mux.HandleFunc("GET /api/agreements/{id}/timesheet/month", middleware.AuthMiddleware(handlers.GetMonthAgreementTimeSheethandler))
	mux.HandleFunc("POST /api/agreements/timesheet/update", middleware.AuthMiddleware(handlers.UpdateTimeSheetHandler))
	mux.HandleFunc("POST /api/agreements/timesheet/create", middleware.AuthMiddleware(handlers.CreateTimeSheetHandler))
	mux.HandleFunc("DELETE /api/agreements/timesheet/delete", middleware.AuthMiddleware(handlers.DeleteTimeSheetHandler))

	//Estimate
	mux.HandleFunc("POST /api/agreements/{id}/estimates/create", middleware.AuthMiddleware(handlers.CreateEstimateHandler))
	mux.HandleFunc("DELETE /api/agreements/estimates/{id}", middleware.AuthMiddleware(handlers.DeleteArgeementEstimateHandler))
	mux.HandleFunc("GET /api/agreements/{id}/estimates", middleware.AuthMiddleware(handlers.GetArgeementEstimatehandler))
	mux.HandleFunc("GET /api/estimates/{id}", middleware.AuthMiddleware(handlers.GetEstimateHandler))

	mux.HandleFunc("DELETE /api/agreements/delete/{id}", middleware.AuthMiddleware(handlers.DeleteAgreementHandler))
	mux.HandleFunc("POST /api/agreements/create", middleware.AuthMiddleware(handlers.CreateAgreementHandler))
	mux.HandleFunc("GET /api/agreements/{id}", middleware.AuthMiddleware(handlers.GetDetailAgreementHandler))
	mux.HandleFunc("GET /api/agreements", middleware.AuthMiddleware(handlers.GetAllAgreementHandler))

	// Expenses
	mux.HandleFunc("POST /api/agreements/{id}/expenses/create", middleware.AuthMiddleware(handlers.CreateExpensesHandler))
	mux.HandleFunc("GET /api/agreements/{id}/expenses", middleware.AuthMiddleware(handlers.GetAgreementExpensesHandler))
	mux.HandleFunc("DELETE /api/expenses/delete/{id}", middleware.AuthMiddleware(handlers.DeleteExpensesHandleer))

	// tools
	mux.HandleFunc("POST /api/tools/create", middleware.AuthMiddleware(handlers.CreateToolHandler))
	mux.HandleFunc("DELETE /api/tools/delete/{id}", middleware.AuthMiddleware(handlers.DeleteToolHandler))
	mux.HandleFunc("GET /api/agreements/{id}/tools/create", middleware.AuthMiddleware(handlers.GetAgreementAllToolHandler))

	// Блок диалогов
	mux.HandleFunc("POST /api/dialog/create", middleware.AuthMiddleware(handlers.CreateDialogHandler))
	mux.HandleFunc("GET /api/dialogs/{id}", middleware.AuthMiddleware(handlers.GetDetailDialogHandler))
	mux.HandleFunc("GET /api/dialogs", middleware.AuthMiddleware(handlers.GetAllDialogHandler))

	// Блок email
	mux.HandleFunc("POST /api/emails/create", middleware.AuthMiddleware(handlers.CreateEmailHandler))
	mux.HandleFunc("GET /api/emails/read/{id}", middleware.AuthMiddleware(handlers.ReadEmailhandler))
	mux.HandleFunc("GET /api/emails", middleware.AuthMiddleware(handlers.GetAllEmailsHandler))
	mux.HandleFunc("GET /api/emails/{id}", middleware.AuthMiddleware(handlers.GetDetailEmailHandler))

	// Блок Attachemt (файлы)
	mux.HandleFunc("GET /api/attachment/{id}", handlers.DownloadAttachmenthandler)

	mux.HandleFunc("POST /api/login", handlers.LoginHandler)

	return middleware.CorsMiddleware(mux)
}
