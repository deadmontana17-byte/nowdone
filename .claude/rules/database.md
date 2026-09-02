---
description: Правила для базы данных
globs: "*.sql"
always: true
---

# Database Rules

- **Таблицы**: Все таблицы имеют поля `id` (UUID) и `created_at` (timestamptz).
- **Индексы**: Создавать индексы на внешние ключи и поля, используемые в WHERE.
- **Миграции**: Все миграции идут в папке `migrations/` с файлами `{timestamp}_name.up.sql` и `{timestamp}_name.down.sql`.
- **Транзакции**: Сервисные методы, изменяющие несколько таблиц, оборачивать в транзакцию.
- **Типы**: Использовать `jsonb` для `description` задач и `content` заметок.