package app

import (
	"CodeBorrowing/internal/checker"
	"CodeBorrowing/internal/config"
	"CodeBorrowing/internal/logger"
	"CodeBorrowing/internal/middleware"
	"CodeBorrowing/internal/task"
	"CodeBorrowing/services/orchestrator"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"path"
	"time"
)

type App interface {
	Run()
	Close() error
}

type appT struct {
	grpcConnection *grpc.ClientConn
	grpcClient     orchestrator.OrchestratorClient
	cfg            config.Config
	logger         *logger.Logger

	taskStorage task.Storage
	taskService task.Service
	taskChecker checker.Checker
}

// Init инициализирует приложение.
func Init(cfg config.Config) (App, error) {
	// Инициализация логгера.
	appLogger := logger.NewLogger(path.Join(cfg.WorkDir, "logs"))
	appLogger.Info("Инициализация приложения")

	// Создание grpc соединения.
	appLogger.Info("Установление подключения к grpc клиенту")
	conn, err := grpc.NewClient(cfg.MainServerHost,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(middleware.NewAuthInterceptor(cfg.MainServerKey)))

	if err != nil {
		return nil, fmt.Errorf("grpc: не получается подключиться %s, %v", cfg.MainServerHost, err)
	}

	// Создание grpc клиента.
	appLogger.Info("Регистрация grpc клиента")
	grpcClient := orchestrator.NewOrchestratorClient(conn)

	// Инициализация хранилища работ студентов.
	appLogger.Info("Инициализация хранилища работ студентов")
	storagePath := path.Join(cfg.WorkDir, "storage")
	taskStorage, err := task.NewStorage(appLogger, storagePath)
	if err != nil {
		return nil, err
	}

	// Сервис обработки работ студентов.
	appLogger.Info("Создание сервиса обработки работ студентов")
	taskService, err := task.NewService(grpcClient, taskStorage, appLogger, storagePath, cfg.StorageSize)
	if err != nil {
		return nil, err
	}

	// Анализатор работ.
	taskChecker := checker.NewJplagChecker(appLogger, cfg.CheckerPath, path.Join(cfg.WorkDir, "check", "01"), taskService)

	appLogger.Info("Приложение успешно инициализировано")

	return &appT{
		grpcConnection: conn,
		grpcClient:     grpcClient,
		cfg:            cfg,
		logger:         appLogger,

		taskStorage: taskStorage,
		taskService: taskService,
		taskChecker: taskChecker,
	}, nil
}

// Run запускает работу приложения.
func (a *appT) Run() {
	quit := make(chan interface{})              // Сюда придёт сигнал, что надо завершить приложение.
	scheduler := time.NewTimer(5 * time.Second) // Будильник для проверки новой задачи.
	isRunning := true                           // Индикатор работы приложения.

	// Функция корректного завершения приложения.
	go a.gracefulShutdown(quit)

	a.logger.Info("Приложение запущено")

	for isRunning {
		select {
		// Завершение приложения.
		case <-quit:
			a.logger.Info("Остановка приложения")
			isRunning = false
			scheduler.Stop()

		// Время проверить наличие новой задачи на сервере.
		case <-scheduler.C:
			// Начать процесс выполнения задачи
			hasTask := a.process()
			if !isRunning {
				break
			}

			// Задержка перед следующей задачей.
			if hasTask {
				scheduler = time.NewTimer(100 * time.Millisecond)
			} else {
				scheduler = time.NewTimer(5 * time.Second)
			}
		}
	}
}

func (a *appT) Close() error {
	_ = a.grpcConnection.Close()
	_ = a.taskStorage.Close()
	_ = a.logger.Close()
	return nil
}
