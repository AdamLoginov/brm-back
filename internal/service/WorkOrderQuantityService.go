// --------- новый код chat: точные десятичные величины без float64
package service

import (
	"errors"
	"strconv"
	"strings"
)

func ParseWorkQuantity(text string, digits int, allowZero bool) (int64, error) {
	if digits != 2 && digits != 3 {
		return 0, errors.New("Некорректная точность")
	}
	text = strings.ReplaceAll(strings.TrimSpace(text), ",", ".")
	parts := strings.Split(text, ".")
	if len(parts) > 2 || len(parts[0]) == 0 || len(parts[0]) > 9 {
		return 0, errors.New("Некорректное количество")
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
		if len(fraction) == 0 || len(fraction) > digits {
			return 0, errors.New("Слишком много знаков после запятой")
		}
	}
	for _, c := range parts[0] + fraction {
		if c < '0' || c > '9' {
			return 0, errors.New("Количество должно быть неотрицательным числом")
		}
	}
	fraction += strings.Repeat("0", digits-len(fraction))
	value, err := strconv.ParseInt(parts[0]+fraction, 10, 64)
	if err != nil || (!allowZero && value == 0) {
		return 0, errors.New("Количество должно быть больше нуля")
	}
	return value, nil
}

// --------- конец нового кода chat
