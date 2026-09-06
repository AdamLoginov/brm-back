package models

import (
	"gorm.io/gorm"
)

type Response struct {
	Message string `json:"message"`
}

type EmployeeCard struct {
	gorm.Model
	Name              string              `gorm:"not null" json:"name"`
	MiddleName        *string             `json:"middle_name"`
	Surname           *string             `json:"surname"`
	Phone             *string             `json:"phone"`
	Email             string              `json:"email"`
	DateOfBirht       string              `json:"date_of_birth"`
	PlaceOFResidence  string              `json:"place_of_residence"`
	MaritalStatus     string              `json:"marital_status"`
	StartDate         string              `json:"start_date"`
	Expenses          string              `json:"experience"`
	Education         string              `json:"education"`
	PasportNumber     string              `json:"pasport_number"`
	PasportSerial     string              `json:"pasport_serial"`
	Snils             string              `json:"snils"`
	Profession        *string             `json:"profession"`
	SalaryRate        string              `json:"salary_rate"`
	Status            bool                `json:"status"`
	AgreementID       uint                `json:"agreement_id"`
	Agreement         Agreement           `gorm:"foreignKey:AgreementID; constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"agreement"`
	EmployeeDocuments []EmployeeDocuments `gorm:"foreignKey:EmployeeCardID;constraint:OnDelete:CASCADE;" json:"employee_documents"`
}

type CategoryEmployeeDocument struct {
	gorm.Model
	CategoryName      string              `json:"category_name"`
	EmployeeDocuments []EmployeeDocuments `gorm:"foreignKey:CategoryEmployeeDocumentID" json:"employee_documents"`
}

type EmployeeDocuments struct {
	gorm.Model
	DocumentName               string                   `json:"document_name"`
	DocumentReal               bool                     `json:"document_real"`
	FileName                   string                   `json:"file_name"`
	FilePath                   string                   `json:"file_path"`
	FileSize                   int64                    `json:"file_size"`
	MimeType                   string                   `json:"mime_type"`
	EmployeeCardID             uint                     `json:"employe_card_id"`
	CategoryEmployeeDocumentID uint                     `json:"category_employee_document_id"`
	CategoryEmployeeDocument   CategoryEmployeeDocument `gorm:"foreignKey:CategoryEmployeeDocumentID" json:"category_document_card"`
}

type TimeSheet struct {
	gorm.Model
	Date           string       `json:"date"`
	EmployeeCard   EmployeeCard `gorm:"foreignKey:EmployeeCardID" json:"employee_card"`
	EmployeeCardID uint         `json:"employee_card_id"`
	AgreementID    uint         `json:"agreement_id"`
	Status         string       `json:"status"`
}

type Advance struct {
	Date           string       `json:"date"`
	Summ           float64      `json:"summ"`
	EmployeeCard   EmployeeCard `gorm:"foreignKey:EmployeeCardID" json:"employee_card"`
	EmployeeCardID uint         `json:"employee_card_id"`
	AgreementID    uint         `json:"agreement_id"`
}

type User struct {
	gorm.Model
	Login          string       `gorm:"not null" json:"login"`
	Password       string       `gorm:"not null" json:"password"`
	EmployeeCardID uint         `json:"employee_card_id"`
	EmployeeCard   EmployeeCard `gorm:"foreignKey:EmployeeCardID" json:"employee_card"`
}

type Agreement struct {
	gorm.Model
	Name         string         `gorm:"not null" json:"name"`
	ShortName    string         `json:"short_name"`
	Number       string         `gorm:"not null" json:"number"`
	Customer     string         `gorm:"not null" json:"customer"`
	Address      string         `gorm:"not null" json:"address"`
	Price        float64        `json:"price"`
	DateEnd      string         `json:"date_end"`
	Status       string         `json:"status"`
	Estimate     []Estimate     `gorm:"foreignKey:AgreementID" json:"estimate,omitempty"`
	Expenses     []Expenses     `gorm:"foreignKey:AgreementID" json:"expenses"`
	Tool         []Tool         `gorm:"foreingKey:AgreementID" json:"tools"`
	Order        []Order        `gorm:"foreingKey:AgreementID" json:"orders"`
	EmployeeCard []EmployeeCard `gorm:"foreingKey:AgreementID" json:"employee_card"`
}

type Estimate struct {
	gorm.Model
	Name        string      `gorm:"not null" json:"name"`
	AgreementID uint        `gorm:"not null" json:"agreement_id"`
	Agreement   Agreement   `gorm:"foreignKey:AgreementID; constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Materials   []Materials `gorm:"foreignKey:EstimateID" json:"materials"`
}

type Suplers struct {
	gorm.Model
	Name              string
	Link              string              `json:"link"`
	CategorySuppliers []CategorySuppliers `gorm:"many2many:supplier_categories;" json:"category_suppliers"`
	ManagerCards      []ManagerCard       `gorm:"foreignKey:SupplierID" json:"manager_cards,omitempty"`
}

type CategorySuppliers struct {
	gorm.Model
	Name    string    `json:"name"`
	Suplers []Suplers `gorm:"many2many:supplier_categories;" json:"suppliers"`
}

type ManagerCard struct {
	gorm.Model
	FullName    string  `json:"full_name"`
	Email       string  `gorm:"not null" json:"email"`
	PhoneNumber string  `json:"phone_number"`
	SupplierID  uint    `gorm:"not null" json:"supplier_id"`
	Suplers     Suplers `gorm:"foreignKey:SupplierID; constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"suppliers"`
	Dialog      Dialog  `gorm:"foreignKey:ManagerCardID"`
}

type Materials struct {
	gorm.Model
	IdSmeta       string
	Name          string          `gorm:"not null"`
	TypeUnit      string          `gorm:"not null"`
	Quantity      float64         `gorm:"not null;decimal(16,2)"`
	PriceSmeta    float64         `gorm:"decimal(16,2)"`
	EstimateID    uint            `json:"estimate_id"`
	Estimate      Estimate        `gorm:"foreignKey:EstimateID; constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	OrderMaterial []OrderMaterial `gorm:"foreignKey:MaterialID" json:"order_materials"`
}

type OrderMaterial struct {
	gorm.Model
	OrderID    uint      `json:"order_id"`
	Order      Order     `gorm:"foreignKey:OrderID" json:"order"`
	MaterialID uint      `json:"material_id"`
	Materials  Materials `gorm:"foreignKey:MaterialID; constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"materials"`
	Quanity    float64   `json:"quantity"`
	PriceOrder float64   `json:"price_order"`
	Comment    string    `json:"comment"`
}

type Order struct {
	gorm.Model
	AgreementID   uint            `json:"agreement_id"`
	Agreement     Agreement       `grom:"foreignKey:AgreementID" json:"agreement"`
	ManagerCardID uint            `json:"manager_card_id"`
	ManagerCard   ManagerCard     `gorm:"foreignKey:ManagerCardID" json:"manager_card"`
	Message       string          `json:"message"`
	Materials     []OrderMaterial `gorm:"foreignKey:OrderID; constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"materials"`
	Status        bool            `gorm:"default:true" json:"status"`
	IsPaid        bool            `gorm:"default:false" json:"is_paid"`
	Dialog        Dialog          `gorm:"foreignKey:OrderID" json:"dialog"`
	AttachmentID  uint            `json:"attachment_id"`
	Comment       string          `json:"comment"`
}

type Dialog struct {
	gorm.Model
	OrderID       uint    `gorm:"default:0" json:"order_id"`
	ManagerCardID uint    `json:"manager_card_id"`
	EmailTo       string  `json:"email_to"`
	Emails        []Email `gorm:"foreignKey:DialogID" json:"emails"`
}

type Email struct {
	gorm.Model
	EmailTo     string       `json:"email_to"`
	EmailFrom   string       `json:"email_from"`
	Title       string       `json:"title"`
	Message     string       `json:"message"`
	InReplyTo   string       `json:"in_reply_to"`
	IsRead      bool         `json:"isRead"`
	DialogID    uint         `json:"dialog_id"`
	Dialog      Dialog       `gorm:"foreignKey:DialogID" json:"-"`
	Attachments []Attachment `gorm:"foreignKey:EmailID;constraint:OnDelete:CASCADE;" json:"attachments"`
}

type Attachment struct {
	gorm.Model
	FileName string `json:"file_name"`
	FilePath string `json:"file_path"`
	FileSize int64  `json:"file_size"`
	MimeType string `json:"mime_type"`
	EmailID  uint   `json:"email_id"`
}

type Expenses struct {
	gorm.Model
	AgreementId    uint         `json:"agreement_id"`
	EmployeeCardId uint         `json:"employee_card_id"`
	EmployeeCard   EmployeeCard `gorm:"foreignKey:EmployeeCardId" json:"employee_card"`
	Name           string       `json:"name"`
	Price          float64      `json:"price"`
	Account        string       `json:"account"`
}

type Tool struct {
	gorm.Model
	AgreementID  uint   `json:"agreement_id"`
	Name         string `json:"name"`
	InvNumber    string `json:"inv_number"`
	SerialNumber string `json:"serial_number"`
	Status       string `json:"status"`
}

type Task struct {
	gorm.Model
	Message          string       `json:"message"`
	Status           bool         `json:"status"`
	ToEmployeeID     uint         `json:"to_employee_id"`
	ToEmployeeCard   EmployeeCard `gorm:"foreignKey:ToEmployeeID" json:"to_employee_card"`
	FromEmployeeID   uint         `json:"from_employee_id"`
	FromEmployeeCard EmployeeCard `gorm:"foreignKey:FromEmployeeID" json:"from_employee_card"`
}
