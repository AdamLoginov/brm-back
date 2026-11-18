// --------- новый код chat: отдельная команда безопасного переноса
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/url"
	"os"
	"path/filepath"
	"project/internal/service"
	"strings"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	dbPath := flag.String("db", "test.db", "Путь к SQLite")
	root := flag.String("root", "uploads", "Корень хранилища")
	apply := flag.Bool("apply", false, "Применить перенос (backend должен быть остановлен)")
	report := flag.String("report", "file-migration-report.json", "Файл отчёта")
	flag.Parse()
	path, err := filepath.Abs(*dbPath)
	if err != nil {
		return err
	}
	if _, err = os.Stat(path); err != nil {
		return err
	}
	storage, err := filepath.Abs(*root)
	if err != nil {
		return err
	}
	if err = os.Setenv("UPLOAD_ROOT", storage); err != nil {
		return err
	}
	uriPath := filepath.ToSlash(path)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	uri := &url.URL{Scheme: "file", Path: uriPath}
	query := url.Values{"mode": {"ro"}, "_busy_timeout": {"5000"}}
	if *apply {
		query.Set("mode", "rw")
	}
	uri.RawQuery = query.Encode()
	db, err := gorm.Open(sqlite.Open(uri.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	backup := ""
	if *apply {
		backup = path + ".before-file-migration-" + time.Now().Format("20060102-150405.000000000") + ".bak"
		// Создаём пустую цель с закрытыми правами до записи содержимого БД.
		reserved, reserveErr := os.OpenFile(backup, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if reserveErr != nil {
			return reserveErr
		}
		if err = reserved.Close(); err != nil {
			return err
		}
		if err = db.Exec("VACUUM INTO ?", backup).Error; err != nil {
			return fmt.Errorf("резервная копия БД: %w", err)
		}
		fmt.Println("Резервная копия БД:", backup)
	}
	rows, err := service.MigrateStoredFiles(db, *apply)
	if err != nil {
		return err
	}
	result := struct {
		Applied bool                       `json:"applied"`
		Backup  string                     `json:"database_backup,omitempty"`
		Rows    []service.FileMigrationRow `json:"files"`
	}{*apply, backup, rows}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(*report, data, 0600); err != nil {
		return err
	}
	counts := map[string]int{}
	for _, row := range rows {
		counts[row.Status]++
	}
	fmt.Printf("Результат: %v. Отчёт: %s\n", counts, *report)
	if counts["error"] > 0 || counts["missing"] > 0 {
		return fmt.Errorf("есть отсутствующие файлы или ошибки; см. отчёт")
	}
	return nil
}

// --------- конец нового кода chat
