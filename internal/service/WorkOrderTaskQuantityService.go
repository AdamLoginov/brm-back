// --------- новый код chat: точные объёмы и совместимость со старыми записями
package service

import "project/internal/models"

func workQuantityThousandths(task models.WorkOrderTask) int64 {
	if task.QuantityThousandths != nil {
		return *task.QuantityThousandths
	}
	return task.QuantityHundredths * 10
}

// --------- конец нового кода chat
