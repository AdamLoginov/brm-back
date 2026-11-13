package service

import (
	"log"
	"project/internal/database"
	"project/internal/models"
)

func CreateBalanceForUser() error {
	var EmployeeCard []models.EmployeeCard

	if err := database.DB.Find(&EmployeeCard).Error; err != nil {
		log.Println("[Error 1]", err)
		return err
	}
	for _, employee := range EmployeeCard {
		var balance models.EmployeeBalance

		if err := database.DB.FirstOrCreate(&balance, models.EmployeeBalance{EmployeeCardID: employee.ID}).Error; err != nil {
			return err
		}
	}
	return nil
}

func ChangeEmployeeBalance(employeeId int, flag string, summ float64) error {
	var employeeBalance models.EmployeeBalance
	if err := database.DB.Where("employee_card_id = ?", employeeId).First(&employeeBalance).Error; err != nil {
		return err
	}
	employeeBalance.BalanceSalary -= summ
	if err := database.DB.Model(&models.EmployeeBalance{}).Where("employee_card_id = ?", employeeId).Update("balance_salary", employeeBalance.BalanceSalary).Error; err != nil {
		return err
	}
	log.Println(employeeBalance.BalanceSalary)
	log.Println(employeeId, flag, summ)

	return nil
}
