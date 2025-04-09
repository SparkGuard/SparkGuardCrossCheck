package task

import (
	"CodeBorrowing/internal/logger"
	"CodeBorrowing/internal/utils"
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"strconv"
	"strings"
	"time"
)

// Заголовки sql сущностей.
const (
	sqlWorksTable    = "sqlWorksTable"
	sqlWorkId        = "workId"
	sqlWorkPath      = "path"
	sqlWorkTimestamp = "time"
)

// Формат времени в sqlite.
const workTimeFormat = "2006-01-02 15:04:05"

// Sql запросы.
var queryCreateTable = fmt.Sprintf("create table if not exists %s (%s integer primary key, %s text, %s text)", sqlWorksTable, sqlWorkId, sqlWorkPath, sqlWorkTimestamp)
var queryGetWork = fmt.Sprintf("select %s, %s, %s from %s where %s = $1", sqlWorkId, sqlWorkPath, sqlWorkTimestamp, sqlWorksTable, sqlWorkId)
var querySaveWork = fmt.Sprintf("insert into %s (%s, %s, %s) values ($1, $2, $3)", sqlWorksTable, sqlWorkId, sqlWorkPath, sqlWorkTimestamp)
var queryUpdateWorksTimestampFormat = fmt.Sprintf("update %s set %s = $1 where %s in (%%s)", sqlWorksTable, sqlWorkTimestamp, sqlWorkId)
var queryGetOldWorks = fmt.Sprintf("select %s, %s, %s from %s order by %s LIMIT $1", sqlWorkId, sqlWorkPath, sqlWorkTimestamp, sqlWorksTable, sqlWorkTimestamp)
var queryDeleteWorksFormat = fmt.Sprintf("delete from %s where %s in (%%s)", sqlWorksTable, sqlWorkId)

type Storage interface {
	// GetWork возвращает сущность работы по id.
	GetWork(id uint64) (WorkEntry, error)

	// SaveWork создаёт сущность о работе.
	SaveWork(workID uint64, path string, timestamp time.Time) error

	// UpdateWorksTimestamp обновляет время запроса к сущности.
	UpdateWorksTimestamp(ids []uint64, timestamp time.Time) error

	// GetOldWorks получает последние по времени count сущностей.
	GetOldWorks(count uint64) ([]WorkEntry, error)

	// DeleteWorks удаляет сущность из хранилища.
	DeleteWorks(ids []uint64) error

	// Close закрывает подключение к хранилищу.
	Close() error
}

type storage struct {
	appLogger *logger.Logger
	db        *sql.DB
}

// NewStorage создаёт подключение к Sqlite.
func NewStorage(appLogger *logger.Logger, path string) (Storage, error) {
	// Создание каталога для sqlite файла.
	if _, err := utils.CreateDirectory(path); err != nil {
		return nil, err
	}

	// Подключение к sqlite.
	db, err := sql.Open("sqlite3", fmt.Sprintf("%s/data.db", path))
	if err != nil {
		return nil, err
	}

	// Проверка подключения.
	if err = db.Ping(); err != nil {
		return nil, err
	}

	// Создание необходимых сущностей.
	_, err = db.Exec(queryCreateTable)
	if err != nil {
		return nil, err
	}

	return &storage{
		appLogger: appLogger,
		db:        db,
	}, nil
}

// Close закрывает подключение к sqlite.
func (s *storage) Close() error {
	if err := s.db.Close(); err != nil {
		return err
	}
	return nil
}

// GetWork возвращает сущность работы по id.
func (s *storage) GetWork(id uint64) (WorkEntry, error) {
	// Sql запрос.
	res := s.db.QueryRow(queryGetWork, id)

	work := WorkEntry{}
	var timeStr string

	// Чтение результата.
	err := res.Scan(&work.WorkID, &work.Path, &timeStr)
	if err != nil {
		return work, err
	}

	// Парсинг времени.
	work.Timestamp, err = time.Parse(workTimeFormat, timeStr)
	if err != nil {
		return work, fmt.Errorf("не получается прочитать значение %s", sqlWorkTimestamp)
	}

	return work, nil
}

// SaveWork создаёт сущность о работе.
func (s *storage) SaveWork(workID uint64, path string, timestamp time.Time) error {
	timeStr := timestamp.Format(workTimeFormat)

	// Sql запрос.
	_, err := s.db.Exec(querySaveWork, workID, path, timeStr)
	if err != nil {
		return err
	}

	return nil
}

// UpdateWorksTimestamp обновляет время запроса к сущности.
func (s *storage) UpdateWorksTimestamp(ids []uint64, timestamp time.Time) error {
	if len(ids) == 0 {
		return nil
	}

	// Конвертация времени.
	timeStr := timestamp.Format(workTimeFormat)

	// Объединение id работ для обновления.
	idsStr := join(ids)

	// Sql запрос.
	queryUpdateWorksTimestamp := fmt.Sprintf(queryUpdateWorksTimestampFormat, idsStr)
	_, err := s.db.Exec(queryUpdateWorksTimestamp, timeStr)
	if err != nil {
		return err
	}

	return nil
}

// GetOldWorks получает последние по времени count сущностей.
func (s *storage) GetOldWorks(count uint64) ([]WorkEntry, error) {
	// Sql запрос.
	res, err := s.db.Query(queryGetOldWorks, count)
	if err != nil {
		return nil, err
	}
	defer res.Close()

	var works []WorkEntry
	var timeStr string

	// Чтение результата.
	for res.Next() { // Iterate and fetch the records from result cursor
		var work WorkEntry
		err = res.Scan(&work.WorkID, &work.Path, &timeStr)
		if err != nil {
			s.appLogger.Error(err)
			continue
		}

		// Парсинг времени.
		work.Timestamp, err = time.Parse(workTimeFormat, timeStr)
		if err != nil {
			s.appLogger.Error(err)
			continue
		}

		works = append(works, work)
	}

	return works, nil
}

// DeleteWorks удаляет сущность из sqlite.
func (s *storage) DeleteWorks(ids []uint64) error {
	if len(ids) == 0 {
		return nil
	}

	// Объединение id работ для обновления.
	idsStr := join(ids)

	// Sql запрос.
	queryDeleteWorks := fmt.Sprintf(queryDeleteWorksFormat, idsStr)
	_, err := s.db.Exec(queryDeleteWorks)
	if err != nil {
		return err
	}

	return nil
}

// Join конвертирует []uint64 в строку с разделителем ','.
func join(ids []uint64) string {
	if len(ids) == 0 {
		return ""
	}

	sb := strings.Builder{}
	sb.WriteString(strconv.FormatUint(ids[0], 10))
	for i := 1; i < len(ids); i++ {
		sb.WriteByte(',')
		sb.WriteString(strconv.FormatUint(ids[i], 10))
	}

	return sb.String()
}
