package app

import (
	"os"
	"os/signal"
	"syscall"
)

// gracefulShutdown получает сигналы для прекращения работы приложения.
func (a *appT) gracefulShutdown(quit chan<- interface{}) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGABRT, syscall.SIGQUIT, syscall.SIGHUP,
		syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)

	sig := <-ch
	a.logger.Infof("ОСТАНОВКА: Получен сигнал %v", sig)
	quit <- nil
}
