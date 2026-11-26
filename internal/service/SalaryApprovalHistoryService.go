// --------- новый код chat: действующие согласования выбранного табеля
package service

import (
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"project/internal/models"
	"sort"
	"time"
)

type SalaryApprovalHistoryEmployee struct {
	EmployeeCardID uint   `json:"employee_card_id"`
	EmployeeName   string `json:"employee_name"`
	ApprovalID     uint   `json:"approval_id"`
	PaymentKopecks int64  `json:"payment_kopecks"`
}
type SalaryApprovalHistoryGroup struct {
	Key          string                          `json:"key"`
	AuthorUserID uint                            `json:"author_user_id"`
	AuthorName   string                          `json:"author_name"`
	CreatedAt    time.Time                       `json:"created_at"`
	Date         string                          `json:"date"`
	TotalKopecks int64                           `json:"total_kopecks"`
	Employees    []SalaryApprovalHistoryEmployee `json:"employees"`
}

func GetSalaryApprovalHistory(db *gorm.DB, agreementID, monthID uint) ([]SalaryApprovalHistoryGroup, error) {
	salaryApprovalMutex.Lock()
	defer salaryApprovalMutex.Unlock()
	result := []SalaryApprovalHistoryGroup{}
	err := db.Transaction(func(tx *gorm.DB) error {
		var agreement models.Agreement
		if err := tx.First(&agreement, agreementID).Error; err != nil {
			return err
		}
		var month models.TimeSheetMonth
		if err := tx.Where("agreement_id = ?", agreementID).First(&month, monthID).Error; err != nil {
			return err
		}
		var approvals []models.SalaryApproval
		if err := tx.Where("agreement_id = ? AND time_sheet_month_id = ? AND (active_employee_key IS NOT NULL OR active_month_id IS NOT NULL)", agreementID, monthID).Order("created_at DESC, id DESC").Find(&approvals).Error; err != nil {
			return err
		}
		indexes := map[string]int{}
		for _, approval := range approvals {
			key := approval.GroupKey
			if key == "" {
				key = fmt.Sprintf("legacy-%d", approval.ID)
			}
			index, exists := indexes[key]
			if !exists {
				index = len(result)
				indexes[key] = index
				name := approval.AuthorName
				if name == "" {
					name = "Не указан"
				}
				result = append(result, SalaryApprovalHistoryGroup{Key: key, AuthorUserID: approval.AuthorUserID, AuthorName: name, CreatedAt: approval.CreatedAt, Date: approval.Date, Employees: []SalaryApprovalHistoryEmployee{}})
			}
			group := &result[index]
			if approval.CreatedAt.Before(group.CreatedAt) {
				group.CreatedAt = approval.CreatedAt
			}
			var snapshot SalaryDebtSummary
			if err := json.Unmarshal([]byte(approval.Snapshot), &snapshot); err != nil {
				return err
			}
			for _, row := range snapshot.Accruals {
				if approval.EmployeeCardID != 0 && row.EmployeeCardID != approval.EmployeeCardID {
					continue
				}
				if row.PaymentKopecks < 0 || row.PaymentKopecks > maxSalaryKopecks-group.TotalKopecks {
					return ErrSalaryAmountOverflow
				}
				group.TotalKopecks += row.PaymentKopecks
				group.Employees = append(group.Employees, SalaryApprovalHistoryEmployee{EmployeeCardID: row.EmployeeCardID, EmployeeName: row.EmployeeName, ApprovalID: approval.ID, PaymentKopecks: row.PaymentKopecks})
			}
		}
		for i := range result {
			sort.SliceStable(result[i].Employees, func(a, b int) bool { return result[i].Employees[a].EmployeeName < result[i].Employees[b].EmployeeName })
		}
		sort.SliceStable(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
		return nil
	})
	return result, err
}

// --------- конец нового кода chat
