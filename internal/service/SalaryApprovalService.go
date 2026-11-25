// --------- новый код chat: атомарное согласование, снимок расчёта и отмена
package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"project/internal/models"
	"strings"
	"sync"
	"time"
)

var ErrSalaryApprovalInput = errors.New("Проверьте дату и неотрицательные суммы выплаты до копеек")
var ErrSalaryApprovalConflict = errors.New("Расчёт изменился или уже согласован. Обновите страницу")
var salaryApprovalMutex sync.Mutex

type SalaryApprovalInput struct {
	Date       string `json:"date"`
	RequestKey string `json:"request_key"`
	Revision   string `json:"revision"`
	Payments   []struct {
		EmployeeCardID uint   `json:"employee_card_id"`
		Amount         string `json:"amount"`
	} `json:"payments"`
}
type salaryApprovalChange struct {
	Table    string     `json:"table"`
	ID       uint       `json:"id"`
	Column   string     `json:"column"`
	Before   any        `json:"before"`
	After    any        `json:"after"`
	ClosedAt *time.Time `json:"closed_at"`
	Created  bool       `json:"created"`
}

func salaryHash(value any) string {
	data, _ := json.Marshal(value)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func loadSalaryApproval(db *gorm.DB, agreementID, monthID uint) (*SalaryDebtSummary, error) {
	var approval models.SalaryApproval
	err := db.Where("agreement_id = ? AND active_month_id = ?", agreementID, monthID).First(&approval).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var result SalaryDebtSummary
	if err = json.Unmarshal([]byte(approval.Snapshot), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Получение и пересчёт всегда возвращают снимок, если табель согласован.
func GetSalaryCalculation(db *gorm.DB, agreementID, monthID uint, calculate bool) (SalaryDebtSummary, error) {
	salaryApprovalMutex.Lock()
	defer salaryApprovalMutex.Unlock()
	var result SalaryDebtSummary
	err := db.Transaction(func(tx *gorm.DB) error {
		var month models.TimeSheetMonth
		if err := findSalaryTimeSheet(tx, agreementID, monthID, &month); err != nil {
			return err
		}
		// --------- новый код chat: индивидуальные выплаты и согласования
		var err error
		result, err = employeeSalaryCalculation(tx, agreementID, monthID, calculate)
		if err != nil {
			return err
		}
		// --------- новый код chat: актуальная отметка профиля для сохранённых и новых начислений
		ids := make([]uint, 0, len(result.Accruals))
		for _, row := range result.Accruals {
			ids = append(ids, row.EmployeeCardID)
		}
		if len(ids) == 0 {
			return nil
		}
		var employees []models.EmployeeCard
		if err := tx.Unscoped().Select("id", "has_alimony").Where("id IN ?", ids).Find(&employees).Error; err != nil {
			return err
		}
		alimonyByEmployee := make(map[uint]bool, len(employees))
		for _, employee := range employees {
			alimonyByEmployee[employee.ID] = employee.HasAlimony
		}
		for i := range result.Accruals {
			result.Accruals[i].HasAlimony = alimonyByEmployee[result.Accruals[i].EmployeeCardID]
		}
		return nil
		// --------- конец нового кода chat
	})
	return result, err
}

func calculateSalaryApproval(db *gorm.DB, agreementID, monthID uint, calculate bool) (SalaryDebtSummary, error) {
	var rows []models.SalaryAccrual
	var err error
	var result SalaryDebtSummary
	if calculate {
		rows, err = CalculateSalaryAccruals(db, agreementID, monthID)
	} else {
		rows, err = GetSalaryAccruals(db, agreementID, monthID)
	}
	if err != nil {
		return result, err
	}
	a, err := AddSalaryAdvances(db, agreementID, rows)
	if err != nil {
		return result, err
	}
	p, err := AddSalaryPenalties(db, agreementID, monthID, a)
	if err != nil {
		return result, err
	}
	d, err := AddSalaryDeductions(db, p)
	if err != nil {
		return result, err
	}
	e, err := AddSalaryExpenses(db, agreementID, d)
	if err != nil {
		return result, err
	}
	b, err := AddSalaryBonuses(db, agreementID, monthID, e)
	if err != nil {
		return result, err
	}
	result, err = AddSalaryDebts(db, agreementID, b)
	if err != nil {
		return result, err
	}
	if err = fillSalaryDetails(db, agreementID, monthID, &result); err != nil {
		return result, err
	}
	// UpdatedAt начисления меняется при пересчёте, поэтому не включается в отпечаток.
	canonical := result
	canonical.Accruals = append([]SalaryAccrualWithDebts{}, result.Accruals...)
	for i := range canonical.Accruals {
		canonical.Accruals[i].Model = gorm.Model{}
	}
	result.Revision = salaryHash(canonical)
	return result, nil
}

func fillSalaryDetails(db *gorm.DB, agreementID, monthID uint, result *SalaryDebtSummary) error {
	var month models.TimeSheetMonth
	if err := db.First(&month, monthID).Error; err != nil {
		return err
	}
	result.CalendarMonth, result.CalendarYear = month.Month, month.Year // новый код chat: календарь остаётся в месяце согласования
	ids := []uint{}
	index := map[uint]int{}
	for i, row := range result.Accruals {
		ids = append(ids, row.EmployeeCardID)
		index[row.EmployeeCardID] = i
	}
	if len(ids) == 0 {
		return nil
	}
	query := func() *gorm.DB {
		return db.Where("agreement_id = ? AND employee_card_id IN ?", agreementID, ids).Order("id")
	}
	var detail SalaryApprovalDetail
	if err := db.Where("time_sheet_month_id = ? AND employee_card_id IN ?", monthID, ids).Order("id").Find(&detail.TimeSheets).Error; err != nil { // новый код chat: месяц уже проверен на принадлежность договору
		return err
	}
	if err := query().Where("is_settled = ?", false).Find(&detail.Advances).Error; err != nil {
		return err
	}
	if err := query().Where("date LIKE ?", fmt.Sprintf("%04d-%02d-%%", month.Year, month.Month)).Find(&detail.Penalties).Error; err != nil {
		return err
	}
	if err := query().Where("account = ? AND is_reimbursed = ?", "employee", false).Find(&detail.Expenses).Error; err != nil {
		return err
	}
	if err := query().Where("month = ? AND year = ?", month.Month, month.Year).Find(&detail.Bonuses).Error; err != nil {
		return err
	}
	if err := query().Where("is_closed = ?", false).Find(&detail.Debts).Error; err != nil {
		return err
	}
	for _, v := range detail.TimeSheets {
		i := index[v.EmployeeCardID]
		result.Accruals[i].Detail.TimeSheets = append(result.Accruals[i].Detail.TimeSheets, v)
	}
	for _, v := range detail.Advances {
		i := index[v.EmployeeCardID]
		result.Accruals[i].Detail.Advances = append(result.Accruals[i].Detail.Advances, v)
	}
	for _, v := range detail.Penalties {
		i := index[v.EmployeeCardID]
		result.Accruals[i].Detail.Penalties = append(result.Accruals[i].Detail.Penalties, v)
	}
	for _, v := range detail.Expenses {
		i := index[v.EmployeeCardId]
		result.Accruals[i].Detail.Expenses = append(result.Accruals[i].Detail.Expenses, v)
	}
	for _, v := range detail.Bonuses {
		i := index[v.EmployeeCardID]
		result.Accruals[i].Detail.Bonuses = append(result.Accruals[i].Detail.Bonuses, v)
	}
	for _, v := range detail.Debts {
		i := index[v.EmployeeCardID]
		result.Accruals[i].Detail.Debts = append(result.Accruals[i].Detail.Debts, v)
	}
	return nil
}

func ApproveSalary(db *gorm.DB, agreementID, monthID uint, input SalaryApprovalInput) (SalaryDebtSummary, error) {
	salaryApprovalMutex.Lock()
	defer salaryApprovalMutex.Unlock()
	return approveSalaryEmployee(db, agreementID, monthID, input, 0)
}

// --------- новый код chat: индивидуальные выплаты и согласования
func approveSalaryEmployee(db *gorm.DB, agreementID, monthID uint, input SalaryApprovalInput, employeeID uint) (SalaryDebtSummary, error) {
	var result SalaryDebtSummary
	date, err := time.Parse("2006-01-02", input.Date)
	if err != nil || date.Year() < 1 || len(input.RequestKey) < 8 || len(input.RequestKey) > 100 || input.Revision == "" {
		return result, ErrSalaryApprovalInput
	}
	hash := salaryHash(struct {
		AgreementID, MonthID uint
		Input                SalaryApprovalInput
	}{agreementID, monthID, input})
	err = db.Transaction(func(tx *gorm.DB) error {
		var previous models.SalaryApproval
		lookup := tx.Where("request_key = ?", input.RequestKey).First(&previous).Error
		if lookup == nil {
			if previous.RequestHash != hash || (previous.ActiveMonthID == nil && previous.ActiveEmployeeKey == nil) {
				return ErrSalaryApprovalConflict
			}
			return json.Unmarshal([]byte(previous.Snapshot), &result)
		}
		if !errors.Is(lookup, gorm.ErrRecordNotFound) {
			return lookup
		}
		stored, err := loadSalaryApproval(tx, agreementID, monthID)
		if err != nil {
			return err
		}
		if stored != nil {
			return ErrSalaryApprovalConflict
		}
		// новый код chat: старое общее API не должно повторно закрывать индивидуальные согласования
		var activeCount int64
		activeQuery := tx.Model(&models.SalaryApproval{}).Where("agreement_id = ? AND time_sheet_month_id = ? AND active_employee_key IS NOT NULL", agreementID, monthID)
		if employeeID != 0 {
			activeQuery = activeQuery.Where("employee_card_id = ?", employeeID)
		}
		if err := activeQuery.Count(&activeCount).Error; err != nil {
			return err
		}
		if activeCount > 0 {
			return ErrSalaryApprovalConflict
		}
		result, err = calculateSalaryApproval(tx, agreementID, monthID, true)
		if err != nil {
			return err
		}
		// --------- новый код chat: индивидуальные выплаты и согласования
		if employeeID != 0 {
			filtered := []SalaryAccrualWithDebts{}
			for _, row := range result.Accruals {
				if row.EmployeeCardID == employeeID {
					row.Revision = salaryRowRevision(row)
					filtered = append(filtered, row)
				}
			}
			result.Accruals = filtered
			if len(filtered) != 1 {
				return ErrSalaryApprovalConflict
			}
			result.Revision = filtered[0].Revision
		}
		if result.Revision != input.Revision {
			return ErrSalaryApprovalConflict
		}
		if len(result.Accruals) == 0 || len(input.Payments) != len(result.Accruals) {
			return ErrSalaryApprovalInput
		}
		payments := map[uint]int64{}
		for _, v := range input.Payments {
			normalized := strings.ReplaceAll(strings.TrimSpace(v.Amount), ",", ".")
			if !salaryDecimalPattern.MatchString(normalized) {
				return ErrSalaryApprovalInput
			}
			amount := salaryRateKopecks(normalized)
			if amount == 0 && parseSalaryDecimal(normalized).Sign() != 0 {
				return ErrSalaryApprovalInput
			}
			if _, exists := payments[v.EmployeeCardID]; exists {
				return ErrSalaryApprovalInput
			}
			payments[v.EmployeeCardID] = amount
		}
		approval := models.SalaryApproval{AgreementID: agreementID, TimeSheetMonthID: monthID, ActiveMonthID: &monthID, Date: input.Date, RequestKey: input.RequestKey, RequestHash: hash}
		// --------- новый код chat: индивидуальные выплаты и согласования
		if employeeID != 0 {
			key := fmt.Sprintf("%d/%d/%d", agreementID, monthID, employeeID)
			approval.ActiveMonthID = nil
			approval.EmployeeCardID = employeeID
			approval.ActiveEmployeeKey = &key
		}
		if err := tx.Create(&approval).Error; err != nil {
			return err
		}
		changes := []salaryApprovalChange{}
		change := func(table string, id uint, column string, before, after any, closedAt *time.Time) error {
			values := map[string]any{column: after, "updated_at": time.Now()}
			if table == "salary_penalties" {
				values["closed_at"] = date
			}
			if err := tx.Table(table).Where("id = ? AND agreement_id = ? AND deleted_at IS NULL", id, agreementID).Updates(values).Error; err != nil {
				return err
			}
			changes = append(changes, salaryApprovalChange{Table: table, ID: id, Column: column, Before: before, After: after, ClosedAt: closedAt})
			return nil
		}
		var month models.TimeSheetMonth
		if err := tx.First(&month, monthID).Error; err != nil {
			return err
		}
		period := fmt.Sprintf("%02d.%04d", month.Month, month.Year)
		result.TotalPaymentKopecks = 0
		for i := range result.Accruals {
			row := &result.Accruals[i]
			amount, ok := payments[row.EmployeeCardID]
			if !ok {
				return ErrSalaryApprovalInput
			}
			if amount > maxSalaryKopecks-result.TotalPaymentKopecks {
				return ErrSalaryAmountOverflow
			}
			result.TotalPaymentKopecks += amount
			row.PaymentKopecks = amount
			// --------- новый код chat: индивидуальные выплаты и согласования
			row.Approved, row.ApprovalID, row.ApprovalDate = true, approval.ID, input.Date
			row.Revision = salaryRowRevision(*row) // новый код chat: стабильная версия снимка
			for _, v := range row.Detail.Advances {
				if err := change("salary_advances", v.ID, "is_settled", false, true, nil); err != nil {
					return err
				}
			}
			for _, v := range row.Detail.Penalties {
				if v.Status == "active" {
					if err := change("salary_penalties", v.ID, "status", v.Status, "closed", v.ClosedAt); err != nil {
						return err
					}
				}
			}
			for _, v := range row.Detail.Expenses {
				if err := change("expenses", v.ID, "is_reimbursed", false, true, nil); err != nil {
					return err
				}
			}
			for _, v := range row.Detail.Bonuses {
				if !v.IsPaid {
					if err := change("salary_bonus", v.ID, "is_paid", false, true, nil); err != nil {
						return err
					}
				}
			}
			for _, v := range row.Detail.Debts {
				if err := change("salary_debts", v.ID, "is_closed", false, true, nil); err != nil {
					return err
				}
			}
			base := row.PayableKopecks
			if base < 0 {
				row.NewEmployeeDebtKopecks = -base
				base = 0
			}
			if amount < base {
				row.NewCompanyDebtKopecks = base - amount
			}
			if amount > base {
				row.ExtraBonusKopecks = amount - base
				bonus := models.SalaryBonus{AgreementID: agreementID, EmployeeCardID: row.EmployeeCardID, EmployeeName: row.EmployeeName, Month: month.Month, Year: month.Year, AmountKopecks: row.ExtraBonusKopecks, IsPaid: true, Comment: "Доплата при согласовании за " + period}
				if err := tx.Create(&bonus).Error; err != nil {
					return err
				}
				changes = append(changes, salaryApprovalChange{Table: "salary_bonus", ID: bonus.ID, Created: true})
			}
			for side, debtAmount := range map[string]int64{"company": row.NewCompanyDebtKopecks, "employee": row.NewEmployeeDebtKopecks} {
				if debtAmount == 0 {
					continue
				}
				debt := models.SalaryDebt{AgreementID: agreementID, EmployeeCardID: row.EmployeeCardID, EmployeeName: row.EmployeeName, Date: input.Date, AmountKopecks: debtAmount, Side: side, Comment: "Долг по согласованию за " + period, SourceApprovalID: &approval.ID}
				if err := tx.Create(&debt).Error; err != nil {
					return err
				}
				changes = append(changes, salaryApprovalChange{Table: "salary_debts", ID: debt.ID, Created: true})
			}
		}
		result.Approved = true
		result.ApprovalID = approval.ID
		result.ApprovalDate = input.Date
		snapshot, err := json.Marshal(result)
		if err != nil {
			return err
		}
		journal, err := json.Marshal(changes)
		if err != nil {
			return err
		}
		return tx.Model(&approval).Updates(map[string]any{"snapshot": string(snapshot), "changes": string(journal)}).Error
	})
	return result, err
}

func CancelSalaryApproval(db *gorm.DB, agreementID, monthID, approvalID uint) error {
	salaryApprovalMutex.Lock()
	defer salaryApprovalMutex.Unlock()
	return cancelSalaryEmployee(db, agreementID, monthID, approvalID)
}

// --------- новый код chat: индивидуальные выплаты и согласования
func cancelSalaryEmployee(db *gorm.DB, agreementID, monthID, approvalID uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var approval models.SalaryApproval
		if err := tx.Where("agreement_id = ? AND time_sheet_month_id = ?", agreementID, monthID).First(&approval, approvalID).Error; err != nil {
			return err
		}
		if approval.ActiveMonthID == nil && approval.ActiveEmployeeKey == nil {
			return nil
		}
		var changes []salaryApprovalChange
		if err := json.Unmarshal([]byte(approval.Changes), &changes); err != nil {
			return err
		}
		for _, v := range changes {
			allowed := map[string]string{"salary_advances": "is_settled", "salary_penalties": "status", "expenses": "is_reimbursed", "salary_bonus": "is_paid", "salary_debts": "is_closed"}
			column, ok := allowed[v.Table]
			if !ok {
				return ErrSalaryApprovalInput
			}
			query := tx.Table(v.Table).Where("id = ? AND agreement_id = ? AND deleted_at IS NULL", v.ID, agreementID)
			if v.Created {
				// Закрытый долг остаётся: он уже мог быть использован следующим согласованием.
				if v.Table == "salary_debts" {
					query = query.Where("is_closed = ?", false)
				}
				if err := query.Updates(map[string]any{"deleted_at": time.Now(), "updated_at": time.Now()}).Error; err != nil {
					return err
				}
			} else {
				if column != v.Column {
					return ErrSalaryApprovalInput
				}
				values := map[string]any{column: v.Before, "updated_at": time.Now()}
				if v.Table == "salary_penalties" {
					values["closed_at"] = v.ClosedAt
				}
				if err := query.Where(column+" = ?", v.After).Updates(values).Error; err != nil {
					return err
				}
			}
		}
		return tx.Model(&approval).Updates(map[string]any{"active_month_id": nil, "active_employee_key": nil}).Error // --------- новый код chat: индивидуальные выплаты и согласования
	})
}

// --------- конец нового кода chat
