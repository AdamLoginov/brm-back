// новый код chat: сводка главной страницы, только чтение
package service

import (
	"encoding/json"
	"gorm.io/gorm"
	"math/big"
	"project/internal/models"
	"strings"
	"time"
)

type DashboardAgreement struct {
	ID   uint
	Name string
}
type DashboardRow struct {
	DashboardAgreement
	MonthID                                                 uint
	People, Approved, Pending, MissingRate                  int
	Hours                                                   float64
	Payable, ApprovedAmount, Expense, Refund, AllowanceDebt int64
}
type DashboardDay struct {
	Date                        string
	People, Marks, Absent, Sick int
	Hours                       float64
}
type Dashboard struct {
	Agreements  []DashboardAgreement
	Rows        []DashboardRow
	Days        []DashboardDay
	People      int
	Overdue     int64
	BalanceDate string
}

func dashboardSalary(db *gorm.DB, m models.TimeSheetMonth, row *DashboardRow) error {
	saved, err := loadSalaryApproval(db, m.AgreementID, m.ID)
	if err != nil {
		return err
	}
	var current []SalaryAccrualWithDebts
	approved := map[uint]SalaryAccrualWithDebts{}
	if saved != nil {
		for _, r := range saved.Accruals {
			approved[r.EmployeeCardID] = r
		}
	} else {
		base, err := salaryRowsForMonth(m)
		if err != nil {
			return err
		}
		calc, err := salaryWithAdjustments(db, m.AgreementID, m.ID, base)
		if err != nil {
			return err
		}
		current = calc.Accruals
		var snapshots []models.SalaryApproval
		if err = db.Select("snapshot").Where("agreement_id=? and time_sheet_month_id=? and active_employee_key is not null", m.AgreementID, m.ID).Find(&snapshots).Error; err != nil {
			return err
		}
		for _, s := range snapshots {
			var summary SalaryDebtSummary
			if err = json.Unmarshal([]byte(s.Snapshot), &summary); err != nil {
				return err
			}
			for _, r := range summary.Accruals {
				approved[r.EmployeeCardID] = r
			}
		}
	}
	for _, r := range current {
		if _, ok := approved[r.EmployeeCardID]; ok {
			continue
		}
		row.Pending++
		row.Payable += r.PayableKopecks
		if r.SalaryRateKopecks == 0 && parseSalaryDecimal(r.TotalHours).Sign() > 0 {
			row.MissingRate++
		}
	}
	for _, r := range approved {
		row.Approved++
		row.ApprovedAmount += r.PaymentKopecks
		row.Payable += r.PayableKopecks
	}
	return nil
}

func ReadDashboard(db *gorm.DB, start, today time.Time, agreement, user uint) (Dashboard, error) {
	end := start.AddDate(0, 1, 0)
	cutoff := end
	if today.Before(cutoff) {
		cutoff = today
	}
	result := Dashboard{Rows: []DashboardRow{}, BalanceDate: cutoff.AddDate(0, 0, -1).Format("2006-01-02")}
	dayPeople := map[string]map[uint]bool{}
	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		result.Days = append(result.Days, DashboardDay{Date: key})
		dayPeople[key] = map[uint]bool{}
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Agreement{}).Select("id, coalesce(nullif(short_name,''),name) as name").Order("name,id").Find(&result.Agreements).Error; err != nil {
			return err
		}
		people := map[uint]bool{}
		for _, a := range result.Agreements {
			if agreement != 0 && agreement != a.ID {
				continue
			}
			row := DashboardRow{DashboardAgreement: a}
			members := map[uint]bool{}
			var months []models.TimeSheetMonth
			if err := tx.Preload("Employees", func(q *gorm.DB) *gorm.DB { return q.Select("id", "surname", "name", "middle_name", "salary_rate") }).Preload("TimeSheets").Where("agreement_id=? AND year=? AND month=?", a.ID, start.Year(), int(start.Month())).Find(&months).Error; err != nil {
				return err
			}
			for _, m := range months {
				row.MonthID = m.ID
				if err := dashboardSalary(tx, m, &row); err != nil {
					return err
				}
				for _, mark := range m.TimeSheets {
					date, err := time.Parse("2006-01-02", mark.Date)
					set, ok := dayPeople[mark.Date]
					if err != nil || !ok || strings.TrimSpace(mark.Status) == "" {
						continue
					}
					day := &result.Days[date.Day()-1]
					day.Marks++
					switch mark.Status {
					case "П":
						day.Absent++
					case "Б":
						day.Sick++
					}
					h := parseSalaryDecimal(mark.Status)
					if h.Sign() == 0 || h.Cmp(big.NewRat(24, 1)) > 0 {
						continue
					}
					hours, _ := h.Float64()
					row.Hours += hours
					day.Hours += hours
					people[mark.EmployeeCardID] = true
					members[mark.EmployeeCardID] = true
					set[mark.EmployeeCardID] = true
				}
			}
			row.People = len(members)
			if err := tx.Model(&models.Expenses{}).Select("coalesce(sum(cast(round(price*100) as integer)),0) as expense, coalesce(sum(case when account='employee' and coalesce(is_reimbursed,0)=0 then cast(round(price*100) as integer) else 0 end),0) as refund").Where("agreement_id=? and substr(coalesce(nullif(expense_date,''),created_at),1,7)=?", a.ID, start.Format("2006-01")).Row().Scan(&row.Expense, &row.Refund); err != nil {
				return err
			}
			if err := tx.Raw(`select coalesce(sum(-balance),0) from (select sum(amount) balance from (
 select employee_card_id, -amount_kopecks amount from daily_allowance_accruals where agreement_id=? and deleted_at is null and date<?
 union all select employee_card_id, amount_kopecks from daily_allowance_payments where agreement_id=? and deleted_at is null and date<?)
 group by employee_card_id having sum(amount)<0)`, a.ID, cutoff.Format("2006-01-02"), a.ID, cutoff.Format("2006-01-02")).Row().Scan(&row.AllowanceDebt); err != nil {
				return err
			}
			result.Rows = append(result.Rows, row)
		}
		result.People = len(people)
		for i := range result.Days {
			result.Days[i].People = len(dayPeople[result.Days[i].Date])
		}
		return tx.Model(&models.Task{}).Where("status=false AND due_date<>'' AND due_date<? AND (exists(select 1 from task_assignees ta where ta.task_id=tasks.id and ta.user_id=?) or (to_user_id=? and not exists(select 1 from task_assignees ta where ta.task_id=tasks.id)))", today.Format("2006-01-02"), user, user).Count(&result.Overdue).Error
	})
	return result, err
}
