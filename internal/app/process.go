package app

import (
	"CodeBorrowing/internal/task"
	"CodeBorrowing/services/orchestrator"
	"errors"
)

// Process запускает главный процесс приложения.
// Возвращает true, если задача была получена, иначе false.
func (a *appT) process() bool {
	// Получение новой задачи.
	tasks, err := a.taskService.GetNewTasksOfCommonEvent()
	if err != nil {
		// Если полученная ошибка не связана с отсутствием задач.
		if errors.Is(err, task.ErrNoNewTask) {
			a.logger.Debug("Новая задача: отсутствует")
		} else {
			a.logger.Error(err)
		}
		return false
	}
	eventID := tasks[0].EventID

	tasksID := make([]uint64, len(tasks))          // массив для id задач, будет нужен для CloseTask.
	newWorksID := make(map[uint64]any, len(tasks)) // множество id работ.
	newWorksIDArr := make([]uint64, len(tasks))    // массив id работ.
	for i := 0; i < len(tasks); i++ {
		tasksID[i] = tasks[i].ID
		newWorksID[tasks[i].WorkID] = nil
		newWorksIDArr[i] = tasks[i].WorkID
	}

	a.logger.Infof("Получены новые задачи (eventId=%d, worksId=%v). Загрузка работ", eventID, newWorksIDArr)

	// Получение id всех работы из event.
	works, err := a.taskService.GetEventWorks(eventID)
	if err != nil {
		a.logger.Error(err)

		// Отправка серверу сигнала о том, что выполнение задачи завершено с ошибкой.
		if err = a.taskService.CloseTaskWithError(tasksID); err != nil {
			a.logger.Error(err)
		}

		return true
	}
	a.logger.Infof("Работы Загружены (count=%d). Начинаем анализ.", len(works))

	// Если работу не с чем сравнивать.
	if len(works) <= 1 {
		a.logger.Info("Единственную работу не с чем сравнивать")

		// Отправка серверу сигнала о том, что выполнение задачи завершено.
		if err = a.taskService.CloseTask(tasksID); err != nil {
			a.logger.Error(err)
		}

		return true
	}

	newWorks := make([]string, 0, len(tasks)) // путь к каталогам с новыми работами.
	oldWorksCount := len(works) - len(tasks)
	if oldWorksCount < 0 {
		oldWorksCount = 0
	}
	oldWorks := make([]string, 0, oldWorksCount) // путь к каталогам с остальными работами.

	// Цикл определяет новые и остальные работы.
	for _, work := range works {
		_, ok := newWorksID[work.WorkID]
		if ok {
			newWorks = append(newWorks, work.Path)
		} else {
			oldWorks = append(oldWorks, work.Path)
		}
	}

	// Запуск анализа работ.
	result, err := a.taskChecker.Run(newWorks, oldWorks)
	if err != nil {
		a.logger.Error(err)

		// Отправка серверу сигнала о том, что выполнение задачи завершено с ошибкой.
		if err = a.taskService.CloseTaskWithError(tasksID); err != nil {
			a.logger.Error(err)
		}
		return true
	}
	a.logger.Info("Работы успешно проанализированы. Отправка отчёта")

	// Обработка результата.
	for _, res := range result {
		// Формирование отчёта.
		report := &orchestrator.SendCrossCheckReportRequest{
			FirstWorkID:  res.Work1ID,
			SecondWorkID: res.Work2ID,
			Match:        make([]*orchestrator.SendCrossCheckReportMatches, len(res.Matches)),
		}

		// Обработка совпадений.
		for i, m := range res.Matches {
			report.Match[i] = &orchestrator.SendCrossCheckReportMatches{
				FirstWorkPath:   m.Work1File,
				FirstWorkStart:  m.Work1Start,
				FirstWorkSize:   m.Work1Size,
				SecondWorkPath:  m.Work2File,
				SecondWorkStart: m.Work2Start,
				SecondWorkSize:  m.Work2Size,
			}
		}

		// Отправка отчета.
		if err = a.taskService.SendReport(report); err != nil {
			a.logger.Error(err)
		}
	}

	// Отправка серверу сигнала о том, что выполнение задачи завершено.
	if err = a.taskService.CloseTask(tasksID); err != nil {
		a.logger.Error(err)
	}

	// Проверка лимита занятого места на диске.
	if err = a.taskService.CheckCacheSize(); err != nil {
		a.logger.Error(err)
	}

	return true
}
