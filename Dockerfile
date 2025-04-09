####################################################################
# Stage 1: Сборка приложения.

# Базовый образ с Go для сборки
FROM golang:1.23.2-alpine AS builder

# Устанавливаем зависимости для сборки с поддержкой CGO
RUN apk update && apk add --no-cache gcc musl-dev

# Включаем CGO
ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=amd64

# Задаём рабочую директорию
WORKDIR /app

# Копируем файлы go.mod и go.sum
COPY go.mod go.sum ./

# Скачиваем зависимости
RUN go mod download

# Копируем остальные файлы приложения
COPY . .

# Сборка приложения
RUN go build -ldflags '-w -s -linkmode external -extldflags "-fno-PIC -static"' -o app cmd/main/main.go

####################################################################
# Stage 2: Формирование итогового образа.

# Финальный образ для запуска
FROM alpine:latest

# Устанавливаем JRE 21
RUN apk update && apk add --no-cache openjdk21-jre

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем скомпилированное приложение из предыдущего контейнера
COPY --from=builder /app/app ./

# Копируем .jar файл
COPY jplag.jar ./

# Делаем исполняемым
RUN chmod +x ./app

# Устанавливаем utf8
ENV LANG=C.UTF-8
ENV LC_ALL=C.UTF-8

# Команда запуска
CMD ["./app"]
