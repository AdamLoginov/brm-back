// --------- новый код chat: годовая статистика по сохранённым отметкам всех договоров
package service

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"math/big"
	"project/internal/models"
	"sort"
	"strings"
	"time"
)

var ErrEmployeeStatisticsInput = errors.New("Укажите корректный год")

// --------- новый код chat: дневные значения календаря, включая отметки разных договоров
type EmployeeStatisticsDay struct {
	Date  string   `json:"date"`
	Hours string   `json:"hours"`
	Marks []string `json:"marks"`
}

// --------- конец нового кода chat

type EmployeeStatisticsMonth struct {
	Month       int    `json:"month"`
	Shifts      string `json:"shifts"`
	Absences    int    `json:"absences"`
	Hours       string `json:"hours"`
	SickDays    int    `json:"sick_days"`
	DaysOff     int    `json:"days_off"`
	TravelDays  int    `json:"travel_days"`
	OffSiteDays int    `json:"off_site_days"`
}
type EmployeeStatistics struct {
	Year        int                       `json:"year"`
	Years       []int                     `json:"years"` // новый код chat: годы с положительными отметками часов
	Shifts      string                    `json:"shifts"`
	Absences    int                       `json:"absences"`
	Hours       string                    `json:"hours"`
	SickDays    int                       `json:"sick_days"`
	DaysOff     int                       `json:"days_off"`
	TravelDays  int                       `json:"travel_days"`
	OffSiteDays int                       `json:"off_site_days"`
	Months      []EmployeeStatisticsMonth `json:"months"`
	Days        []EmployeeStatisticsDay   `json:"days"` // новый код chat: календарь выбранного года
}

func GetEmployeeStatistics(db *gorm.DB, employeeID uint, year int) (EmployeeStatistics, error) {
	result := EmployeeStatistics{Year: year, Years: []int{}, Months: []EmployeeStatisticsMonth{}, Days: []EmployeeStatisticsDay{}}
	if year < 1 || year > 9999 {
		return result, ErrEmployeeStatisticsInput
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		var employee models.EmployeeCard
		if err := tx.First(&employee, employeeID).Error; err != nil {
			return err
		}
		var months []models.TimeSheetMonth
		// новый код chat: табели сотрудника за все годы для списка доступных периодов
		if err := tx.Where("id IN (?)", tx.Model(&models.TimeSheet{}).Select("time_sheet_month_id").Where("employee_card_id = ?", employeeID)).Find(&months).Error; err != nil {
			return err
		}
		byID := map[uint]models.TimeSheetMonth{}
		ids := []uint{}
		for _, month := range months {
			if month.Month >= 1 && month.Month <= 12 {
				byID[month.ID] = month
				ids = append(ids, month.ID)
			}
		}
		var entries []models.TimeSheet
		if len(ids) > 0 {
			if err := tx.Where("employee_card_id = ? AND time_sheet_month_id IN ?", employeeID, ids).Order("id").Find(&entries).Error; err != nil {
				return err
			}
		}
		// Исторический дубль одной даты в одном табеле не удваивает часы или прогул.
		latest := map[string]models.TimeSheet{}
		for _, entry := range entries {
			latest[fmt.Sprintf("%d/%s", entry.TimeSheetMonthID, entry.Date)] = entry
		}
		// --------- новый код chat: только фактически отработанные годы, без исторических дублей
		workedYears := map[int]bool{}
		for _, entry := range latest {
			month := byID[entry.TimeSheetMonthID]
			date, err := time.Parse("2006-01-02", entry.Date)
			hours := parseSalaryDecimal(entry.Status)
			if err == nil && date.Year() >= 1 && date.Year() <= 9999 &&
				date.Year() == int(month.Year) && uint(date.Month()) == month.Month &&
				hours.Sign() > 0 && hours.Cmp(big.NewRat(24, 1)) <= 0 {
				workedYears[date.Year()] = true
			}
		}
		for value := range workedYears {
			result.Years = append(result.Years, value)
		}
		sort.Sort(sort.Reverse(sort.IntSlice(result.Years)))
		// --------- конец нового кода chat
		// --------- новый код chat: одна положительная отметка — одна смена; часы отдельно
		shifts := [12]int{}
		hoursByMonth := [12]*big.Rat{}
		counts := [12]EmployeeStatisticsMonth{}
		// новый код chat: точная сумма часов и полный перечень действительных отметок даты
		dayHours := map[string]*big.Rat{}
		dayMarks := map[string][]string{}
		for i := range hoursByMonth {
			hoursByMonth[i] = new(big.Rat)
		}
		for _, entry := range latest {
			month := byID[entry.TimeSheetMonthID]
			date, err := time.Parse("2006-01-02", entry.Date)
			if err != nil || date.Year() != year || date.Year() != int(month.Year) || uint(date.Month()) != month.Month {
				continue
			}
			index := int(month.Month) - 1
			mark := strings.TrimSpace(entry.Status)
			// новый код chat: нули, пустые и недопустимые отметки не создают активность
			if mark == "О" {
				mark = "Н"
			}
			hours := parseSalaryDecimal(mark)
			letter := mark == "П" || mark == "Б" || mark == "В" || mark == "Д" || mark == "Н"
			if letter || (hours.Sign() > 0 && hours.Cmp(big.NewRat(24, 1)) <= 0) {
				dayMarks[entry.Date] = append(dayMarks[entry.Date], mark)
				if dayHours[entry.Date] == nil {
					dayHours[entry.Date] = new(big.Rat)
				}
				if !letter {
					dayHours[entry.Date].Add(dayHours[entry.Date], hours)
				}
			}
			switch mark {
			case "П":
				counts[index].Absences++
				continue
			case "Б":
				counts[index].SickDays++
				continue
			case "В":
				counts[index].DaysOff++
				continue
			case "Д":
				counts[index].TravelDays++
				continue
			case "Н", "О": // Старое обозначение той же отметки.
				counts[index].OffSiteDays++
				continue
			}
			if hours.Sign() <= 0 || hours.Cmp(big.NewRat(24, 1)) > 0 {
				continue
			}
			shifts[index]++
			hoursByMonth[index].Add(hoursByMonth[index], hours)
		}
		total := new(big.Rat)
		totalShifts := 0
		for i := 0; i < 12; i++ {
			total.Add(total, hoursByMonth[i])
			totalShifts += shifts[i]
			item := counts[i]
			item.Month, item.Shifts, item.Hours = i+1, fmt.Sprint(shifts[i]), hoursByMonth[i].FloatString(3)
			result.Absences += item.Absences
			result.SickDays += item.SickDays
			result.DaysOff += item.DaysOff
			result.TravelDays += item.TravelDays
			result.OffSiteDays += item.OffSiteDays
			result.Months = append(result.Months, item)
		}
		result.Shifts = fmt.Sprint(totalShifts)
		result.Hours = total.FloatString(3)
		// новый код chat: календарь включает каждый день, в том числе 29 февраля
		for date := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC); date.Year() == year; date = date.AddDate(0, 0, 1) {
			key := date.Format("2006-01-02")
			marks := dayMarks[key]
			if marks == nil {
				marks = []string{}
			}
			sort.Strings(marks)
			hours := "0.000"
			if value := dayHours[key]; value != nil {
				hours = value.FloatString(3)
			}
			result.Days = append(result.Days, EmployeeStatisticsDay{Date: key, Hours: hours, Marks: marks})
		}
		// --------- конец нового кода chat
		return nil
	})
	return result, err
}

// --------- конец нового кода chat
