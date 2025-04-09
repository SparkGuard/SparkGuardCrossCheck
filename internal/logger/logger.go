package logger

import (
	"CodeBorrowing/internal/utils"
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/writer"
	"io"
	"os"
	"time"
)

const (
	// Время хранение логов (неделя).
	oldLimitTime = time.Hour * 24 * 7

	// Обновление файла логов (каждый час).
	updateLogTime = time.Hour * 24
)

type Logger struct {
	*logrus.Logger
	file                *os.File
	updateLogFileTicker *time.Ticker
	logDir              string
}

// NewLogger создаёт логгер приложения.
func NewLogger(logDir string) *Logger {
	inner, f, err := createLogrus(logDir)
	if err != nil {
		panic(err)
	}

	instance := &Logger{
		Logger:              inner,
		logDir:              logDir,
		file:                f,
		updateLogFileTicker: time.NewTicker(updateLogTime),
	}

	// Удаляем логи, которые хранятся дольше недели.
	instance.removeOldLogs(oldLimitTime)

	// Обработчик обновления лог-файлов.
	go func() {
		for {
			<-instance.updateLogFileTicker.C
			instance.updateLogFile()
			instance.removeOldLogs(oldLimitTime)
		}
	}()

	return instance
}

func createLogrus(logDir string) (*logrus.Logger, *os.File, error) {
	instance := logrus.New()

	instance.SetLevel(logrus.DebugLevel)  // логироват все уровни
	instance.SetReportCaller(true)        // установить "function-caller"
	instance.SetFormatter(&myFormatter{}) // установить собственный форматер

	// Создание каталога логов.
	if _, err := utils.CreateDirectory(logDir); err != nil {
		return nil, nil, err
	}

	// Создание лог-файла.
	timeTag := time.Now().UTC().Format("2006_01_02_15_04_05") // YYYY_MM_dd_HH_mm_ss
	logFileName := fmt.Sprintf("%s/%s.log", logDir, timeTag)
	file, err := os.OpenFile(logFileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return nil, nil, err
	}

	// Удалить предыдущие потоки вывода.
	instance.SetOutput(io.Discard)

	// Добавить вывод в консоль.
	instance.AddHook(&writer.Hook{
		Writer:    os.Stdout,
		LogLevels: logrus.AllLevels,
	})

	// Добавить вывод в файл.
	instance.AddHook(&writer.Hook{
		Writer: file,
		LogLevels: []logrus.Level{logrus.InfoLevel, logrus.WarnLevel,
			logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel},
	})

	return instance, file, nil
}

func (log *Logger) Close() error {
	log.updateLogFileTicker.Stop()
	_ = log.file.Sync()
	_ = log.file.Close()
	return nil
}
