// --------- новый код chat: доступность уволенных по месяцу табеля
package handlers

import (
	"encoding/json"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http/httptest"
	"path/filepath"
	"project/internal/database"
	"project/internal/models"
	"testing"
)

func TestTimeSheetEmployeeDates(t *testing.T) {
	for _, date := range []string{"2026-11-01", "2026-11-30"} {
		employees := []models.EmployeeCard{{Name: "Иван", Status: true, DismissalDate: date}}
		for _, period := range [][3]uint{{2026, 10, 1}, {2026, 11, 1}, {2026, 12, 0}, {2025, 12, 1}, {2027, 1, 0}} {
			err := validateTimeSheetEmployeeDates(employees, period[0], period[1])
			if (err == nil) != (period[2] == 1) {
				t.Fatalf("%s period %v: %v", date, period, err)
			}
		}
	}
	if validateTimeSheetEmployeeDates([]models.EmployeeCard{{Status: true}}, 2026, 11) == nil {
		t.Fatal("unknown dismissal accepted")
	}
	if err := validateTimeSheetEmployeeDates([]models.EmployeeCard{{Status: false}}, 2026, 12); err != nil {
		t.Fatal(err)
	}
}

func TestTimeSheetEmployeeCandidates(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "candidates.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	old := database.DB
	database.DB = db
	t.Cleanup(func() { database.DB = old; sqlDB, _ := db.DB(); sqlDB.Close() })
	if err := db.AutoMigrate(&models.EmployeeCard{}); err != nil {
		t.Fatal(err)
	}
	for _, employee := range []models.EmployeeCard{
		{Name: "Свой", AgreementID: 17},
		{Name: "Чужой", AgreementID: 18},
		{Name: "Уволен", Status: true, DismissalDate: "2026-11-15"},
		{Name: "Без даты", Status: true},
	} {
		if err := db.Create(&employee).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		query string
		count int
	}{{"", 1}, {"?timesheet_candidates=1", 2}} {
		req := httptest.NewRequest("GET", "/"+test.query, nil)
		req.SetPathValue("id", "17")
		res := httptest.NewRecorder()
		GetAllEmployeeCardAgreementHandler(res, req)
		var employees []models.EmployeeCard
		if err := json.Unmarshal(res.Body.Bytes(), &employees); err != nil {
			t.Fatal(err)
		}
		if res.Code != 200 || len(employees) != test.count {
			t.Fatalf("response %d %s", res.Code, res.Body.String())
		}
		for _, employee := range employees {
			if employee.Name == "Чужой" || employee.Name == "Без даты" {
				t.Fatal("unexpected candidate")
			}
		}
	}
}

// --------- конец нового кода chat
