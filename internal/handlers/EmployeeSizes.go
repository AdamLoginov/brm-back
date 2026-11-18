// --------- новый код chat: проверка необязательного роста сотрудника
package handlers

import (
	"net/http"
	"project/internal/models"
)

func validateEmployeeSizes(w http.ResponseWriter, employee *models.EmployeeCard) bool {
	if employee.HeightCM != nil && *employee.HeightCM <= 0 {
		http.Error(w, "Рост должен быть положительным целым числом в сантиметрах", http.StatusBadRequest)
		return false
	}
	return true
}

// --------- конец нового кода chat
