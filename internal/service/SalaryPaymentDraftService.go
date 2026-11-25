// --------- новый код chat: индивидуальные выплаты и согласования
package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"project/internal/models"
	"strings"
	"time"
)

func salaryRowRevision(row SalaryAccrualWithDebts) string {
	row.Model = gorm.Model{}
	row.Revision = ""
	row.HasDraft, row.DraftReset = false, false
	row.PaymentKopecks = 0
	return salaryHash(row)
}

func salaryPaymentAmount(text string) (int64, error) {
	text = strings.ReplaceAll(strings.TrimSpace(text), ",", ".")
	if !salaryDecimalPattern.MatchString(text) {
		return 0, ErrSalaryApprovalInput
	}
	amount := salaryRateKopecks(text)
	if amount == 0 && parseSalaryDecimal(text).Sign() != 0 {
		return 0, ErrSalaryApprovalInput
	}
	return amount, nil
}

// --------- новый код chat: индивидуальные выплаты и согласования
func employeeSalaryCalculation(db *gorm.DB, agreementID, monthID uint, calculate bool) (SalaryDebtSummary, error) {
	legacy, err := loadSalaryApproval(db, agreementID, monthID)
	if err != nil {
		return SalaryDebtSummary{}, err
	}
	if legacy != nil {
		for i := range legacy.Accruals {
			r := &legacy.Accruals[i]
			r.Approved = true
			r.ApprovalID = legacy.ApprovalID
			r.ApprovalDate = legacy.ApprovalDate
			if r.Revision == "" {
				r.Revision = salaryRowRevision(*r)
			}
		}
		return *legacy, nil
	}
	result, err := calculateSalaryApproval(db, agreementID, monthID, calculate)
	if err != nil {
		return result, err
	}
	var approvals []models.SalaryApproval
	if err = db.Where("agreement_id = ? AND time_sheet_month_id = ? AND active_employee_key IS NOT NULL", agreementID, monthID).Order("id").Find(&approvals).Error; err != nil {
		return result, err
	}
	stored := map[uint]SalaryAccrualWithDebts{}
	for _, approval := range approvals {
		var snapshot SalaryDebtSummary
		if err = json.Unmarshal([]byte(approval.Snapshot), &snapshot); err != nil {
			return result, err
		}
		for _, row := range snapshot.Accruals {
			row.Approved = true
			row.ApprovalID = approval.ID
			row.ApprovalDate = approval.Date
			stored[row.EmployeeCardID] = row
		}
	}
	for i, row := range result.Accruals {
		if saved, ok := stored[row.EmployeeCardID]; ok {
			result.Accruals[i] = saved
			delete(stored, row.EmployeeCardID)
		}
	}
	// --------- новый код chat: индивидуальные выплаты и согласования
	for _, approval := range approvals {
		if row, ok := stored[approval.EmployeeCardID]; ok {
			result.Accruals = append(result.Accruals, row)
			delete(stored, approval.EmployeeCardID)
		}
	}
	var drafts []models.SalaryPaymentDraft
	if err = db.Where("agreement_id = ? AND time_sheet_month_id = ?", agreementID, monthID).Find(&drafts).Error; err != nil {
		return result, err
	}
	draftMap := map[uint]models.SalaryPaymentDraft{}
	for _, d := range drafts {
		draftMap[d.EmployeeCardID] = d
	}
	for i := range result.Accruals {
		r := &result.Accruals[i]
		if r.Approved {
			continue
		}
		r.Revision = salaryRowRevision(*r)
		r.PaymentKopecks = r.PayableKopecks
		if r.PaymentKopecks < 0 {
			r.PaymentKopecks = 0
		}
		if d, ok := draftMap[r.EmployeeCardID]; ok {
			if d.Revision == r.Revision {
				r.PaymentKopecks = d.AmountKopecks
				r.HasDraft = true
			} else {
				if err = db.Unscoped().Delete(&d).Error; err != nil {
					return result, err
				}
				r.DraftReset = true
			}
		}
	}
	if err = recountEmployeeSalary(&result); err != nil {
		return result, err
	}
	return result, nil
}

func recountEmployeeSalary(s *SalaryDebtSummary) error {
	s.TotalKopecks = 0
	s.TotalAdvancesKopecks = 0
	s.TotalPenaltiesKopecks = 0
	s.TotalDeductionKopecks = 0
	s.TotalAlimonyKopecks = 0 // новый код chat
	s.TotalExpensesKopecks = 0
	s.TotalBonusesKopecks = 0
	s.TotalCompanyDebtKopecks = 0
	s.TotalEmployeeDebtKopecks = 0
	s.TotalPayableKopecks = 0
	s.TotalPaymentKopecks = 0
	s.Approved = len(s.Accruals) > 0
	s.ApprovalID = 0
	s.ApprovalDate = ""
	for _, r := range s.Accruals {
		pairs := []struct {
			total *int64
			value int64
		}{
			{&s.TotalAlimonyKopecks, r.AlimonyKopecks}, // новый код chat
			{&s.TotalKopecks, r.AmountKopecks}, {&s.TotalAdvancesKopecks, r.AdvancesKopecks}, {&s.TotalPenaltiesKopecks, r.PenaltiesKopecks},
			{&s.TotalDeductionKopecks, r.DeductionKopecks}, {&s.TotalExpensesKopecks, r.ExpensesKopecks}, {&s.TotalBonusesKopecks, r.BonusesKopecks},
			{&s.TotalCompanyDebtKopecks, r.CompanyDebtKopecks}, {&s.TotalEmployeeDebtKopecks, r.EmployeeDebtKopecks}, {&s.TotalPayableKopecks, r.PayableKopecks}, {&s.TotalPaymentKopecks, r.PaymentKopecks},
		}
		for _, p := range pairs {
			if p.value > 0 && *p.total > maxSalaryKopecks-p.value || p.value < 0 && *p.total < -maxSalaryKopecks-p.value {
				return ErrSalaryAmountOverflow
			}
			*p.total += p.value
		}
		s.Approved = s.Approved && r.Approved
	}
	return nil
}

type SalaryPaymentDraftInput struct {
	EmployeeCardID uint    `json:"employee_card_id"`
	Revision       string  `json:"revision"`
	Amount         string  `json:"amount"`
	Reset          bool    `json:"reset"`
	AlimonyAmount  *string `json:"alimony_amount"` // новый код chat: отсутствие поля сохраняет прежнее значение
}

func SaveSalaryPaymentDraft(db *gorm.DB, agreementID, monthID uint, input SalaryPaymentDraftInput) (SalaryDebtSummary, error) {
	salaryApprovalMutex.Lock()
	defer salaryApprovalMutex.Unlock()
	var result SalaryDebtSummary
	err := db.Transaction(func(tx *gorm.DB) error {
		current, err := employeeSalaryCalculation(tx, agreementID, monthID, true)
		if err != nil {
			return err
		}
		for _, r := range current.Accruals {
			if r.EmployeeCardID != input.EmployeeCardID {
				continue
			}
			if r.Approved || r.Revision != input.Revision || input.Revision == "" {
				return ErrSalaryApprovalConflict
			}
			query := tx.Unscoped().Where("agreement_id = ? AND time_sheet_month_id = ? AND employee_card_id = ?", agreementID, monthID, input.EmployeeCardID)
			// --------- новый код chat: обе суммы сохраняются в одной транзакции
			if !input.Reset && input.AlimonyAmount != nil {
				if err = saveSalaryAlimony(tx, r, *input.AlimonyAmount); err != nil {
					return err
				}
				updated, e := employeeSalaryCalculation(tx, agreementID, monthID, false)
				if e != nil {
					return e
				}
				for _, item := range updated.Accruals {
					if item.EmployeeCardID == r.EmployeeCardID {
						r = item
						break
					}
				}
			}
			// --------- конец нового кода chat
			if input.Reset {
				err = query.Delete(&models.SalaryPaymentDraft{}).Error
			} else {
				amount, e := salaryPaymentAmount(input.Amount)
				if e != nil {
					return e
				}
				draft := models.SalaryPaymentDraft{AgreementID: agreementID, TimeSheetMonthID: monthID, EmployeeCardID: input.EmployeeCardID, AmountKopecks: amount, Revision: r.Revision}
				err = tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "agreement_id"}, {Name: "time_sheet_month_id"}, {Name: "employee_card_id"}}, DoUpdates: clause.AssignmentColumns([]string{"amount_kopecks", "revision", "updated_at", "deleted_at"})}).Create(&draft).Error
			}
			if err != nil {
				return err
			}
			result, err = employeeSalaryCalculation(tx, agreementID, monthID, false)
			return err
		}
		return ErrSalaryApprovalConflict
	})
	return result, err
}

// --------- новый код chat: индивидуальные выплаты и согласования
func splitLegacySalaryApproval(tx *gorm.DB, agreementID, monthID uint) error {
	var old models.SalaryApproval
	err := tx.Where("agreement_id = ? AND active_month_id = ?", agreementID, monthID).First(&old).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	var snapshot SalaryDebtSummary
	var changes []salaryApprovalChange
	if err = json.Unmarshal([]byte(old.Snapshot), &snapshot); err != nil {
		return err
	}
	if err = json.Unmarshal([]byte(old.Changes), &changes); err != nil {
		return err
	}
	journals := map[uint][]salaryApprovalChange{}
	for _, c := range changes {
		allowed := map[string]bool{"salary_advances": true, "salary_penalties": true, "expenses": true, "salary_bonus": true, "salary_debts": true}
		if !allowed[c.Table] {
			return ErrSalaryApprovalInput
		}
		var owner struct{ EmployeeCardID uint }
		if err = tx.Table(c.Table).Where("id = ? AND agreement_id = ?", c.ID, agreementID).Select("employee_card_id").Take(&owner).Error; err != nil {
			return err
		}
		journals[owner.EmployeeCardID] = append(journals[owner.EmployeeCardID], c)
	}
	for _, row := range snapshot.Accruals {
		key := fmt.Sprintf("%d/%d/%d", agreementID, monthID, row.EmployeeCardID)
		a := models.SalaryApproval{AgreementID: agreementID, TimeSheetMonthID: monthID, EmployeeCardID: row.EmployeeCardID, ActiveEmployeeKey: &key, Date: old.Date, RequestKey: fmt.Sprintf("legacy-%d-%d", old.ID, row.EmployeeCardID)}
		if err = tx.Create(&a).Error; err != nil {
			return err
		}
		row.Approved = true
		row.ApprovalID = a.ID
		row.ApprovalDate = a.Date
		part := snapshot
		part.Accruals = []SalaryAccrualWithDebts{row}
		data, e := json.Marshal(part)
		if e != nil {
			return e
		}
		journal, e := json.Marshal(journals[row.EmployeeCardID])
		if e != nil {
			return e
		}
		if err = tx.Model(&a).Updates(map[string]any{"snapshot": string(data), "changes": string(journal)}).Error; err != nil {
			return err
		}
		if err = tx.Model(&models.SalaryDebt{}).Where("source_approval_id = ? AND employee_card_id = ?", old.ID, row.EmployeeCardID).Update("source_approval_id", a.ID).Error; err != nil {
			return err
		}
	}
	return tx.Model(&old).Update("active_month_id", nil).Error
}

type SalaryEmployeeSelection struct {
	EmployeeCardID uint   `json:"employee_card_id"`
	Approved       bool   `json:"approved"`
	ApprovalID     uint   `json:"approval_id"`
	Revision       string `json:"revision"`
	Amount         string `json:"amount"`
}
type SalaryEmployeeApprovalInput struct {
	Date       string                    `json:"date"`
	RequestKey string                    `json:"request_key"`
	Employees  []SalaryEmployeeSelection `json:"employees"`
}

func ApplySalaryEmployeeApprovals(db *gorm.DB, agreementID, monthID uint, input SalaryEmployeeApprovalInput) (SalaryDebtSummary, error) {
	salaryApprovalMutex.Lock()
	defer salaryApprovalMutex.Unlock()
	var result SalaryDebtSummary
	if len(input.Employees) == 0 || len(input.RequestKey) < 8 || len(input.RequestKey) > 80 {
		return result, ErrSalaryApprovalInput
	}
	requestHash := salaryHash(struct {
		AgreementID, MonthID uint
		Input                SalaryEmployeeApprovalInput
	}{agreementID, monthID, input})
	err := db.Transaction(func(tx *gorm.DB) error {
		// новый код chat: повтор после потери ответа возвращает тот же результат
		var receipt models.SalaryApprovalBatch
		lookup := tx.Where("request_key = ?", input.RequestKey).First(&receipt).Error
		if lookup == nil {
			if receipt.RequestHash != requestHash {
				return ErrSalaryApprovalConflict
			}
			return json.Unmarshal([]byte(receipt.Snapshot), &result)
		}
		if !errors.Is(lookup, gorm.ErrRecordNotFound) {
			return lookup
		}
		// --------- новый код chat: индивидуальные выплаты и согласования
		current, err := employeeSalaryCalculation(tx, agreementID, monthID, true)
		if err != nil {
			return err
		}
		rows := map[uint]SalaryAccrualWithDebts{}
		seen := map[uint]bool{}
		for _, r := range current.Accruals {
			rows[r.EmployeeCardID] = r
		}
		for _, v := range input.Employees {
			r, ok := rows[v.EmployeeCardID]
			if !ok || seen[v.EmployeeCardID] {
				return ErrSalaryApprovalInput
			}
			seen[v.EmployeeCardID] = true
			if r.Approved {
				if r.ApprovalID != v.ApprovalID {
					return ErrSalaryApprovalConflict
				}
			} else if r.Revision != v.Revision || v.Revision == "" {
				return ErrSalaryApprovalConflict
			}
			if v.Approved && !r.Approved {
				if _, err = salaryPaymentAmount(v.Amount); err != nil {
					return err
				}
				date, e := time.Parse("2006-01-02", input.Date)
				if e != nil || date.Year() < 1 {
					return ErrSalaryApprovalInput
				}
			}
		}
		if err = splitLegacySalaryApproval(tx, agreementID, monthID); err != nil {
			return err
		}
		current, err = employeeSalaryCalculation(tx, agreementID, monthID, false)
		if err != nil {
			return err
		}
		for _, r := range current.Accruals {
			rows[r.EmployeeCardID] = r
		}
		for _, v := range input.Employees {
			r := rows[v.EmployeeCardID]
			if r.Approved == v.Approved {
				continue
			}
			if !v.Approved {
				err = cancelSalaryEmployee(tx, agreementID, monthID, r.ApprovalID)
			} else {
				var approval SalaryApprovalInput
				approval.Date = input.Date
				approval.RequestKey = fmt.Sprintf("%s-%d", input.RequestKey, v.EmployeeCardID)
				approval.Revision = r.Revision
				approval.Payments = append(approval.Payments, struct {
					EmployeeCardID uint   `json:"employee_card_id"`
					Amount         string `json:"amount"`
				}{v.EmployeeCardID, v.Amount})
				_, err = approveSalaryEmployee(tx, agreementID, monthID, approval, v.EmployeeCardID)
			}
			if err != nil {
				return err
			}
			if err = tx.Unscoped().Where("agreement_id = ? AND time_sheet_month_id = ? AND employee_card_id = ?", agreementID, monthID, v.EmployeeCardID).Delete(&models.SalaryPaymentDraft{}).Error; err != nil {
				return err
			}
		}
		result, err = employeeSalaryCalculation(tx, agreementID, monthID, true)
		if err != nil {
			return err
		}
		snapshot, err := json.Marshal(result)
		if err != nil {
			return err
		}
		return tx.Create(&models.SalaryApprovalBatch{RequestKey: input.RequestKey, RequestHash: requestHash, Snapshot: string(snapshot)}).Error
	})
	return result, err
}

// --------- новый код chat: индивидуальные выплаты и согласования
