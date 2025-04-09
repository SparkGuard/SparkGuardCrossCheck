package utils

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// CreateDirectory создаёт каталог по указанному пути.
// Если каталог был успешно создан, возвращает (false, nil).
// Если каталог уже был создан, возвращает (true, nil).
// Если при создании каталога произошла ошибка, возвращает (false, Error).
func CreateDirectory(path string) (alreadyExisted bool, error error) {
	alreadyExisted = false

	// Проверка существования каталога.
	stat, err := os.Stat(path)
	if err != nil {
		// Если произошла ошибка НЕ по причине существования каталога.
		if !errors.Is(err, fs.ErrNotExist) {
			return false, err
		}
	} else if stat.IsDir() { // Если каталог уже существует.
		return true, nil
	}

	// Создание каталога.
	if err = os.MkdirAll(path, os.ModePerm); err != nil {
		return false, err
	}

	return false, nil
}

// ClearDirectory полностью очищает каталог от содержимого.
func ClearDirectory(path string) error {
	// Удалить каталог и его содержимое.
	err := os.RemoveAll(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	// Создать каталог.
	if err = os.MkdirAll(path, os.ModePerm); err != nil {
		return err
	}

	return nil
}

// GetDirectorySize вычисляет размер каталога в байтах.
func GetDirectorySize(path string) (uint64, error) {
	var size int64

	// Вычисление размера каталога.
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Если файл.
		if !info.IsDir() {
			size += info.Size()
		}

		return err
	})

	return uint64(size), err
}
