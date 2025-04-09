package config

import (
	"fmt"
	"os"
	"strconv"
	"sync"
)

type Config struct {
	WorkDir        string
	CheckerPath    string
	StorageSize    uint64
	MainServerHost string
	MainServerKey  string
}

// Заголовки переменных среды.
const (
	envWorkDir        = "workdir"        // Путь к каталогу приложения
	envStorageSize    = "storageSize"    // Размер папки хранилища работ в Мб.
	envCrossCheckLib  = "checkerPath"    // Путь к библиотеке для анализа работ.
	envMainServerHost = "mainServerHost" // IP адрес главного сервера
	envMainServerKey  = "mainServerKey"  // Ключ идентификации для главного сервера.
)

var instance Config
var once = sync.Once{}

// GetConfig читает и сохраняет переменные среды (Singleton).
func GetConfig() (Config, error) {
	var configErr error = nil
	isErr := true

	once.Do(func() {
		instance = Config{}
		cacheSize, err := strconv.ParseUint(os.Getenv(envStorageSize), 10, 64)
		if err != nil {
			configErr = err
			return
		}

		instance.WorkDir = os.Getenv(envWorkDir)
		instance.CheckerPath = os.Getenv(envCrossCheckLib)
		instance.MainServerHost = os.Getenv(envMainServerHost)
		instance.MainServerKey = os.Getenv(envMainServerKey)
		instance.StorageSize = cacheSize

		// Проверка входных параметров.
		if instance.WorkDir == "" {
			err = fmt.Errorf("переменная среды \"%s\" не установлена", envWorkDir)
		} else if instance.CheckerPath == "" {
			err = fmt.Errorf("переменная среды \"%s\" не установлена", envCrossCheckLib)
		} else {
			isErr = false
		}
	})

	if isErr {
		return instance, configErr
	}
	return instance, nil
}
