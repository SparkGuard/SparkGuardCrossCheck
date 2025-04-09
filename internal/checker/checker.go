package checker

import (
	"errors"
)

var ErrNoNewWork = errors.New("не указан путь до новой работы")
var ErrNoWorks = errors.New("нет работ для сравнения")

type Checker interface {
	// Run запускает анализ работ.
	// newWorks - путь к каталогам с новыми работами.
	// oldWorks - путь к каталогам со старыми работами.
	Run(newWorks []string, oldWorks []string) ([]*ReportItem, error)
}
