// --------- новый код chat: суточные внутри договора, точные денежные суммы
package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"project/internal/models"
	"sort"
	"strings"
	"time"
)

var ErrDailyAllowanceInput = errors.New("Некорректные данные суточных")

func dailyError(message string) error { return fmt.Errorf("%w: %s", ErrDailyAllowanceInput, message) }
func DailyAllowanceToday() time.Time  { return dailyDate(time.Now()) }
func dailyDate(now time.Time) time.Time {
	local := now.In(time.FixedZone("Europe/Moscow", 3*60*60))
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
}
func dailyEmployeeName(employee models.EmployeeCard) string {
	parts := []string{}
	if employee.Surname != nil {
		parts = append(parts, *employee.Surname)
	}
	parts = append(parts, employee.Name)
	if employee.MiddleName != nil {
		parts = append(parts, *employee.MiddleName)
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}
func dailyAdd(a, b int64) (int64, error) {
	if b < 0 || a < 0 || b > maxSalaryKopecks-a {
		return 0, ErrSalaryAmountOverflow
	}
	return a + b, nil
}

// Используется в транзакции сохранения табеля и при первом открытии старых табелей.
func SyncDailyAllowanceAccruals(tx *gorm.DB, monthID uint) error {
	var month models.TimeSheetMonth
	if err := tx.Preload("Employees").Preload("TimeSheets").First(&month, monthID).Error; err != nil {
		return err
	}
	if month.DailyAllowanceKopecks < 0 || month.DailyAllowanceKopecks > maxSalaryKopecks {
		return ErrSalaryAmountOverflow
	}
	members := map[uint]string{}
	dismissals := map[uint]models.EmployeeCard{} // новый код chat: исключение дней увольнения
	for _, employee := range month.Employees {
		dismissals[employee.ID] = employee // новый код chat
		members[employee.ID] = dailyEmployeeName(employee)
	}
	// Последняя запись дня определяет отметку; исторические дубли не начисляют день дважды.
	entries := map[string]models.TimeSheet{}
	for _, entry := range month.TimeSheets {
		key := fmt.Sprintf("%d:%s", entry.EmployeeCardID, entry.Date)
		if previous, ok := entries[key]; !ok || entry.ID > previous.ID {
			entries[key] = entry
		}
	}
	keep := []uint{}
	for _, entry := range entries {
		if dailyDismissedOn(dismissals[entry.EmployeeCardID], entry.Date) {
			continue
		} // новый код chat
		name, member := members[entry.EmployeeCardID]
		if !member || parseSalaryDecimal(entry.Status).Sign() <= 0 || month.DailyAllowanceKopecks == 0 {
			continue
		}
		if _, err := time.Parse("2006-01-02", entry.Date); err != nil {
			return dailyError("Некорректная дата в табеле")
		}
		row := models.DailyAllowanceAccrual{AgreementID: month.AgreementID, TimeSheetMonthID: month.ID, EmployeeCardID: entry.EmployeeCardID, EmployeeName: name, Date: entry.Date, AmountKopecks: month.DailyAllowanceKopecks}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "time_sheet_month_id"}, {Name: "employee_card_id"}, {Name: "date"}}, DoUpdates: clause.Assignments(map[string]any{"agreement_id": month.AgreementID, "employee_name": name, "amount_kopecks": row.AmountKopecks, "deleted_at": nil, "updated_at": time.Now()})}).Create(&row).Error; err != nil {
			return err
		}
		// SQLite RETURNING при upsert возвращает ID существующего начисления.
		if err := tx.Where("time_sheet_month_id = ? AND employee_card_id = ? AND date = ?", month.ID, entry.EmployeeCardID, entry.Date).First(&row).Error; err != nil {
			return err
		}
		keep = append(keep, row.ID)
	}
	removed := tx.Where("time_sheet_month_id = ?", monthID)
	if len(keep) > 0 {
		removed = removed.Where("id NOT IN ?", keep)
	}
	return removed.Delete(&models.DailyAllowanceAccrual{}).Error
}

func DeleteDailyAllowanceAccruals(tx *gorm.DB, monthID uint) error {
	return tx.Where("time_sheet_month_id = ?", monthID).Delete(&models.DailyAllowanceAccrual{}).Error
}

// новый код chat: дата увольнения включительно не оплачивается
func dailyDismissedOn(employee models.EmployeeCard, date string) bool {
	return employee.Status && employee.DismissalDate != "" && date >= employee.DismissalDate
}

type DailyAllowanceCell struct {
	Date                   string `json:"date"`
	Mark                   string `json:"mark"`
	State                  string `json:"state"`
	AmountKopecks          int64  `json:"amount_kopecks"`
	CoveredKopecks         int64  `json:"covered_kopecks"`
	PreviewRequiredKopecks int64  `json:"preview_required_kopecks"` // новый код chat: сколько дополнительно выдать для полного покрытия дня
}
type DailyAllowanceEmployee struct {
	// --------- новый код chat: увольнение для подсветки сотрудника в месяце табеля
	Status        bool   `json:"status"`
	DismissalDate string `json:"dismissal_date"`
	// --------- конец нового кода chat
	IsITR            bool                 `json:"is_itr"` // новый код chat: подсветка ФИО в табеле суточных
	EmployeeCardID   uint                 `json:"employee_card_id"`
	EmployeeName     string               `json:"employee_name"`
	IsAssigned       bool                 `json:"is_assigned"`
	BalanceKopecks   int64                `json:"balance_kopecks"`
	SuggestedKopecks int64                `json:"suggested_kopecks"`
	Cells            []DailyAllowanceCell `json:"cells"`
}
type DailyAllowanceTable struct {
	MonthID         uint                     `json:"month_id"`
	Year            uint                     `json:"year"`
	Month           uint                     `json:"month"`
	RateKopecks     int64                    `json:"rate_kopecks"`
	Today           string                   `json:"today"`
	Through         string                   `json:"through"`
	ForecastWarning string                   `json:"forecast_warning"`
	Employees       []DailyAllowanceEmployee `json:"employees"`
}

// Баланс — только фактические дни по вчера; будущие дни не создают задолженность.
func GetDailyAllowanceTable(db *gorm.DB, agreementID, monthID uint, through string, today time.Time) (DailyAllowanceTable, error) {
	result := DailyAllowanceTable{Employees: []DailyAllowanceEmployee{}, Today: today.Format("2006-01-02"), Through: through}
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&models.Agreement{}, agreementID).Error; err != nil {
			return err
		}
		var months []models.TimeSheetMonth
		if err := tx.Where("agreement_id = ?", agreementID).Order("year, month, id").Preload("TimeSheets").Preload("Employees").Find(&months).Error; err != nil { // новый код chat: отметки для покрытия остатком
			return err
		}
		var selected models.TimeSheetMonth
		if err := tx.Where("agreement_id = ?", agreementID).Preload("Employees").Preload("TimeSheets").First(&selected, monthID).Error; err != nil {
			return err
		}
		if selected.Month < 1 || selected.Month > 12 || selected.Year < 1 || selected.Year > 9999 {
			return dailyError("Проверьте месяц и год табеля")
		}
		for _, month := range months {
			if err := SyncDailyAllowanceAccruals(tx, month.ID); err != nil {
				return err
			}
		}
		result.MonthID = selected.ID
		result.Year = selected.Year
		result.Month = selected.Month
		result.RateKopecks = selected.DailyAllowanceKopecks
		var accruals []models.DailyAllowanceAccrual
		if err := tx.Where("agreement_id = ? AND date < ?", agreementID, result.Today).Order("date, id").Find(&accruals).Error; err != nil {
			return err
		}
		var payments []models.DailyAllowancePayment
		if err := tx.Where("agreement_id = ?", agreementID).Find(&payments).Error; err != nil {
			return err
		}
		people := map[uint]string{}
		dismissalCards := map[uint]models.EmployeeCard{} // новый код chat: данные увольнения из карточки
		itrMembers := map[uint]bool{}                    // новый код chat: признак из карточки сотрудника
		tableMembers := map[uint]bool{}                  // новый код chat: показываем только сотрудников выбранного табеля
		for _, employee := range selected.Employees {
			dismissalCards[employee.ID] = employee // новый код chat
			people[employee.ID] = dailyEmployeeName(employee)
			tableMembers[employee.ID] = true         // новый код chat
			itrMembers[employee.ID] = employee.IsITR // новый код chat
		}
		var current []models.EmployeeCard
		if err := tx.Where("agreement_id = ?", agreementID).Find(&current).Error; err != nil {
			return err
		}
		assigned := map[uint]bool{}
		for _, employee := range current {
			people[employee.ID] = dailyEmployeeName(employee)
			assigned[employee.ID] = true
		}
		paid := map[uint]int64{}
		earned := map[uint]int64{}
		earnedThrough := map[uint]int64{} // новый код chat: фактические начисления только по выбранную дату
		for _, payment := range payments {
			if _, ok := people[payment.EmployeeCardID]; !ok {
				people[payment.EmployeeCardID] = payment.EmployeeName
			}
			sum, err := dailyAdd(paid[payment.EmployeeCardID], payment.AmountKopecks)
			if err != nil {
				return err
			}
			paid[payment.EmployeeCardID] = sum
		}
		actual := map[string]DailyAllowanceCell{}
		lastWorked := map[uint]string{} // новый код chat: последний фактически отработанный день
		remaining := map[uint]int64{}
		for id, value := range paid {
			remaining[id] = value
		}
		for _, accrual := range accruals {
			if _, ok := people[accrual.EmployeeCardID]; !ok {
				people[accrual.EmployeeCardID] = accrual.EmployeeName
			}
			key := fmt.Sprintf("%d:%s", accrual.EmployeeCardID, accrual.Date)
			if _, exists := actual[key]; exists {
				return dailyError("Один день сотрудника встречается в нескольких табелях. Устраните дубли табелей")
			}
			sum, err := dailyAdd(earned[accrual.EmployeeCardID], accrual.AmountKopecks)
			if err != nil {
				return err
			}
			earned[accrual.EmployeeCardID] = sum
			// --------- новый код chat: более поздние дни не увеличивают выдачу за прошлый период
			if through != "" && accrual.Date <= through {
				value, err := dailyAdd(earnedThrough[accrual.EmployeeCardID], accrual.AmountKopecks)
				if err != nil {
					return err
				}
				earnedThrough[accrual.EmployeeCardID] = value
			}
			// --------- конец нового кода chat
			covered := remaining[accrual.EmployeeCardID]
			if covered > accrual.AmountKopecks {
				covered = accrual.AmountKopecks
			}
			remaining[accrual.EmployeeCardID] -= covered
			cell := DailyAllowanceCell{Date: accrual.Date, State: "unpaid", AmountKopecks: accrual.AmountKopecks, CoveredKopecks: covered}
			if required := sum - paid[accrual.EmployeeCardID]; required > 0 {
				cell.PreviewRequiredKopecks = required
			} // новый код chat: сначала покрываем старый долг
			if covered == accrual.AmountKopecks {
				cell.State = "paid"
				cell.Mark = "С"
			}
			actual[key] = cell
			lastWorked[accrual.EmployeeCardID] = accrual.Date // новый код chat: начисления отсортированы по дате
		}
		monthEnd := time.Date(int(selected.Year), time.Month(selected.Month)+1, 0, 0, 0, 0, 0, time.UTC)
		forecastEnd := monthEnd
		if through != "" {
			end, err := time.Parse("2006-01-02", through)
			if err != nil || end.Year() < 1 || end.Year() > 9999 { // новый код chat: разрешены прошлые даты
				return dailyError("Укажите корректную дату обеспечения")
			}
			forecastEnd = end
			if monthEnd.After(forecastEnd) {
				forecastEnd = monthEnd
			}
		}
		rates := map[string]int64{}
		ambiguous := map[string]bool{}
		for _, month := range months {
			key := fmt.Sprintf("%04d-%02d", month.Year, month.Month)
			if value, ok := rates[key]; ok && value != month.DailyAllowanceKopecks {
				ambiguous[key] = true
			}
			rates[key] = month.DailyAllowanceKopecks
		}
		// Выбранный табель определяет ставку своего месяца и кратность новой выдачи.
		selectedKey := fmt.Sprintf("%04d-%02d", selected.Year, selected.Month)
		rates[selectedKey] = selected.DailyAllowanceKopecks
		ambiguous[selectedKey] = false
		// новый код chat: расчёт диапазона выполняется отдельно для каждого сотрудника ниже.
		if forecastEnd.Sub(today) > 366*24*time.Hour*10 {
			return dailyError("Выберите дату обеспечения в пределах 10 лет")
		}
		marks := map[string]models.TimeSheet{}
		for _, entry := range selected.TimeSheets {
			key := fmt.Sprintf("%d:%s", entry.EmployeeCardID, entry.Date)
			if previous, ok := marks[key]; !ok || entry.ID > previous.ID {
				marks[key] = entry
			}
		}
		// --------- новый код chat: начало прогноза по рабочим отметкам именно выбранного табеля
		selectedLastWorked := map[uint]string{}
		for _, entry := range marks {
			if dailyDismissedOn(dismissalCards[entry.EmployeeCardID], entry.Date) {
				continue
			} // новый код chat
			if entry.Date < result.Today && entry.Date > selectedLastWorked[entry.EmployeeCardID] && parseSalaryDecimal(entry.Status).Sign() > 0 {
				selectedLastWorked[entry.EmployeeCardID] = entry.Date
			}
		}
		// --------- конец нового кода chat
		// --------- новый код chat: буквенные отметки всех месяцев не расходуют остаток при подсветке
		reservationMarks := map[string]models.TimeSheet{}
		for _, month := range months {
			members := map[uint]bool{}
			for _, employee := range month.Employees {
				members[employee.ID] = true
			}
			for _, entry := range month.TimeSheets {
				if !members[entry.EmployeeCardID] {
					continue
				}
				key := fmt.Sprintf("%d:%s", entry.EmployeeCardID, entry.Date)
				if previous, ok := reservationMarks[key]; !ok || entry.ID > previous.ID {
					reservationMarks[key] = entry
				}
			}
		}
		// Рабочие дни с нулевой ставкой тоже задают начало последующего покрытия.
		for _, entry := range reservationMarks {
			if dailyDismissedOn(dismissalCards[entry.EmployeeCardID], entry.Date) {
				continue
			} // новый код chat
			if entry.Date < result.Today && entry.Date > lastWorked[entry.EmployeeCardID] && parseSalaryDecimal(entry.Status).Sign() > 0 {
				lastWorked[entry.EmployeeCardID] = entry.Date
			}
		}
		// --------- конец нового кода chat
		for id, name := range people {
			if !tableMembers[id] {
				continue
			} // новый код chat: история и другие сотрудники договора не расширяют список
			employee := DailyAllowanceEmployee{EmployeeCardID: id, EmployeeName: name, IsITR: itrMembers[id], IsAssigned: assigned[id], BalanceKopecks: paid[id] - earned[id], Cells: []DailyAllowanceCell{}}
			employee.Status = dismissalCards[id].Status               // новый код chat
			employee.DismissalDate = dismissalCards[id].DismissalDate // новый код chat
			if through != "" && employee.IsAssigned {
				if result.RateKopecks <= 0 {
					return dailyError("Укажите суточные в выбранном табеле")
				}
				// --------- новый код chat: индивидуальное количество ещё не отработанных дней и ставка выбранного табеля
				start := today // если рабочих отметок пока нет, сохраняем выдачу вперёд с сегодня
				if date, ok := selectedLastWorked[id]; ok {
					worked, err := time.Parse("2006-01-02", date)
					if err != nil {
						return err
					}
					start = worked.AddDate(0, 0, 1)
				}
				end, _ := time.Parse("2006-01-02", through)
				var needed int64
				for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
					date := day.Format("2006-01-02")
					if dailyDismissedOn(dismissalCards[id], date) {
						break
					} // новый код chat: прогноз заканчивается до увольнения
					key := fmt.Sprintf("%d:%s", id, date)
					if _, worked := actual[key]; worked {
						continue
					} // уже включено в фактические начисления
					if entry, ok := reservationMarks[key]; ok {
						mark := strings.TrimSpace(entry.Status)
						if mark != "" && !salaryDecimalPattern.MatchString(strings.ReplaceAll(mark, ",", ".")) {
							continue
						}
					}
					monthKey := day.Format("2006-01")
					if rate, exists := rates[monthKey]; !exists || rate <= 0 || ambiguous[monthKey] {
						return dailyError("Создайте табель за " + monthKey + " и проверьте ставку суточных")
					}
					var err error
					needed, err = dailyAdd(needed, result.RateKopecks)
					if err != nil {
						return err
					}
				}
				// --------- конец нового кода chat
				target, err := dailyAdd(earnedThrough[id], needed) // новый код chat: долг по выбранную дату плюс будущие дни, если они есть
				if err != nil {
					return err
				}
				deficit := target - paid[id]
				if deficit > 0 {
					extra := (result.RateKopecks - deficit%result.RateKopecks) % result.RateKopecks
					employee.SuggestedKopecks, err = dailyAdd(deficit, extra)
					if err != nil {
						return err
					}
				}
			}
			reserve := remaining[id]
			previewTotal := earned[id] // новый код chat: накопленная стоимость дней, включая непогашенный долг
			future := map[string]DailyAllowanceCell{}
			// --------- новый код chat: покрытие сразу после последнего рабочего дня, а не после сегодняшней даты
			start := today
			if date, ok := lastWorked[id]; ok {
				worked, err := time.Parse("2006-01-02", date)
				if err != nil {
					return err
				}
				start = worked.AddDate(0, 0, 1)
			}
			for day := start; !day.After(monthEnd) && employee.IsAssigned; day = day.AddDate(0, 0, 1) { // новый код chat: пороги прогноза также для ещё не обеспеченных дней
				date := day.Format("2006-01-02")
				if dailyDismissedOn(dismissalCards[id], date) {
					break
				} // новый код chat: остаток после увольнения не расходуем
				key := fmt.Sprintf("%d:%s", id, date)
				if entry, ok := reservationMarks[key]; ok {
					mark := strings.TrimSpace(entry.Status)
					if mark != "" && !salaryDecimalPattern.MatchString(strings.ReplaceAll(mark, ",", ".")) {
						continue
					}
				}
				monthKey := day.Format("2006-01")
				rate, ok := rates[monthKey]
				if !ok || rate <= 0 || ambiguous[monthKey] {
					break
				}
				covered := reserve
				if covered > rate {
					covered = rate
				}
				reserve -= covered
				cell := DailyAllowanceCell{Date: date, State: "empty", AmountKopecks: rate, CoveredKopecks: covered}
				// --------- новый код chat: прогноз без создания выплат и изменения баланса
				var err error
				previewTotal, err = dailyAdd(previewTotal, rate)
				if err != nil {
					break
				} // сумма такого прогноза уже превышает допустимый диапазон
				if required := previewTotal - paid[id]; required > 0 {
					cell.PreviewRequiredKopecks = required
				}
				// --------- конец нового кода chat
				if covered == rate {
					cell.State = "reserved"
				}
				future[date] = cell
			}
			// --------- конец нового кода chat
			for day := 1; day <= monthEnd.Day(); day++ {
				date := fmt.Sprintf("%04d-%02d-%02d", selected.Year, selected.Month, day)
				key := fmt.Sprintf("%d:%s", id, date)
				cell := DailyAllowanceCell{Date: date, State: "empty"}
				if known, ok := actual[key]; ok {
					cell = known
				}
				if known, ok := future[date]; ok {
					cell = known
				} // новый код chat: остаток виден и после рабочего дня в прошлом
				if entry, ok := marks[key]; ok {
					mark := strings.TrimSpace(entry.Status)
					if mark != "" && !salaryDecimalPattern.MatchString(strings.ReplaceAll(mark, ",", ".")) {
						cell.State = "letter"
						cell.Mark = mark
					}
				}
				// --------- новый код chat: закрытый день без начисления и прогноза
				if dailyDismissedOn(dismissalCards[id], date) {
					cell = DailyAllowanceCell{Date: date, State: "dismissed"}
				}
				// --------- конец нового кода chat
				employee.Cells = append(employee.Cells, cell)
			}
			result.Employees = append(result.Employees, employee)
		}
		sort.Slice(result.Employees, func(i, j int) bool {
			a, b := result.Employees[i], result.Employees[j]
			if a.EmployeeName == b.EmployeeName {
				return a.EmployeeCardID < b.EmployeeCardID
			}
			return a.EmployeeName < b.EmployeeName
		})
		return nil
	})
	return result, err
}

type DailyAllowancePaymentItem struct {
	EmployeeCardID uint   `json:"employee_card_id"`
	Amount         string `json:"amount"`
}
type DailyAllowanceBatchInput struct {
	RequestKey string                      `json:"request_key"`
	MonthID    uint                        `json:"month_id"`
	Date       string                      `json:"date"`
	Payments   []DailyAllowancePaymentItem `json:"payments"`
}

func validDailyPaymentDate(value string) bool {
	date, err := time.Parse("2006-01-02", value)
	return err == nil && date.Year() > 0
}

// Не допускаем выдачу, после которой суммарный баланс нельзя точно представить.
func checkDailyPaymentTotal(tx *gorm.DB, agreementID, employeeID, excludeID uint, amount int64) error {
	var existing []models.DailyAllowancePayment
	query := tx.Where("agreement_id = ? AND employee_card_id = ?", agreementID, employeeID)
	if excludeID != 0 {
		query = query.Where("id <> ?", excludeID)
	}
	if err := query.Select("amount_kopecks").Find(&existing).Error; err != nil {
		return err
	}
	total := amount
	for _, row := range existing {
		var err error
		total, err = dailyAdd(total, row.AmountKopecks)
		if err != nil {
			return err
		}
	}
	return nil
}
func CreateDailyAllowancePayments(db *gorm.DB, agreementID uint, input DailyAllowanceBatchInput) ([]models.DailyAllowancePayment, error) {
	rows := []models.DailyAllowancePayment{}
	if len(input.RequestKey) < 16 || len(input.RequestKey) > 100 || !validDailyPaymentDate(input.Date) || input.MonthID == 0 || len(input.Payments) == 0 || len(input.Payments) > 1000 {
		return rows, dailyError("Проверьте дату, табель и список получателей")
	}
	amounts := map[uint]int64{}
	var total int64
	for _, item := range input.Payments {
		amount := salaryRateKopecks(item.Amount)
		if amount <= 0 || item.EmployeeCardID == 0 {
			return rows, dailyError("Сумма выдачи должна быть положительной, с точностью до копеек")
		}
		if _, ok := amounts[item.EmployeeCardID]; ok {
			return rows, dailyError("Сотрудник повторяется в выдаче")
		}
		amounts[item.EmployeeCardID] = amount
		var err error
		total, err = dailyAdd(total, amount)
		if err != nil {
			return rows, err
		}
	}
	canonical := struct {
		MonthID uint
		Date    string
		Amounts map[uint]int64
	}{input.MonthID, input.Date, amounts}
	body, _ := json.Marshal(canonical)
	hash := sha256.Sum256(body)
	fingerprint := hex.EncodeToString(hash[:])
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&models.Agreement{}, agreementID).Error; err != nil {
			return err
		}
		var previous []models.DailyAllowancePayment
		if err := tx.Unscoped().Where("agreement_id = ? AND request_key = ?", agreementID, input.RequestKey).Find(&previous).Error; err != nil {
			return err
		}
		if len(previous) > 0 {
			for _, row := range previous {
				if row.RequestHash != fingerprint {
					return dailyError("Этот запрос уже использован для другой выдачи")
				}
			}
			rows = previous
			return nil
		}
		var month models.TimeSheetMonth
		if err := tx.Where("agreement_id = ?", agreementID).First(&month, input.MonthID).Error; err != nil {
			return err
		}
		if month.DailyAllowanceKopecks <= 0 {
			return dailyError("Укажите суточные в выбранном табеле")
		}
		for _, item := range input.Payments {
			amount := amounts[item.EmployeeCardID]
			if amount%month.DailyAllowanceKopecks != 0 {
				return dailyError("Сумма должна быть кратна суточным выбранного табеля")
			}
			var employee models.EmployeeCard
			if err := tx.Where("agreement_id = ?", agreementID).First(&employee, item.EmployeeCardID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return dailyError("Получатель больше не закреплён за договором")
				}
				return err
			}
			row := models.DailyAllowancePayment{AgreementID: agreementID, EmployeeCardID: employee.ID, EmployeeName: dailyEmployeeName(employee), TimeSheetMonthID: month.ID, Date: input.Date, AmountKopecks: amount, RequestKey: input.RequestKey, RequestHash: fingerprint}
			if err := checkDailyPaymentTotal(tx, agreementID, employee.ID, 0, amount); err != nil {
				return err
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			rows = append(rows, row)
		}
		return nil
	})
	return rows, err
}

func GetDailyAllowancePayments(db *gorm.DB, agreementID uint) ([]models.DailyAllowancePayment, error) {
	rows := []models.DailyAllowancePayment{}
	if err := db.First(&models.Agreement{}, agreementID).Error; err != nil {
		return rows, err
	}
	err := db.Where("agreement_id = ?", agreementID).Order("date DESC, id DESC").Find(&rows).Error
	return rows, err
}

type DailyAllowancePaymentUpdate struct {
	Date   string `json:"date"`
	Amount string `json:"amount"`
}

func UpdateDailyAllowancePayment(db *gorm.DB, agreementID, paymentID uint, input DailyAllowancePaymentUpdate) (models.DailyAllowancePayment, error) {
	var row models.DailyAllowancePayment
	amount := salaryRateKopecks(input.Amount)
	if !validDailyPaymentDate(input.Date) || amount <= 0 {
		return row, dailyError("Проверьте дату и положительную сумму выплаты")
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&models.Agreement{}, agreementID).Error; err != nil {
			return err
		}
		if err := tx.Where("agreement_id = ?", agreementID).First(&row, paymentID).Error; err != nil {
			return err
		}
		if amount != row.AmountKopecks {
			var month models.TimeSheetMonth
			// Удалённый табель сохраняет последнюю ставку для проверки истории выдач.
			if err := tx.Unscoped().Where("agreement_id = ?", agreementID).First(&month, row.TimeSheetMonthID).Error; err != nil {
				return err
			}
			if month.DailyAllowanceKopecks <= 0 || amount%month.DailyAllowanceKopecks != 0 {
				return dailyError("Новая сумма должна быть кратна текущей ставке исходного табеля")
			}
		}
		row.Date = input.Date
		row.AmountKopecks = amount
		if err := checkDailyPaymentTotal(tx, agreementID, row.EmployeeCardID, row.ID, amount); err != nil {
			return err
		}
		return tx.Save(&row).Error
	})
	return row, err
}
func DeleteDailyAllowancePayment(db *gorm.DB, agreementID, paymentID uint) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&models.Agreement{}, agreementID).Error; err != nil {
			return err
		}
		var row models.DailyAllowancePayment
		if err := tx.Where("agreement_id = ?", agreementID).First(&row, paymentID).Error; err != nil {
			return err
		}
		return tx.Delete(&row).Error
	})
}

// --------- конец нового кода chat
