---
description: Правила для бэкенда на Go
globs: "*.go"
always: true
---

# Backend Rules

- **Архитектура**: Handler → Service → Repository. Каждый слой — отдельный пакет.
- **Контекст**: Все запросы к БД и внешним API должны передавать `context.Context`.
- **Обработка JSON**: Использовать стандартный `encoding/json`.
- **Миграции**: Хранить в `migrations/` и применять через `golang-migrate` при старте контейнера.
- **Логирование**: Использовать `log/slog` с уровнями: Debug, Info, Warn, Error.
- **Валидация**: Использовать `go-playground/validator` для структур запросов.
- **JWT**: Хранить в http-only cookies (не в localStorage).
- **Стрик**: Расчет выполнять в cron-воркере, который запускается раз в час.