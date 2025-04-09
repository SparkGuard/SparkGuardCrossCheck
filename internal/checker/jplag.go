package checker

import (
	"CodeBorrowing/internal/logger"
	"CodeBorrowing/internal/task"
	"CodeBorrowing/internal/utils"
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
)

// ResultFile название результирующего файла.
const ResultFile = "result.zip"

type jplag struct {
	logger      *logger.Logger
	checkerPath string
	workDir     string
	taskService task.Service
}

// NewJplagChecker создаёт адаптер для работы с Jplag.
func NewJplagChecker(logger *logger.Logger, checkerPath string, workDir string, taskService task.Service) Checker {
	return &jplag{
		logger:      logger,
		checkerPath: checkerPath,
		workDir:     workDir,
		taskService: taskService,
	}
}

// Run запускает анализ работ.
// newWorks - путь к каталогам с новыми работами.
// oldWorks - путь к каталогам со старыми работами.
func (c *jplag) Run(newWorks []string, oldWorks []string) ([]*ReportItem, error) {
	// Путь к результирующему файлу.
	resultPath := path.Join(c.workDir, ResultFile)
	defer os.Remove(resultPath)

	// Запуск анализа.
	if err := c.exec(newWorks, oldWorks, resultPath); err != nil {
		return nil, err
	}

	// Получение анализа.
	result, err := c.parse(resultPath)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Exec запускает анализ работ.
func (c *jplag) exec(newWorks []string, oldWorks []string, resultPath string) error {
	if len(newWorks) == 0 {
		return ErrNoNewWork
	}
	if len(oldWorks) == 0 && len(newWorks) == 1 {
		return ErrNoWorks
	}

	// Создание рабочего каталога.
	if _, err := utils.CreateDirectory(c.workDir); err != nil {
		return err
	}

	// Удаление предыдущего результата, если остался.
	if info, err := os.Stat(resultPath); err != nil {
		if info != nil && !info.IsDir() {
			if err = os.Remove(resultPath); err != nil {
				return err
			}
		}
	}

	// Формирование команды запуска анализа.
	newWorksStr := strings.Join(newWorks, ",")
	cmd := exec.Command("java", "-jar", c.checkerPath, "-new", newWorksStr, "-l", "csharp", "-r", resultPath)

	// Если имеются старые работы, добавить их в соответствующую категорию.
	if len(oldWorks) != 0 {
		oldWorksStr := strings.Join(oldWorks, ",")
		cmd.Args = append(cmd.Args, "-old", oldWorksStr)
	}

	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

// Parse читает результат анализа работ.
func (c *jplag) parse(resultPath string) ([]*ReportItem, error) {
	// Открыть результирующий архив.
	zipReader, err := zip.OpenReader(resultPath)
	if err != nil {
		return nil, err
	}
	defer zipReader.Close()

	var reports []*ReportItem

	for _, f := range zipReader.File {
		// Обработка элемента архива.
		report, err := c.readItem(f)
		if err != nil {
			c.logger.Error(err)
		}

		// Если отчёт был создан - добавить.
		if report != nil {
			reports = append(reports, report)
		}
	}

	return reports, nil
}

// readItem читает элемент результирующего архива.
func (c *jplag) readItem(f *zip.File) (*ReportItem, error) {
	// Если элемент является не интересным для чтения.
	if strings.HasPrefix(f.Name, "files") ||
		f.Name == "options.json" ||
		f.Name == "overview.json" ||
		f.Name == "README.txt" ||
		f.Name == "submissionFileIndex.json" {
		return nil, nil
	}

	// Открыть элемент архива.
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	// Прочитать данные.
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, err
	}

	// Парсинг данных.
	var result ResultDTO
	if err = json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	// Формирование отчета.
	report := &ReportItem{
		Avg: result.Similarities.Avg,
		Max: result.Similarities.Max,
	}

	// Идентификатор первой работы.
	report.Work1ID, err = strconv.ParseUint(strings.Split(result.ID1, "_")[0], 10, 64)
	if err != nil {
		return nil, err
	}

	// Идентификатор второй работы.
	report.Work2ID, err = strconv.ParseUint(strings.Split(result.ID2, "_")[0], 10, 64)
	if err != nil {
		return nil, err
	}

	// Количество лишних байт которые содержаться в пути к файлам.
	f1Skip, f2Skip := len(result.ID1)+1, len(result.ID2)+1

	for _, matchDTO := range result.Matches {
		// Путь к файлу первой и второй работы.
		match := MatchItem{
			Work1File: matchDTO.File1[f1Skip:],
			Work2File: matchDTO.File2[f2Skip:],
		}

		// Путь к файлам на сервере.
		work1Path := path.Join(c.taskService.GetWorkPath(report.Work1ID), matchDTO.File1[f1Skip/2:])
		work2Path := path.Join(c.taskService.GetWorkPath(report.Work2ID), matchDTO.File2[f2Skip/2:])

		// Вычисление позиций в первой работе, в которой замечена схожесть.
		match.Work1Start, match.Work1Size, err = getPositions(work1Path, matchDTO.Start1, matchDTO.Start1Col, matchDTO.End1, matchDTO.End1Col)
		if err != nil {
			c.logger.Error(err)
		}

		// Вычисление позиций во второй работе, в которой замечена схожесть.
		match.Work2Start, match.Work2Size, err = getPositions(work2Path, matchDTO.Start2, matchDTO.Start2Col, matchDTO.End2, matchDTO.End2Col)
		if err != nil {
			c.logger.Error(err)
		}

		report.Matches = append(report.Matches, match)
	}

	return report, nil
}

// getPositions вычисляет позиции в которых замечена схожесть.
// Path: путь к файлу.
// startLine: номер строки, начиная с которой замечена схожесть.
// startCol: номер столбца, начиная с которого замечена схожесть.
// endLine: номер строки, заканчивая с которой замечена схожесть.
// endCol: номер столбца, заканчивая с которого замечена схожесть.
func getPositions(path string, startLine, startCol, endLine, endCol uint64) (uint64, uint64, error) {
	// Открытие файла.
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	// Чтение файла.
	content, err := io.ReadAll(f)
	if err != nil {
		return 0, 0, err
	}

	runes := []rune(string(content))
	length := uint64(len(runes))
	idx := uint64(0)

	// Счётчики номеров строк.
	n, m := startLine, endLine

	// Цикл для пропуска строк, номер которых меньше startLine.
	for ; idx < length && n > 1; idx++ {
		if runes[idx] == '\n' {
			n--
			m--
		}
	}
	if idx == length {
		return 0, 0, nil
	}

	// Позиция, начиная с которой замечена схожесть.
	start := idx + startCol
	if startLine != 1 {
		start--
	}

	// Цикл для пропуска строк, номер которых меньше endLine.
	for idx = start; idx < length && m > 1; idx++ {
		if runes[idx] == '\n' {
			m--
		}
	}

	return start, idx - start + endCol, nil
}
