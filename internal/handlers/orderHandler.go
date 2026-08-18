package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"project/internal/database"
	"project/internal/mailer"
	"project/internal/models"
	"strings"
)

func GetAgreementOrderHandler(w http.ResponseWriter, r *http.Request) {
	var agreementId = r.PathValue("id")
	var orders []models.Order
	if err := database.DB.Where("agreement_id=?", agreementId).Preload("ManagerCard.Suplers").Preload("Materials.Materials").Find(&orders).Error; err != nil {
		http.Error(w, "Ошибка в получении данных", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
}

func GetAgreementOrderDetailHandler(w http.ResponseWriter, r *http.Request) {
	var orderId = r.PathValue("id")
	var order models.Order

	if err := database.DB.Preload("Materials.Materials.Estimate").Preload("ManagerCard.Dialog.Emails.Attachments").First(&order, orderId).Error; err != nil {
		http.Error(w, "Ошибка в получении данных", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

func UpdateAgreementOrder(w http.ResponseWriter, r *http.Request) {
	var orderId = r.PathValue("id")
	var input struct {
		Attachement_id uint   `json:"attachement_id"`
		Comment        string `json:"comment"`
		Materials      []struct {
			MaterialsId uint    `json:"materials_order_id"`
			Comment     string  `json:"comment"`
			PriceOrder  float64 `json:"price_order"`
		} `json:"materials"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "Ошибка в получении данных", http.StatusBadRequest)
		return
	}

	tx := database.DB.Begin()
	if tx.Error != nil {
		http.Error(w, "Ошибка инициализации транзакции", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	if err := tx.Model(&models.Order{}).Where("id = ?", orderId).Updates(map[string]interface{}{
		"attachment_id": input.Attachement_id,
		"comment":       input.Comment,
		"status":        false,
	}).Error; err != nil {
		http.Error(w, "Ошибка при обновлении данных", http.StatusBadRequest)
		return
	}

	for _, material_order := range input.Materials {
		if err := tx.Model(&models.OrderMaterial{}).Where("id = ?", material_order.MaterialsId).Updates(map[string]interface{}{
			"price_order": material_order.PriceOrder,
			"comment":     material_order.Comment,
		}).Error; err != nil {
			http.Error(w, "Ошибка при обновлении данных", http.StatusBadRequest)
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "Не удалось зафиксировать изменения в БД", http.StatusInternalServerError)
		return
	}
}

func GetPaidOrderHandler(w http.ResponseWriter, r *http.Request) {
	var orderId = r.PathValue("id")

	if err := database.DB.Model(&models.Order{}).Where("id = ?", orderId).Update("is_paid", true).Error; err != nil {
		msg := "[Error] ошибка при обновлении поля is_paid ордера"
		log.Println(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "is_paid успешно изменен на true"})
}

func CreateOrderHandler(w http.ResponseWriter, r *http.Request) {
	var order models.Order
	var managerCard models.ManagerCard

	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		http.Error(w, "Ошибка в получении данных", http.StatusBadRequest)
		return
	}

	tx := database.DB.Begin()
	if tx.Error != nil {
		http.Error(w, "Ошибка инициализации транзакции", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	if err := tx.Create(&order).Error; err != nil {
		http.Error(w, "Не получилось сохранить данные в базу данных", http.StatusBadRequest)
		return
	}

	if err := tx.Preload("Materials.Materials").Preload("ManagerCard").First(&order, order.ID).Error; err != nil {
		log.Printf("[ERROR] Не удалось загрузить связи для заказа №%d: %v", order.ID, err)
	}

	message, _ := BuildOrderMessage(order)
	log.Println(message)

	if err := tx.Preload("Dialog").First(&managerCard, order.ManagerCardID).Error; err != nil {
		http.Error(w, "Не получилось сохранить данные в базу данных", http.StatusBadRequest)
		return
	}

	email := models.Email{
		EmailTo:  order.ManagerCard.Email,
		Title:    "Заказ на покупку материала",
		Message:  message,
		DialogID: managerCard.Dialog.ID,
		IsRead:   true,
	}

	if err := tx.Create(&email).Error; err != nil {
		http.Error(w, "Не получилось сохранить данные в базу данных", http.StatusBadRequest)
		return
	}

	if err := tx.Commit().Error; err != nil {
		http.Error(w, "Не удалось зафиксировать изменения в БД", http.StatusInternalServerError)
		return
	}

	go func(email models.Email) {
		if err := mailer.SendOrderEmail(email, true); err != nil {
			log.Printf("[ERROR] Не удалось отправить письмо для заказа №%d: %v", email.ID, err)
		} else {
			log.Printf("[SUCCESS] Письмо для заказа №%d успешно отправлено!", email.ID)
		}
	}(email)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.Response{Message: "Заказ успешно создан!"})
}

type OrderMaterials struct {
	Quantity float64
	TypeUnit string
}

func BuildOrderMessage(o models.Order) (string, error) {
	var builder strings.Builder
	materialmap := make(map[string]*OrderMaterials)

	for _, material := range o.Materials {
		name := material.Materials.Name
		if existing, found := materialmap[name]; found {
			existing.Quantity += material.Quanity
		} else {
			materialmap[name] = &OrderMaterials{
				Quantity: material.Quanity,
				TypeUnit: material.Materials.TypeUnit,
			}
		}
	}

	builder.WriteString("Добрый день меня зовут Михаил и я представляю компанию ООО'БРМ' и хочу заказать у вас материалы\n")
	builder.WriteString(fmt.Sprintf("Комментарий: %s\n\n", o.Message))

	index := 1
	for name, data := range materialmap {
		builder.WriteString(fmt.Sprintf("%d. %s -- %.2f %s\n", index, name, data.Quantity, data.TypeUnit))
		index++
	}

	return builder.String(), nil
}
