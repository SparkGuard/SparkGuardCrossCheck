package logger

import (
	"os"
	"path"
	"time"
)

// setOutputs устанавливает потоки вывода логов.
func (log *Logger) updateLogFile() {
	// Создание нового логгера.
	newLogger, f, err := createLogrus(log.logDir)
	if err != nil {
		log.Error(err)
		return
	}

	// Замена логгера.
	log.Logger = newLogger
	log.file.Close()
	log.file = f
}

// removeOldLogs удаляет старые логи, которые хранятся дольше limit.
func (log *Logger) removeOldLogs(limit time.Duration) {
	// Получение всех объектов каталога логов.
	entries, err := os.ReadDir(log.logDir)
	if err != nil {
		log.Error(err)
		return
	}

	// Вычисление даты старых логов.
	exp := time.Now().Add(-limit).UTC()
	for _, e := range entries {
		// Каталог пропускаем.
		if e.IsDir() || len(e.Name()) < 4 { // 4: ".log"
			continue
		}

		// Парсинг времени из названия лога.
		timeTag := e.Name()[:len(e.Name())-4] // YYYY_MM_dd_HH_mm_ss
		tm, err := time.Parse("2006_01_02_15_04_05", timeTag)
		if err != nil {
			continue
		}

		// Если лог старый - удаляем.
		if tm.Compare(exp) <= 0 {
			err = os.Remove(path.Join(log.logDir, e.Name()))
			if err != nil {
				log.Error(err)
			}
		}
	}
}
