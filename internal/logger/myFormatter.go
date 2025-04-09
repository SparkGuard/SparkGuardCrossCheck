package logger

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"runtime"
	"strings"
)

type myFormatter struct {
	logrus.TextFormatter
}

func (f *myFormatter) Format(e *logrus.Entry) ([]byte, error) {
	// Формат лога.
	format := "[%s] - [%s] - %s%s%s\n"
	timeTag := e.Time.Format("02.01.2006 15:04:05")
	caller, data := "", ""

	// Если лог критический, извлечь название функции, в которой был лог.
	if e.Caller != nil && (e.Level == logrus.ErrorLevel ||
		e.Level == logrus.FatalLevel || e.Level == logrus.PanicLevel) {
		caller = fmt.Sprintf("(%s) - ", f.extractCallFunction(e.Caller))
	}

	// Объединение параметров лога.
	if e.Data != nil && len(e.Data) != 0 {
		sb := strings.Builder{}
		for k, v := range e.Data {
			sb.WriteString(fmt.Sprintf("%s:%v; ", k, v))
		}

		s := sb.String()
		data = fmt.Sprintf(" {%s}", s[:len(s)-2])
	}

	// Формирование лога.
	return []byte(fmt.Sprintf(format, timeTag, e.Level, caller, e.Message, data)), nil
}

// extractCallFunction получает название функции, из которой пришёл лог.
func (f *myFormatter) extractCallFunction(caller *runtime.Frame) string {
	count := 0
	idx := strings.LastIndexFunc(caller.File, func(r rune) bool {
		if r == '/' || r == '\\' {
			count++
			if count == 2 {
				return true
			}
		}
		return false
	})

	idx++
	return fmt.Sprintf("%s:%d", caller.File[idx:], caller.Line)
}
