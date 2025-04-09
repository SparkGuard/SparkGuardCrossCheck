package task

import (
	"CodeBorrowing/internal/logger"
	"CodeBorrowing/internal/utils"
	"CodeBorrowing/services/orchestrator"
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"
)

var ErrNoNewTask = errors.New("нет новой задачи")
var ErrNoWork = errors.New("нет работы")

type Service interface {
	// GetNewTasksOfCommonEvent получает новые задачи от сервера из одного события.
	GetNewTasksOfCommonEvent() ([]*orchestrator.Task, error)

	// GetEventWorks получает сущности работ event-а.
	GetEventWorks(eventID uint64) ([]WorkEntry, error)

	// SendReport отправляет отчёт на сервер.
	SendReport(report *orchestrator.SendCrossCheckReportRequest) error

	// CloseTask отправляет сигнал о завершении выполнения задачи.
	CloseTask(taskID []uint64) error

	// CloseTaskWithError отправляет сигнал о завершении выполнения задачи.
	CloseTaskWithError(taskID []uint64) error

	// GetWorkPath - получение пути к каталогу с работой.
	GetWorkPath(workID uint64) string

	// CheckCacheSize следит за лимитом занятого места на диске.
	CheckCacheSize() error
}

type service struct {
	grpcClient orchestrator.OrchestratorClient
	storage    Storage
	logger     *logger.Logger
	root       string
	size       uint64
}

// NewService создаёт новый сервис для работы с задачами.
// GrpcClient: grpc клиент.
// TaskStorage: хранилище работ студентов.
// Logger: логгер приложения.
// Path: путь к хранилищу работ.
// Size: лимит заполняемого на диске пространства в Мб.
func NewService(grpcClient orchestrator.OrchestratorClient, taskStorage Storage,
	logger *logger.Logger, path string, size uint64) (Service, error) {

	// Проверка лимита заполняемого на диске пространства
	if size < 50 {
		return nil, errors.New("слишком маленький размер для хранилища работ. Минимальный размер 50 Мб")
	}

	return &service{
		grpcClient: grpcClient,
		storage:    taskStorage,
		logger:     logger,
		root:       path,
		size:       size,
	}, nil
}

// GetWorkPath - получение пути к каталогу с работой.
func (s *service) GetWorkPath(workID uint64) string {
	return path.Join(s.root, "works", strconv.FormatUint(workID, 10))
}

// GetNewTasksOfCommonEvent получает новые задачи от сервера из одного события.
func (s *service) GetNewTasksOfCommonEvent() ([]*orchestrator.Task, error) {
	resp, err := s.grpcClient.GetAllNewTasksOfEvent(context.Background(), nil)
	if err != nil || resp.GetTask() == nil || len(resp.GetTask()) == 0 {
		return nil, ErrNoNewTask
	}

	return resp.GetTask(), nil
}

// getWorksID получает id работ event-а.
func (s *service) getWorksID(eventID uint64) ([]uint64, error) {
	req := &orchestrator.GetWorksOfEventRequest{
		EventID: eventID,
	}
	resp, err := s.grpcClient.GetWorksOfEvent(context.Background(), req)

	if err != nil {
		return nil, err
	}

	if resp.GetWorkID() == nil {
		return []uint64{}, nil
	}

	return resp.GetWorkID(), nil
}

// getWorksEntry читает информацию о работах из хранилища.
func (s *service) getWorksEntry(ids []uint64) (works []WorkEntry, notFound []uint64) {
	works = make([]WorkEntry, 0, len(ids)) // сущности работ.
	notFound = make([]uint64, 0, len(ids)) // список не найденных в хранилище работ.
	found := make([]uint64, 0, len(ids))   // список найденных в хранилище работ.

	for _, id := range ids {
		// Получение работы из хранилища.
		work, err := s.storage.GetWork(id)

		if err != nil { // работа не получена.
			notFound = append(notFound, id)
		} else { // работа найдена.
			works = append(works, work)
			found = append(found, id)
		}
	}

	// Обновление времени найденных работ.
	if err := s.storage.UpdateWorksTimestamp(found, time.Now()); err != nil {
		s.logger.Error(err)
	}

	return
}

// getDownloadUrls получает ссылку на скачивание работ.
func (s *service) getDownloadUrls(ids []uint64) ([]WorkUrl, error) {
	resp, err := s.grpcClient.GetWorksDownloadLinks(context.Background(), &orchestrator.GetWorksDownloadLinksRequest{
		WorkID: ids,
	})

	if err != nil {
		return nil, err
	}

	// Ответ сервера.
	items := resp.GetItem()
	if items == nil {
		return []WorkUrl{}, nil
	}

	// Формирование результата.
	result := make([]WorkUrl, len(items))
	for i, item := range items {
		if item == nil {
			continue
		}

		result[i] = WorkUrl{
			WorkID: item.GetWorkID(),
			Url:    item.GetDownloadLink(),
		}
	}

	return result, nil
}

// downloadWorks скачивает работы по id.
func (s *service) downloadWorks(ids []uint64) ([]WorkEntry, error) {
	if len(ids) == 0 {
		return []WorkEntry{}, nil
	}

	// Получить ссылки на скачивание.
	urls, err := s.getDownloadUrls(ids)
	if err != nil {
		return nil, err
	}

	// Формирование результата.
	result := make([]WorkEntry, 0, len(ids))
	for _, url := range urls {
		// Скачивание работы.
		work, err := s.downloadWork(url.WorkID, url.Url)

		if err == nil { // если работа скачана.
			result = append(result, work)
			continue
		}

		if !errors.Is(err, ErrNoWork) { // если скачать не получилось.
			s.logger.Errorf("workID: %d, %v, (%s)", url.WorkID, err, url.Url)
		}
	}

	return result, err
}

// prepareWorkDirectory подготавливает каталог для скачивания работы.
func prepareWorkDirectory(path string) error {
	// Создание каталога.
	existed, err := utils.CreateDirectory(path)

	if err != nil {
		return err
	}

	// Если каталог был уже создан, то очистить его.
	if existed {
		if err = utils.ClearDirectory(path); err != nil {
			return err
		}
	}

	return nil
}

// downloadFile скачивает файл и сохраняет его содержимое в массив байт.
func downloadFile(url string) ([]byte, error) {
	// http get запрос.
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	// Выполнение http запроса.
	client := http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, ErrNoWork
	}
	defer res.Body.Close()

	// Чтение ответа.
	buf := bytes.NewBuffer(make([]byte, res.ContentLength))
	_, err = io.Copy(buf, res.Body)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// unzipWork распаковывает архив в buf в каталог по пути unzipPath.
func (s *service) unzipWork(buf []byte, unzipPath string) error {
	// Открытие архива.
	reader := bytes.NewReader(buf)
	zipReader, err := zip.NewReader(reader, int64(len(buf)))
	if err != nil {
		return err
	}

	// Чтение элементов архива.
	for _, f := range zipReader.File {
		s.unzipItem(f, unzipPath)
	}

	return nil
}

// unzipItem разархивирует элемент архива.
func (s *service) unzipItem(f *zip.File, unzipPath string) {
	// Путь элемента архива.
	newFilePath := path.Join(unzipPath, f.Name)

	// Разархивируем папку.
	if f.FileInfo().IsDir() {
		if err := os.MkdirAll(newFilePath, os.ModePerm); err != nil {
			s.logger.Error(err)
		}
		return
	}

	// Разархивируем файл.
	s.unzipFile(f, newFilePath)
}

// unzipFile разархивирует файл.
func (s *service) unzipFile(f *zip.File, unzipPath string) {
	// Создание файла.
	destFile, err := os.Create(unzipPath)
	if err != nil {
		s.logger.Error(err)
		return
	}
	defer destFile.Close()

	// Открытие файла из архива.
	rc, err := f.Open()
	if err != nil {
		s.logger.Error(err)
		return
	}
	defer rc.Close()

	// Сохранение (копирование) байт в файл.
	if _, err = io.Copy(destFile, rc); err != nil {
		s.logger.Error(err)
	}
}

// downloadWork скачивает работу по ссылки.
func (s *service) downloadWork(workID uint64, url string) (WorkEntry, error) {
	work := WorkEntry{
		Path:      s.GetWorkPath(workID),
		Timestamp: time.Now(),
	}

	// Путь для распаковки архива.
	unzipPath := path.Join(work.Path, strconv.FormatUint(workID, 10))

	// Скачать архив по url.
	buf, err := downloadFile(url)
	if err != nil {
		return work, err
	}

	// Подготовка каталога для разархивирования.
	if err := prepareWorkDirectory(unzipPath); err != nil {
		return work, err
	}

	// Разархивировать работу.
	if err = s.unzipWork(buf, unzipPath); err != nil {
		return work, err
	}

	// Сохранить работу в хранилище.
	err = s.storage.SaveWork(workID, work.Path, work.Timestamp)
	if err != nil {
		return work, nil
	}

	return work, nil
}

// GetEventWorks получает сущности работ event-а.
func (s *service) GetEventWorks(eventID uint64) ([]WorkEntry, error) {
	// Получить id работ из event-а.
	ids, err := s.getWorksID(eventID)
	if err != nil {
		return nil, err
	}

	// Получить информацию о работах из хранилища.
	works, notFound := s.getWorksEntry(ids)
	if len(notFound) == 0 {
		return works, nil
	}

	// Скачивание не найденных работ.
	downloaded, err := s.downloadWorks(notFound)
	if err != nil {
		s.logger.Error(err)
		return works, nil
	}

	works = append(works, downloaded...)
	return works, nil
}

// SendReport отправляет отчёт на сервер.
func (s *service) SendReport(report *orchestrator.SendCrossCheckReportRequest) error {
	_, err := s.grpcClient.SendCrossCheckReport(context.Background(), report)
	if err != nil {
		return err
	}

	return nil
}

// CloseTask отправляет сигнал о завершении выполнения задачи.
func (s *service) CloseTask(taskID []uint64) error {
	_, err := s.grpcClient.CloseTask(context.Background(), &orchestrator.CloseTaskRequest{
		ID: taskID,
	})

	if err != nil {
		return err
	}

	return nil
}

// CloseTaskWithError отправляет сигнал о завершении выполнения задачи с ошибкой.
func (s *service) CloseTaskWithError(taskID []uint64) error {
	_, err := s.grpcClient.CloseTaskWithError(context.Background(), &orchestrator.CloseTaskRequest{
		ID: taskID,
	})

	if err != nil {
		return err
	}

	return nil
}

// removeOldWorks удаление старых работ.
func (s *service) removeOldWorks() (uint64, error) {
	// Получение последних 10 работ.
	works, err := s.storage.GetOldWorks(10)
	if err != nil || len(works) == 0 {
		return 0, err
	}

	// Размер удалённого пространства.
	var removed uint64 = 0

	for _, work := range works {
		// Вычисление размера каталога.
		rm, err := utils.GetDirectorySize(work.Path)
		if err != nil {
			s.logger.Error(err)
			continue
		}

		// Удаление каталога.
		err = os.RemoveAll(work.Path)
		if err != nil {
			s.logger.Error(err)
			continue
		}

		removed += rm
	}

	// id удалённых работ.
	ids := make([]uint64, len(works))
	for i, work := range works {
		ids[i] = work.WorkID
	}

	// Удаление работ из хранилища.
	if err = s.storage.DeleteWorks(ids); err != nil {
		return removed, nil
	}

	return removed, nil
}

// CheckCacheSize следит за лимитом занятого места на диске.
func (s *service) CheckCacheSize() error {
	// Вычисление размера каталога.
	size, err := utils.GetDirectorySize(s.root)
	if err != nil {
		return err
	}

	limit := s.size * 1024 * 1024

	// Пока размер каталога превышает лимит.
	for limit < size {
		// Удалить старые работы.
		removed, err := s.removeOldWorks()
		if err != nil {
			return err
		}

		size -= removed
	}

	return nil
}
