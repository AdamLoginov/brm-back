// --------- новый код chat: сохранение черновика при отмене согласования
package service

import (
	"encoding/json"
	"gorm.io/gorm"
	"project/internal/models"
	"time"
)

// Служебное обновление даты при закрытии и открытии удержаний не меняет расчёт.
func salaryDraftSourceRevision(row SalaryAccrualWithDebts) string {
	row.Detail.Alimonies = append([]models.SalaryAlimony(nil), row.Detail.Alimonies...)
	row.Detail.Advances = append([]models.SalaryAdvance(nil), row.Detail.Advances...)
	row.Detail.Penalties = append([]models.SalaryPenalty(nil), row.Detail.Penalties...)
	row.Detail.Expenses = append([]models.Expenses(nil), row.Detail.Expenses...)
	row.Detail.Bonuses = append([]models.SalaryBonus(nil), row.Detail.Bonuses...)
	row.Detail.Debts = append([]models.SalaryDebt(nil), row.Detail.Debts...)
	for i := range row.Detail.Alimonies {
		row.Detail.Alimonies[i].UpdatedAt = time.Time{}
	}
	for i := range row.Detail.Advances {
		row.Detail.Advances[i].UpdatedAt = time.Time{}
	}
	for i := range row.Detail.Penalties {
		row.Detail.Penalties[i].UpdatedAt = time.Time{}
	}
	for i := range row.Detail.Expenses {
		row.Detail.Expenses[i].UpdatedAt = time.Time{}
	}
	for i := range row.Detail.Bonuses {
		row.Detail.Bonuses[i].UpdatedAt = time.Time{}
	}
	for i := range row.Detail.Debts {
		row.Detail.Debts[i].UpdatedAt = time.Time{}
	}
	return salaryRowRevision(row)
}

func restoreSalaryDraftRevision(tx *gorm.DB, approval models.SalaryApproval) error {
	var drafts []models.SalaryPaymentDraft
	query := tx.Where("agreement_id = ? AND time_sheet_month_id = ?", approval.AgreementID, approval.TimeSheetMonthID)
	if approval.EmployeeCardID != 0 {
		query = query.Where("employee_card_id = ?", approval.EmployeeCardID)
	}
	if err := query.Find(&drafts).Error; err != nil {
		return err
	}
	if len(drafts) == 0 {
		return nil
	}
	var snapshot SalaryDebtSummary
	if err := json.Unmarshal([]byte(approval.Snapshot), &snapshot); err != nil {
		return err
	}
	current, err := calculateSalaryApproval(tx, approval.AgreementID, approval.TimeSheetMonthID, true)
	if err != nil {
		return err
	}
	for _, original := range snapshot.Accruals {
		original.Approved, original.ApprovalID, original.ApprovalDate = false, 0, ""
		original.ExtraBonusKopecks, original.NewCompanyDebtKopecks, original.NewEmployeeDebtKopecks = 0, 0, 0
		for _, draft := range drafts {
			if draft.EmployeeCardID != original.EmployeeCardID || draft.Revision != salaryRowRevision(original) {
				continue
			}
			for _, row := range current.Accruals {
				if row.EmployeeCardID == original.EmployeeCardID && salaryDraftSourceRevision(row) == salaryDraftSourceRevision(original) {
					if err := tx.Model(&draft).Update("revision", salaryRowRevision(row)).Error; err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// --------- конец нового кода chat
