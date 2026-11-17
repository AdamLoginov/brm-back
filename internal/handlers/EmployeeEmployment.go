// --------- новый код chat: правила официального трудоустройства
package handlers

import (
	"net/http"
	"project/internal/models"
)

func validateEmployeeEmployment(w http.ResponseWriter, employee *models.EmployeeCard) bool {
	if !employee.OfficiallyEmployed {
		employee.MonthlySalaryKopecks = 0
		return true
	}
	if employee.MonthlySalaryKopecks < 0 || employee.MonthlySalaryKopecks > 9007199254740991 {
		http.Error(w, "Месячная зарплата должна быть неотрицательной суммой до копеек в допустимом диапазоне", http.StatusBadRequest)
		return false
	}
	return true
}

// --------- конец нового кода chat
