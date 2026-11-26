package app

import (
	"net/http"
	"project/internal/handlers"
	"project/internal/middleware"
)

func NewRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/agreements/{id}/work-orders/books/{bookId}/blocks/{blockId}/statement", middleware.AuthMiddleware(handlers.WorkOrderPositionExcelHandler))       // новый код chat
	mux.HandleFunc("GET /api/agreements/{id}/work-orders/books/{bookId}/statements/{statementMode}", middleware.AuthMiddleware(handlers.WorkOrderPositionExcelHandler))       // новый код chat: все ведомости одной книгой
	mux.HandleFunc("GET /api/agreements/{id}/work-orders/books/{bookId}/positions/{positionId}/statement", middleware.AuthMiddleware(handlers.WorkOrderPositionExcelHandler)) // новый код chat: ведомость по шаблону
	mux.HandleFunc("PATCH /api/agreements/{id}/work-orders/tasks/{taskId}", middleware.AuthMiddleware(handlers.WorkOrderHandler))                                             // новый код chat
	mux.HandleFunc("GET /api/agreements/{id}/work-orders/books/{bookId}/positions/excel", middleware.AuthMiddleware(handlers.WorkOrderPositionExcelHandler))                  // новый код chat: сводка всех позиций
	// --------- новый код chat: Excel доступен с той же авторизацией, что и книга
	mux.HandleFunc("POST /api/agreements/{id}/work-orders/books/{bookId}/excel/{excelAction}", middleware.AuthMiddleware(handlers.WorkOrderExcelHandler))
	// --------- конец нового кода chat
	// --------- новый код chat: книга нарядов, склад и справочники договора
	for _, pattern := range []string{"GET /api/agreements/{id}/work-references", "POST /api/agreements/{id}/work-references/{kind}/create", "PATCH /api/agreements/{id}/work-references/{kind}/{referenceId}", "DELETE /api/agreements/{id}/work-references/{kind}/{referenceId}"} {
		mux.HandleFunc(pattern, middleware.AuthMiddleware(handlers.WorkOrderReferenceHandler))
	}
	for _, pattern := range []string{"GET /api/agreements/{id}/work-orders", "GET /api/agreements/{id}/work-orders/books/{bookId}", "POST /api/agreements/{id}/work-orders/books/create"} {
		mux.HandleFunc(pattern, middleware.AuthMiddleware(handlers.WorkOrderHandler))
	}
	for _, pattern := range []string{"POST /api/agreements/{id}/work-orders/shifts/create", "PATCH /api/agreements/{id}/work-orders/shifts/{shiftId}", "DELETE /api/agreements/{id}/work-orders/shifts/{shiftId}"} {
		mux.HandleFunc(pattern, middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
			r.SetPathValue("action", "shifts")
			handlers.WorkOrderHandler(w, r)
		}))
	}
	for _, pattern := range []string{"GET /api/agreements/{id}/warehouse", "POST /api/agreements/{id}/warehouse/create", "GET /api/agreements/{id}/warehouse/{materialId}/history", "PATCH /api/agreements/{id}/warehouse/{materialId}", "DELETE /api/agreements/{id}/warehouse/{materialId}"} {
		mux.HandleFunc(pattern, middleware.AuthMiddleware(handlers.WarehouseHandler))
	}
	for _, pattern := range []string{"POST /api/agreements/{id}/warehouse/{materialId}/receipts/create", "PATCH /api/agreements/{id}/warehouse/{materialId}/receipts/{receiptId}", "DELETE /api/agreements/{id}/warehouse/{materialId}/receipts/{receiptId}"} {
		mux.HandleFunc(pattern, middleware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
			r.SetPathValue("action", "receipts")
			handlers.WarehouseHandler(w, r)
		}))
	}
	// --------- конец нового кода chat
	// --------- новый код chat: контакты доступны только авторизованным пользователям
	mux.HandleFunc("GET /api/agreements/{id}/contacts", middleware.AuthMiddleware(handlers.AgreementContactHandler))
	mux.HandleFunc("POST /api/agreements/{id}/contacts/create", middleware.AuthMiddleware(handlers.AgreementContactHandler)) // новый код chat: без конфликта с изменением договора
	mux.HandleFunc("PATCH /api/agreements/{id}/contacts/{contactId}", middleware.AuthMiddleware(handlers.AgreementContactHandler))
	mux.HandleFunc("DELETE /api/agreements/{id}/contacts/{contactId}", middleware.AuthMiddleware(handlers.AgreementContactHandler))
	// --------- конец нового кода chat

	// --------- новый код chat: управление справочником для авторизованных пользователей
	mux.HandleFunc("GET /api/employee-specializations", middleware.AuthMiddleware(handlers.ListEmployeeSpecializations))
	mux.HandleFunc("POST /api/employee-specializations", middleware.AuthMiddleware(handlers.SaveEmployeeSpecialization))
	mux.HandleFunc("POST /api/employee-specializations/{id}", middleware.AuthMiddleware(handlers.SaveEmployeeSpecialization))
	mux.HandleFunc("DELETE /api/employee-specializations/{id}", middleware.AuthMiddleware(handlers.DeleteEmployeeSpecialization))
	// --------- конец нового кода chat
	mux.HandleFunc("GET /api/employeecards", middleware.AuthMiddleware(handlers.GetAllEmployeeCardHandler))
	mux.HandleFunc("GET /api/employeescard/{id}/detail", middleware.AuthMiddleware(handlers.GetEmployeeCardDetailHandler))
	mux.HandleFunc("GET /api/employeescard/{id}/statistics", middleware.AuthMiddleware(handlers.EmployeeStatisticsHandler)) // новый код chat: статистика за год
	mux.HandleFunc("GET /api/employeescard/document/{id}/detail", middleware.AuthMiddleware(handlers.GetEmployeeCardDocumentDetailHandler))
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

	mux.HandleFunc("GET /api/agreements/{id}/timesheetmonth", middleware.AuthMiddleware(handlers.GetAllTimeSheetMonthHandler))
	mux.HandleFunc("GET /api/agreements/{id}/timesheetmonth/{monthId}/employees", middleware.AuthMiddleware(handlers.TimeSheetEmployeeListHandler)) // новый код chat
	mux.HandleFunc("PUT /api/agreements/{id}/timesheetmonth/{monthId}/employees", middleware.AuthMiddleware(handlers.TimeSheetEmployeeListHandler)) // новый код chat
	mux.HandleFunc("GET /api/agreements/timesheetmonth/detail/{id}", middleware.AuthMiddleware(handlers.GetDetailTimeSheetMonthHandler))
	mux.HandleFunc("POST /api/agreements/timesheetmonth/update", middleware.AuthMiddleware(handlers.UpdateTimeSheetMonthHandler))
	mux.HandleFunc("POST /api/agreements/timesheetmonth/create", middleware.AuthMiddleware(handlers.CreateTimeSheetMonthHandler))
	mux.HandleFunc("DELETE /api/agreements/timesheetmonth/delete/{id}", middleware.AuthMiddleware(handlers.DeleteTimeSheetMonthHandler))

	mux.HandleFunc("GET /api/agreements/{id}/payment", middleware.AuthMiddleware(handlers.GetAllPaymentHandler))
	// --------- новый код chat: суточные и история выдач внутри договора
	mux.HandleFunc("GET /api/agreements/{id}/daily-allowance", middleware.AuthMiddleware(handlers.DailyAllowanceTableHandler))
	mux.HandleFunc("PUT /api/agreements/{id}/daily-allowance/list", middleware.AuthMiddleware(handlers.DailyAllowanceListHandler)) // новый код chat
	mux.HandleFunc("GET /api/agreements/{id}/daily-allowance/payments", middleware.AuthMiddleware(handlers.DailyAllowancePaymentHandler))
	mux.HandleFunc("POST /api/agreements/{id}/daily-allowance/payments", middleware.AuthMiddleware(handlers.DailyAllowancePaymentHandler))
	mux.HandleFunc("PUT /api/agreements/{id}/daily-allowance/payments/{paymentId}", middleware.AuthMiddleware(handlers.DailyAllowancePaymentHandler))
	mux.HandleFunc("DELETE /api/agreements/{id}/daily-allowance/payments/{paymentId}", middleware.AuthMiddleware(handlers.DailyAllowancePaymentHandler))
	mux.HandleFunc("DELETE /api/agreements/{id}/daily-allowance/payments/by-date/{date}", middleware.AuthMiddleware(handlers.DailyAllowancePaymentDateHandler)) // новый код chat: все выдачи за дату
	// --------- конец нового кода chat
	// --------- новый код chat: просмотр и пересчёт начислений выбранного табеля
	mux.HandleFunc("GET /api/agreements/{id}/timesheetmonth/{monthId}/accruals", middleware.AuthMiddleware(handlers.GetSalaryAccrualHandler))
	mux.HandleFunc("POST /api/agreements/{id}/timesheetmonth/{monthId}/accruals/calculate", middleware.AuthMiddleware(handlers.CalculateSalaryAccrualHandler))
	// --------- новый код chat: маршруты авансов договора
	mux.HandleFunc("GET /api/agreements/{id}/advances", middleware.AuthMiddleware(handlers.SalaryAdvanceHandler))
	// --------- новый код chat: алименты сотрудника, договор задаёт только область просмотра
	mux.HandleFunc("GET /api/agreements/{id}/alimony", middleware.AuthMiddleware(handlers.SalaryAlimonyHandler))
	mux.HandleFunc("GET /api/agreements/{id}/alimony/employees", middleware.AuthMiddleware(handlers.SalaryAlimonyEmployeesHandler))
	mux.HandleFunc("POST /api/agreements/{id}/alimony/create", middleware.AuthMiddleware(handlers.SalaryAlimonyHandler))
	mux.HandleFunc("POST /api/agreements/{id}/alimony/{alimonyId}/update", middleware.AuthMiddleware(handlers.SalaryAlimonyHandler))
	mux.HandleFunc("DELETE /api/agreements/{id}/alimony/{alimonyId}", middleware.AuthMiddleware(handlers.SalaryAlimonyHandler))
	// --------- конец нового кода chat
	mux.HandleFunc("POST /api/agreements/{id}/advances/create", middleware.AuthMiddleware(handlers.SalaryAdvanceHandler))
	mux.HandleFunc("POST /api/agreements/{id}/advances/{advanceId}/update", middleware.AuthMiddleware(handlers.SalaryAdvanceHandler))
	mux.HandleFunc("DELETE /api/agreements/{id}/advances/{advanceId}", middleware.AuthMiddleware(handlers.SalaryAdvanceHandler))
	// --------- новый код chat: API штрафов
	mux.HandleFunc("GET /api/agreements/{id}/penalties", middleware.AuthMiddleware(handlers.SalaryPenaltyHandler))
	mux.HandleFunc("POST /api/agreements/{id}/penalties/create", middleware.AuthMiddleware(handlers.SalaryPenaltyHandler))
	mux.HandleFunc("POST /api/agreements/{id}/penalties/{penaltyId}/update", middleware.AuthMiddleware(handlers.SalaryPenaltyHandler))
	mux.HandleFunc("DELETE /api/agreements/{id}/penalties/{penaltyId}", middleware.AuthMiddleware(handlers.SalaryPenaltyHandler))
	mux.HandleFunc("PATCH /api/agreements/{id}/penalties/{penaltyId}/status", middleware.AuthMiddleware(handlers.SalaryPenaltyHandler)) // новый код chat: простить или восстановить штраф
	// --------- конец нового кода chat
	// --------- конец нового кода chat
	// --------- конец нового кода chat
	mux.HandleFunc("POST /api/agreements/payments/create", middleware.AuthMiddleware(handlers.CreatePaymentHandler))

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
	mux.HandleFunc("GET /api/expense-categories", middleware.AuthMiddleware(handlers.ListExpenseCategories))         // новый код chat
	mux.HandleFunc("POST /api/expense-categories", middleware.AuthMiddleware(handlers.SaveExpenseCategory))          // новый код chat
	mux.HandleFunc("PATCH /api/expense-categories/{id}", middleware.AuthMiddleware(handlers.SaveExpenseCategory))    // новый код chat
	mux.HandleFunc("DELETE /api/expense-categories/{id}", middleware.AuthMiddleware(handlers.DeleteExpenseCategory)) // новый код chat
	// --------- новый код chat: долги и согласование начислений
	mux.HandleFunc("GET /api/agreements/{id}/debts", middleware.AuthMiddleware(handlers.SalaryDebtHandler))
	mux.HandleFunc("POST /api/agreements/{id}/debts/create", middleware.AuthMiddleware(handlers.SalaryDebtHandler))
	mux.HandleFunc("POST /api/agreements/{id}/debts/{debtId}/update", middleware.AuthMiddleware(handlers.SalaryDebtHandler))
	mux.HandleFunc("DELETE /api/agreements/{id}/debts/{debtId}", middleware.AuthMiddleware(handlers.SalaryDebtHandler))
	mux.HandleFunc("PATCH /api/agreements/{id}/debts/{debtId}/status", middleware.AuthMiddleware(handlers.SalaryDebtHandler))
	// --------- новый код chat: индивидуальные выплаты и согласования
	mux.HandleFunc("POST /api/agreements/{id}/timesheetmonth/{monthId}/payment-draft", middleware.AuthMiddleware(handlers.SalaryPaymentDraftHandler))
	mux.HandleFunc("GET /api/agreements/{id}/timesheetmonth/{monthId}/approval-history", middleware.AuthMiddleware(handlers.SalaryApprovalHistoryHandler)) // новый код chat: история согласований
	mux.HandleFunc("POST /api/agreements/{id}/timesheetmonth/{monthId}/employee-approvals", middleware.AuthMiddleware(handlers.SalaryEmployeeApprovalHandler))
	mux.HandleFunc("POST /api/agreements/{id}/timesheetmonth/{monthId}/approval", middleware.AuthMiddleware(handlers.SalaryApprovalHandler))
	mux.HandleFunc("DELETE /api/agreements/{id}/timesheetmonth/{monthId}/approval", middleware.AuthMiddleware(handlers.SalaryApprovalHandler))
	// --------- конец нового кода chat
	// --------- новый код chat: премии договора
	mux.HandleFunc("GET /api/agreements/{id}/bonuses", middleware.AuthMiddleware(handlers.SalaryBonusHandler))
	mux.HandleFunc("POST /api/agreements/{id}/bonuses/create", middleware.AuthMiddleware(handlers.SalaryBonusHandler))
	mux.HandleFunc("POST /api/agreements/{id}/bonuses/{bonusId}/update", middleware.AuthMiddleware(handlers.SalaryBonusHandler))
	mux.HandleFunc("DELETE /api/agreements/{id}/bonuses/{bonusId}", middleware.AuthMiddleware(handlers.SalaryBonusHandler))
	mux.HandleFunc("PATCH /api/agreements/{id}/bonuses/{bonusId}/status", middleware.AuthMiddleware(handlers.SalaryBonusHandler))
	// --------- конец нового кода chat
	mux.HandleFunc("PATCH /api/agreements/{id}/expenses/{expenseId}/status", middleware.AuthMiddleware(handlers.UpdateExpenseReimbursementHandler)) // новый код chat: статус возмещения
	mux.HandleFunc("POST /api/agreements/{id}/expenses/create", middleware.AuthMiddleware(handlers.CreateExpensesHandler))
	mux.HandleFunc("PATCH /api/agreements/{id}/expenses/{expenseId}", middleware.AuthMiddleware(handlers.UpdateExpensesHandler)) // новый код chat: редактирование расхода
	mux.HandleFunc("POST /api/agreements/update/{id}", middleware.AuthMiddleware(handlers.UpdateAgreementHandler))
	mux.HandleFunc("GET /api/agreements/{id}/expenses", middleware.AuthMiddleware(handlers.GetAgreementExpensesHandler))
	mux.HandleFunc("DELETE /api/expenses/delete/{id}", middleware.AuthMiddleware(handlers.DeleteExpensesHandleer))

	// tools
	mux.HandleFunc("POST /api/tools/create", middleware.AuthMiddleware(handlers.CreateToolHandler))
	mux.HandleFunc("GET /api/tools", middleware.AuthMiddleware(handlers.GetAgreementAllToolHandler)) // новый код chat: общий список МТР
	// --------- новый код chat: общий и договорный учёт СИЗ
	mux.HandleFunc("GET /api/ppe", middleware.AuthMiddleware(handlers.ListPpe))
	mux.HandleFunc("GET /api/agreements/{id}/ppe", middleware.AuthMiddleware(handlers.ListPpe))
	mux.HandleFunc("POST /api/ppe", middleware.AuthMiddleware(handlers.SavePpe))
	mux.HandleFunc("PUT /api/ppe/{id}", middleware.AuthMiddleware(handlers.SavePpe))
	mux.HandleFunc("DELETE /api/ppe/{id}", middleware.AuthMiddleware(handlers.DeletePpe))
	// --------- конец нового кода chat
	// --------- новый код chat: расходники и общий справочник категорий
	mux.HandleFunc("GET /api/consumables", middleware.AuthMiddleware(handlers.ListConsumables))
	mux.HandleFunc("GET /api/agreements/{id}/consumables", middleware.AuthMiddleware(handlers.ListConsumables))
	mux.HandleFunc("POST /api/consumables", middleware.AuthMiddleware(handlers.SaveConsumable))
	mux.HandleFunc("PUT /api/consumables/{id}", middleware.AuthMiddleware(handlers.SaveConsumable))
	mux.HandleFunc("DELETE /api/consumables/{id}", middleware.AuthMiddleware(handlers.DeleteConsumable))
	mux.HandleFunc("GET /api/consumable-categories", middleware.AuthMiddleware(handlers.ListConsumableCategories))
	mux.HandleFunc("POST /api/consumable-categories", middleware.AuthMiddleware(handlers.SaveConsumableCategory))
	mux.HandleFunc("PATCH /api/consumable-categories/{id}", middleware.AuthMiddleware(handlers.SaveConsumableCategory))
	mux.HandleFunc("DELETE /api/consumable-categories/{id}", middleware.AuthMiddleware(handlers.DeleteConsumableCategory))
	// --------- конец нового кода chat
	mux.HandleFunc("PUT /api/tools/{id}", middleware.AuthMiddleware(handlers.UpdateToolHandler)) // новый код chat: редактирование инструмента
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

	// Блок Task
	// --------- новый код chat: личная лента и явное прочтение
	mux.HandleFunc("GET /api/notifications", middleware.AuthMiddleware(handlers.GetNotificationsHandler))
	mux.HandleFunc("POST /api/notifications/read-all", middleware.AuthMiddleware(handlers.ReadAllNotificationsHandler))
	mux.HandleFunc("POST /api/notifications/{id}/read", middleware.AuthMiddleware(handlers.ReadNotificationHandler))
	// --------- конец нового кода chat
	mux.HandleFunc("GET /api/task", middleware.AuthMiddleware(handlers.GetAllTaskHandler))
	mux.HandleFunc("POST /api/task/success/{id}", middleware.AuthMiddleware(handlers.UpdateStatusTaskHandler))
	mux.HandleFunc("POST /api/task/create", middleware.AuthMiddleware(handlers.CreateTaskHandler))
	mux.HandleFunc("POST /api/task/update", middleware.AuthMiddleware(handlers.UpdateTaskHandler))
	mux.HandleFunc("DELETE /api/task/delete/{id}", middleware.AuthMiddleware(handlers.DeleteTaskHandler))

	// Блок Attachemt (файлы)
	mux.HandleFunc("GET /api/attachment/{id}", middleware.AuthMiddleware(handlers.DownloadAttachmenthandler))

	// --------- новый код chat: изолированные маршруты чтения файлов
	mux.HandleFunc("POST /api/files/session", handlers.FileSessionHandler)
	mux.HandleFunc("POST /api/files/logout", handlers.FileLogoutHandler)
	mux.HandleFunc("GET /api/files/documents/{id}", middleware.FileAuthMiddleware(handlers.GetEmployeeCardDocumentDetailHandler))
	mux.HandleFunc("GET /api/files/attachments/{id}", middleware.FileAuthMiddleware(handlers.DownloadAttachmenthandler))
	// --------- конец нового кода chat
	mux.HandleFunc("POST /api/login", handlers.LoginHandler)
	mux.HandleFunc("POST /api/session/refresh", middleware.AuthMiddleware(handlers.RefreshSessionHandler)) // новый код chat: активность продлевает сессию

	return middleware.CorsMiddleware(mux)
}
