package models

import (
	"gorm.io/gorm"
	"time" // новый код chat: дата закрытия или прощения штрафа
)

type Response struct {
	Message string `json:"message"`
}

// --------- новый код chat: книга наряд-заданий и склад; количества — целые сотые/тысячные
type MeasurementUnit struct {
	ID      uint   `gorm:"primaryKey" json:"ID"`
	Name    string `json:"name"`
	NameKey string `gorm:"uniqueIndex" json:"-"`
}
type AgreementWorkshop struct {
	ID          uint   `gorm:"primaryKey" json:"ID"`
	AgreementID uint   `gorm:"uniqueIndex:workshop_name" json:"agreement_id"`
	Name        string `json:"name"`
	NameKey     string `gorm:"uniqueIndex:workshop_name" json:"-"`
}
type AgreementWorkBlock struct {
	ID          uint   `gorm:"primaryKey" json:"ID"`
	AgreementID uint   `gorm:"index" json:"agreement_id"`
	WorkshopID  uint   `gorm:"uniqueIndex:block_name" json:"workshop_id"`
	Name        string `json:"name"`
	NameKey     string `gorm:"uniqueIndex:block_name" json:"-"`
}
type AgreementWorkPosition struct {
	ID          uint   `gorm:"primaryKey" json:"ID"`
	AgreementID uint   `gorm:"index" json:"agreement_id"`
	BlockID     uint   `gorm:"uniqueIndex:position_number" json:"block_id"`
	Number      string `gorm:"uniqueIndex:position_number" json:"number"`
	Name        string `json:"name"`
}
type WorkOrderBook struct {
	ID          uint `gorm:"primaryKey" json:"ID"`
	AgreementID uint `gorm:"uniqueIndex:work_book_month" json:"agreement_id"`
	Year        int  `gorm:"uniqueIndex:work_book_month" json:"year"`
	Month       int  `gorm:"uniqueIndex:work_book_month" json:"month"`
}
type WorkOrderDay struct {
	ID     uint   `gorm:"primaryKey" json:"ID"`
	BookID uint   `gorm:"uniqueIndex:work_book_day" json:"book_id"`
	Date   string `gorm:"uniqueIndex:work_book_day;index" json:"date"`
}
type WorkOrderShift struct {
	ID          uint  `gorm:"primaryKey" json:"ID"`
	AgreementID uint  `gorm:"index" json:"agreement_id"`
	DayID       uint  `gorm:"uniqueIndex:work_day_shift" json:"day_id"`
	WorkshopID  uint  `gorm:"uniqueIndex:work_day_shift" json:"workshop_id"`
	Number      int   `gorm:"uniqueIndex:work_day_shift" json:"number"`
	Version     int64 `json:"version"`
}
type WorkOrderEmployee struct {
	ID              uint   `gorm:"primaryKey" json:"ID"`
	ShiftID         uint   `gorm:"uniqueIndex:work_shift_employee" json:"shift_id"`
	EmployeeCardID  uint   `gorm:"uniqueIndex:work_shift_employee;index" json:"employee_card_id"`
	EmployeeName    string `json:"employee_name"`
	HoursHundredths int64  `json:"hours_hundredths"`
}
type WorkOrderTask struct {
	Accounted           bool   `gorm:"not null;default:false" json:"accounted"` // новый код chat: работа учтена отдельно
	ID                  uint   `gorm:"primaryKey" json:"ID"`
	ShiftID             uint   `gorm:"index" json:"shift_id"`
	BlockID             uint   `gorm:"index;not null;default:0" json:"block_id"`          // новый код chat: блок сохраняется и без позиции
	HoursNotSpecified   bool   `gorm:"not null;default:false" json:"hours_not_specified"` // новый код chat: отличаем пустые часы от введённого нуля
	PositionID          uint   `gorm:"index" json:"position_id"`
	Name                string `json:"name"`
	UnitID              uint   `json:"unit_id"`
	QuantityHundredths  int64  `json:"quantity_hundredths"`
	QuantityThousandths *int64 `json:"quantity_thousandths"` // новый код chat: nil у старых записей
	HoursHundredths     int64  `json:"hours_hundredths"`
}
type WarehouseMaterial struct {
	ID          uint   `gorm:"primaryKey" json:"ID"`
	AgreementID uint   `gorm:"index" json:"agreement_id"`
	WorkshopID  uint   `gorm:"uniqueIndex:warehouse_material_name" json:"workshop_id"`
	Name        string `json:"name"`
	NameKey     string `gorm:"uniqueIndex:warehouse_material_name" json:"-"`
	UnitID      uint   `json:"unit_id"`
}
type WarehouseReceipt struct {
	ID                  uint   `gorm:"primaryKey" json:"ID"`
	MaterialID          uint   `gorm:"index" json:"material_id"`
	Date                string `json:"date"`
	QuantityThousandths int64  `json:"quantity_thousandths"`
}
type WorkOrderMaterial struct {
	ID                  uint  `gorm:"primaryKey" json:"ID"`
	TaskID              uint  `gorm:"index" json:"task_id"`
	MaterialID          uint  `gorm:"index" json:"material_id"`
	QuantityThousandths int64 `json:"quantity_thousandths"`
}

// --------- конец нового кода chat

// --------- новый код chat: общий справочник специализаций
type EmployeeSpecialization struct {
	ID   uint   `gorm:"primaryKey" json:"ID"`
	Name string `gorm:"not null;uniqueIndex" json:"name"`
}

// --------- конец нового кода chat

type EmployeeCard struct {
	gorm.Model
	// --------- новый код chat: телефоны и специализации для будущей фильтрации
	Phones            []string                 `gorm:"serializer:json" json:"phones"`
	Specializations   []EmployeeSpecialization `gorm:"many2many:employee_card_specializations;" json:"specializations"`
	SpecializationIDs []uint                   `gorm:"-" json:"specialization_ids,omitempty"`
	// --------- конец нового кода chat
	Name             string  `gorm:"not null" json:"name"`
	MiddleName       *string `json:"middle_name"`
	Surname          *string `json:"surname"`
	Phone            *string `json:"phone"`
	Email            string  `json:"email"`
	DateOfBirht      string  `json:"date_of_birth"`
	PlaceOFResidence string  `json:"place_of_residence"`
	MaritalStatus    string  `json:"marital_status"`
	StartDate        string  `json:"start_date"`
	DismissalDate    string  `json:"dismissal_date"` // новый код chat: дата увольнения, пустая для работающих
	Expenses         string  `json:"experience"`
	Education        string  `json:"education"`
	PasportNumber    string  `json:"pasport_number"`
	PasportSerial    string  `json:"pasport_serial"`
	Snils            string  `json:"snils"`
	Profession       *string `json:"profession"`
	SalaryRate       string  `json:"salary_rate"`
	// --------- новый код chat: официальное трудоустройство и месячная зарплата в копейках
	OfficiallyEmployed   bool  `gorm:"not null;default:false" json:"officially_employed"`
	MonthlySalaryKopecks int64 `gorm:"not null;default:0" json:"monthly_salary_kopecks"`
	// --------- конец нового кода chat
	// --------- новый код chat: размеры сотрудника и принадлежность к ИТР
	ClothingSize string `json:"clothing_size"`
	ShoeSize     string `json:"shoe_size"`
	HeightCM     *int   `json:"height_cm"`
	IsITR        bool   `gorm:"not null;default:false" json:"is_itr"`
	HasAlimony   bool   `gorm:"not null;default:false" json:"has_alimony"` // новый код chat: отметка об алиментах
	// --------- конец нового кода chat
	Status            bool                `json:"status"`
	AgreementID       uint                `json:"agreement_id"`
	Agreement         Agreement           `gorm:"foreignKey:AgreementID; constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"agreement"`
	EmployeeDocuments []EmployeeDocuments `gorm:"foreignKey:EmployeeCardID;constraint:OnDelete:CASCADE;" json:"employee_documents"`
	TimeSheetMonths   []TimeSheetMonth    `gorm:"many2many:timesheet_month_employees;" json:"time_sheet_months"`
}

type CategoryEmployeeDocument struct {
	gorm.Model
	CategoryName      string              `json:"category_name"`
	EmployeeDocuments []EmployeeDocuments `gorm:"foreignKey:CategoryEmployeeDocumentID" json:"employee_documents"`
}

// --------- новый код chat: единый реестр физических файлов
type StoredFile struct {
	gorm.Model
	StorageKey   string `gorm:"uniqueIndex;not null" json:"-"`
	OriginalName string `json:"original_name"`
	Size         int64  `json:"size"`
	MIMEType     string `json:"mime_type"`
	SHA256       string `gorm:"size:64" json:"sha256"`
	UploadedBy   uint   `json:"uploaded_by"`
	Kind         string `gorm:"index" json:"kind"`
}

// --------- конец нового кода chat

type EmployeeDocuments struct {
	gorm.Model
	StoredFileID               *uint                    `gorm:"index" json:"stored_file_id,omitempty"` // новый код chat: старые пути остаются совместимыми
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

type TimeSheetMonth struct {
	gorm.Model
	DailyAllowanceDisabledIDs []uint `gorm:"serializer:json" json:"daily_allowance_disabled_ids"` // новый код chat: отключённые суточные в этом месяце
	HiddenEmployeeIDs         []uint `gorm:"serializer:json" json:"hidden_employee_ids"`          // новый код chat: скрытие строк без удаления отметок и зарплаты
	// --------- новый код chat: настройки месячного табеля, точные десятичные часы
	ShiftHours            string `gorm:"not null;default:12" json:"shift_hours"`
	AbsencePenaltyKopecks int64  `gorm:"not null;default:0" json:"absence_penalty_kopecks"`
	DailyAllowanceKopecks int64  `gorm:"not null;default:0" json:"daily_allowance_kopecks"` // новый код chat: суточные, пока без начисления
	// --------- конец нового кода chat
	Month       uint           `json:"month"`
	Year        uint           `json:"year"`
	AgreementID uint           `json:"agreement_id"`
	TimeSheets  []TimeSheet    `gorm:"foreignKey:TimeSheetMonthID;" json:"time_sheets"`
	Employees   []EmployeeCard `gorm:"many2many:timesheet_month_employees;" json:"employees"`

	SalaryAccruals []SalaryAccrual `gorm:"foreignKey:TimeSheetMonthID;constraint:OnDelete:CASCADE;" json:"salary_accruals,omitempty"` // новый код chat: начисления табеля
}

// --------- новый код chat: сохранённое начисление сотрудника по табелю
type SalaryAccrual struct {
	gorm.Model
	TimeSheetMonthID  uint   `gorm:"not null;uniqueIndex:idx_salary_accrual_employee_month" json:"time_sheet_month_id"`
	EmployeeCardID    uint   `gorm:"not null;uniqueIndex:idx_salary_accrual_employee_month" json:"employee_card_id"`
	AgreementID       uint   `gorm:"not null;index" json:"agreement_id"`
	EmployeeName      string `json:"employee_name"`
	TotalHours        string `json:"total_hours"`
	ShiftHours        string `json:"shift_hours"` // новый код chat: дробная норма из табеля
	SalaryRateKopecks int64  `json:"salary_rate_kopecks"`
	AmountKopecks     int64  `json:"amount_kopecks"`
}

// --------- конец нового кода chat

type TimeSheet struct {
	gorm.Model
	Date             string       `json:"date"`
	EmployeeCard     EmployeeCard `gorm:"foreignKey:EmployeeCardID" json:"employee_card"`
	EmployeeCardID   uint         `json:"employee_card_id"`
	AgreementID      uint         `json:"agreement_id"`
	Status           string       `json:"status"`
	TimeSheetMonthID uint         `json:"time_sheet_month_id"`
}

type EmployeeBalance struct {
	gorm.Model
	EmployeeCardID uint         `gorm:"uniqueIndex" json:"employee_card_id"`
	EmployeeCard   EmployeeCard `gorm:"foreignKey:EmployeeCardID" json:"employee_card"`
	BalanceSalary  float64      `gorm:"default:0" json:"balance_salary"`
}

type Payment struct {
	gorm.Model
	Date           string       `json:"date"`
	Summ           float64      `json:"summ"`
	EmployeeCard   EmployeeCard `gorm:"foreignKey:EmployeeCardID" json:"employee_card"`
	EmployeeCardID uint         `json:"employee_card_id"`
	AgreementID    uint         `json:"agreement_id"`
	Message        string       `json:"message"`
	Status         string       `json:"status"`
	Flag           string       `json:"flag"`
}

type User struct {
	gorm.Model
	LastActiveAt   *time.Time   `json:"last_active_at"` // новый код chat: последнее посещение, неизвестное время — NULL
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

// --------- новый код chat: представители другой стороны договора, отдельно от сотрудников
type AgreementContact struct {
	gorm.Model
	AgreementID uint     `gorm:"not null;index" json:"agreement_id"`
	FullName    string   `gorm:"not null" json:"full_name"`
	Position    string   `json:"position"`
	Phones      []string `gorm:"serializer:json" json:"phones"`
	Emails      []string `gorm:"serializer:json" json:"emails"`
	Comment     string   `json:"comment"`
}

// --------- конец нового кода chat

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
	StoredFileID *uint  `gorm:"index" json:"stored_file_id,omitempty"` // новый код chat
	FileName     string `json:"file_name"`
	FilePath     string `json:"file_path"`
	FileSize     int64  `json:"file_size"`
	MimeType     string `json:"mime_type"`
	EmailID      uint   `json:"email_id"`
}

// --------- новый код chat: выплаченные авансы, сохраняемые отдельно от начислений
// --------- новый код chat: начисления суточных за дни и фактические выдачи
type DailyAllowanceAccrual struct {
	gorm.Model
	AgreementID      uint   `gorm:"index;not null" json:"agreement_id"`
	TimeSheetMonthID uint   `gorm:"uniqueIndex:idx_daily_accrual_day;not null" json:"time_sheet_month_id"`
	EmployeeCardID   uint   `gorm:"uniqueIndex:idx_daily_accrual_day;not null" json:"employee_card_id"`
	Date             string `gorm:"uniqueIndex:idx_daily_accrual_day;not null" json:"date"`
	EmployeeName     string `json:"employee_name"`
	AmountKopecks    int64  `json:"amount_kopecks"`
}

type DailyAllowancePayment struct {
	gorm.Model
	AgreementID      uint   `gorm:"uniqueIndex:idx_daily_payment_request;index;not null" json:"agreement_id"`
	EmployeeCardID   uint   `gorm:"uniqueIndex:idx_daily_payment_request;not null" json:"employee_card_id"`
	RequestKey       string `gorm:"uniqueIndex:idx_daily_payment_request;not null" json:"-"`
	RequestHash      string `json:"-"`
	TimeSheetMonthID uint   `gorm:"index;not null" json:"time_sheet_month_id"`
	EmployeeName     string `json:"employee_name"`
	Date             string `json:"date"`
	AmountKopecks    int64  `json:"amount_kopecks"`
}

// --------- конец нового кода chat

type SalaryAdvance struct {
	gorm.Model
	AgreementID    uint   `gorm:"index" json:"agreement_id"`
	EmployeeCardID uint   `gorm:"index" json:"employee_card_id"`
	EmployeeName   string `json:"employee_name"`
	Date           string `json:"date"`
	AmountKopecks  int64  `gorm:"check:amount_kopecks > 0" json:"amount_kopecks"`
	Comment        string `json:"comment"`
	IsSettled      bool   `gorm:"default:false" json:"is_settled"`
}

// --------- новый код chat: премии за конкретный месяц договора
type SalaryBonus struct {
	gorm.Model
	AgreementID    uint   `gorm:"index;not null" json:"agreement_id"`
	EmployeeCardID uint   `gorm:"index;not null" json:"employee_card_id"`
	EmployeeName   string `json:"employee_name"`
	Month          uint   `gorm:"not null" json:"month"`
	Year           uint   `gorm:"not null" json:"year"`
	AmountKopecks  int64  `gorm:"not null;check:amount_kopecks > 0" json:"amount_kopecks"`
	Comment        string `json:"comment"`
	IsPaid         bool   `gorm:"not null;default:false" json:"is_paid"`
}

// --------- новый код chat: долги и сохранённые согласования начислений
type SalaryDebt struct {
	gorm.Model
	AgreementID      uint   `gorm:"index;not null" json:"agreement_id"`
	EmployeeCardID   uint   `gorm:"index;not null" json:"employee_card_id"`
	EmployeeName     string `json:"employee_name"`
	Date             string `json:"date"`
	AmountKopecks    int64  `gorm:"not null;check:amount_kopecks > 0" json:"amount_kopecks"`
	Side             string `gorm:"not null" json:"side"`
	Comment          string `json:"comment"`
	IsClosed         bool   `gorm:"not null;default:false" json:"is_closed"`
	SourceApprovalID *uint  `gorm:"index" json:"source_approval_id"`
}

// --------- новый код chat: индивидуальные выплаты и согласования
type SalaryPaymentDraft struct {
	gorm.Model
	AgreementID      uint `gorm:"uniqueIndex:idx_salary_draft;not null"`
	TimeSheetMonthID uint `gorm:"uniqueIndex:idx_salary_draft;not null"`
	EmployeeCardID   uint `gorm:"uniqueIndex:idx_salary_draft;not null"`
	AmountKopecks    int64
	Revision         string
}

// --------- новый код chat: алименты принадлежат сотруднику, независимо от договора
type SalaryAlimony struct {
	gorm.Model
	EmployeeCardID       uint       `gorm:"index;not null" json:"employee_card_id"`
	EmployeeName         string     `json:"employee_name"`
	Date                 string     `json:"date"`
	AmountKopecks        int64      `gorm:"not null" json:"amount_kopecks"`
	Comment              string     `json:"comment"`
	IsSettled            bool       `gorm:"not null;default:false" json:"is_settled"`
	ClosedAt             *time.Time `json:"closed_at"`
	SettlementApprovalID *uint      `gorm:"index" json:"settlement_approval_id"`
}

// новый код chat: история видимости сотрудника во вкладке; не владелец и не область удержания
type SalaryAlimonyEmployeeHistory struct {
	EmployeeCardID uint `gorm:"primaryKey"`
	AgreementID    uint `gorm:"primaryKey"`
}

// --------- конец нового кода chat

// --------- новый код chat: повтор массового запроса не повторяет финансовые операции
type SalaryApprovalBatch struct {
	gorm.Model
	AuthorUserID uint   `gorm:"index"` // новый код chat: повтор запроса только тем же пользователем
	RequestKey   string `gorm:"uniqueIndex;not null"`
	RequestHash  string
	Snapshot     string `gorm:"type:text"`
}

// --------- конец нового кода chat

// --------- новый код chat: индивидуальные выплаты и согласования
type SalaryApproval struct {
	// новый код chat: автор и группа сохраняются вместе со снимком согласования
	AuthorUserID uint   `gorm:"index" json:"author_user_id"`
	AuthorName   string `json:"author_name"`
	GroupKey     string `gorm:"index" json:"-"`
	// --------- новый код chat: индивидуальные выплаты и согласования
	EmployeeCardID    uint    `gorm:"index"`
	ActiveEmployeeKey *string `gorm:"uniqueIndex"`
	// --------- новый код chat: индивидуальные выплаты и согласования
	gorm.Model
	AgreementID      uint   `gorm:"index;not null" json:"agreement_id"`
	TimeSheetMonthID uint   `gorm:"index;not null" json:"time_sheet_month_id"`
	ActiveMonthID    *uint  `gorm:"uniqueIndex" json:"-"`
	Date             string `json:"date"`
	RequestKey       string `gorm:"uniqueIndex;not null" json:"-"`
	RequestHash      string `json:"-"`
	Snapshot         string `gorm:"type:text" json:"-"`
	Changes          string `gorm:"type:text" json:"-"`
}

// --------- конец нового кода chat

// --------- конец нового кода chat

// --------- конец нового кода chat

// --------- новый код chat: штрафы по дате назначения
type SalaryPenalty struct {
	gorm.Model
	// --------- новый код chat: источник автоматического штрафа, nil для ручного
	SourceTimeSheetMonthID *uint   `gorm:"index" json:"source_time_sheet_month_id"`
	SourceTimeSheetID      *uint   `gorm:"index" json:"source_time_sheet_id"`
	SourceTimeSheetKey     *string `gorm:"uniqueIndex" json:"-"`
	// --------- конец нового кода chat
	AgreementID    uint   `gorm:"index" json:"agreement_id"`
	EmployeeCardID uint   `gorm:"index" json:"employee_card_id"`
	EmployeeName   string `json:"employee_name"`
	Date           string `gorm:"index" json:"date"`
	AmountKopecks  int64  `gorm:"check:amount_kopecks > 0" json:"amount_kopecks"`
	Comment        string `json:"comment"`
	// --------- новый код chat: прежние штрафы получают статус active при миграции
	Status   string     `gorm:"not null;default:active" json:"status"`
	ClosedAt *time.Time `json:"closed_at"`
	// --------- конец нового кода chat
}

// --------- конец нового кода chat

// --------- новый код chat: общий справочник категорий расходов
type ExpenseCategory struct {
	ID      uint   `gorm:"primaryKey" json:"ID"`
	Name    string `gorm:"not null" json:"name"`
	NameKey string `gorm:"not null;uniqueIndex" json:"-"`
}

// --------- конец нового кода chat

type Expenses struct {
	gorm.Model
	CategoryID     *uint            `gorm:"index" json:"category_id"`                    // новый код chat: nil — без категории
	Category       *ExpenseCategory `gorm:"constraint:OnDelete:SET NULL;" json:"-"`      // новый код chat
	ExpenseDate    string           `gorm:"not null;default:''" json:"expense_date"`     // новый код chat: дата расхода отдельно от создания записи
	IsReimbursed   bool             `gorm:"not null;default:false" json:"is_reimbursed"` // новый код chat: возмещён ли личный расход
	AgreementId    uint             `json:"agreement_id"`
	EmployeeCardId uint             `json:"employee_card_id"`
	EmployeeCard   EmployeeCard     `gorm:"foreignKey:EmployeeCardId" json:"employee_card"`
	Name           string           `json:"name"`
	Price          float64          `json:"price"`
	Account        string           `json:"account"`
}

type Tool struct {
	gorm.Model
	// --------- новый код chat: ответственный и сроки проверки инструмента
	EmployeeCardID     *uint         `gorm:"index" json:"employee_card_id"`
	EmployeeCard       *EmployeeCard `gorm:"foreignKey:EmployeeCardID" json:"employee_card"`
	InspectionDate     string        `json:"inspection_date"`
	NextInspectionDate string        `json:"next_inspection_date"`
	// --------- конец нового кода chat
	AgreementID  uint   `json:"agreement_id"`
	Name         string `json:"name"`
	InvNumber    string `json:"inv_number"`
	SerialNumber string `json:"serial_number"`
	Status       string `json:"status"`
}

// --------- новый код chat: расходники МТР и общий справочник категорий
type ConsumableCategory struct {
	gorm.Model
	Name    string `json:"name"`
	NameKey string `gorm:"uniqueIndex:idx_consumable_category_name,where:deleted_at IS NULL" json:"-"`
}

// --------- новый код chat: одна запись СИЗ — одна вещь, цена в копейках
type Ppe struct {
	gorm.Model
	Name           string        `json:"name"`
	Size           string        `json:"size"`
	Category       string        `json:"category"`
	PriceKopecks   int64         `json:"price_kopecks"`
	AgreementID    uint          `gorm:"index" json:"agreement_id"`
	EmployeeCardID *uint         `gorm:"index" json:"employee_card_id"`
	EmployeeCard   *EmployeeCard `gorm:"foreignKey:EmployeeCardID" json:"employee_card"`
}

// --------- конец нового кода chat

type Consumable struct {
	gorm.Model
	Name        string `json:"name"`
	AddedDate   string `json:"added_date"`
	Quantity    int64  `json:"quantity"`
	AgreementID *uint  `gorm:"index" json:"agreement_id"`
	CategoryID  *uint  `gorm:"index" json:"category_id"`
}

// --------- конец нового кода chat

type Task struct {
	DueDate string `gorm:"default:''" json:"due_date"` // новый код chat: дата включительно, без времени; пусто — без срока
	gorm.Model
	// --------- новый код chat: участники общей задачи и фактический исполнитель
	Assignees     []User `gorm:"many2many:task_assignees;" json:"assignees"`
	AssigneeIDs   []uint `gorm:"-" json:"assignee_ids,omitempty"`
	CompletedByID *uint  `json:"completed_by_id"`
	CompletedBy   User   `gorm:"foreignKey:CompletedByID" json:"completed_by"`
	// --------- конец нового кода chat
	CompletedAt *time.Time `json:"completed_at"` // новый код chat: время последнего выполнения, nil до выполнения
	Message     string     `json:"message"`
	Status      bool       `json:"status"`
	ToUserID    uint       `json:"to_user_id"`
	ToUser      User       `gorm:"foreignKey:ToUserID" json:"to_user_card"`
	FromUserID  uint       `json:"from_user_id"`
	FromUser    User       `gorm:"foreignKey:FromUserID" json:"from_user_card"`
	Priority    uint       `json:"priority"`
}

// --------- новый код chat: личные уведомления о событиях задач
type Notification struct {
	gorm.Model
	RecipientID uint       `gorm:"index:idx_notification_recipient;not null" json:"recipient_id"`
	TaskID      uint       `gorm:"index;not null" json:"task_id"`
	ActorID     uint       `json:"actor_id"`
	ActorName   string     `json:"actor_name"`
	Kind        string     `json:"kind"`
	TaskMessage string     `json:"task_message"`
	ReadAt      *time.Time `json:"read_at"`
}

// --------- конец нового кода chat
