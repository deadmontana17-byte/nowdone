# NowDone

Ежедневник с задачами, стриками и Telegram-ботом. Вход только через Telegram
(deep link + одноразовый код), доступ к приложению защищён 4-значным PIN,
бот умеет создавать/менять задачи по тексту и голосу через OpenAI, ночной
воркер считает стрики и рассылает напоминания.

## Стек

- **Frontend**: React 18, TypeScript, Vite, Material UI, Zustand, React Query,
  Framer Motion, PWA (`vite-plugin-pwa`)
- **Backend**: Go 1.22, Gin, PostgreSQL 15 (pgx), golang-migrate
- **Bot**: `go-telegram-bot-api/v5`, OpenAI `gpt-4o-mini` (интенты) + Whisper
  (распознавание голоса)
- **Инфраструктура**: Docker Compose (postgres, backend, bot, worker,
  frontend/Nginx). SSL настраивается на уровне хостинга — Caddy не
  используется.

## Структура репозитория

```
migrations/          # SQL-миграции (golang-migrate, up/down)
backend/
  cmd/api/            # HTTP API (Gin)
  cmd/bot/            # Telegram-бот
  cmd/worker/         # cron: стрики (раз в час) + напоминания (раз в минуту)
  internal/
    config/           # чтение переменных окружения
    db/                # пул соединений + запуск миграций
    models/           # доменные структуры
    repository/       # доступ к БД (pgx)
    service/           # бизнес-логика (auth, tasks, notes, streak, S3, OpenAI)
    handlers/          # HTTP-хендлеры + роутер
    telegram/           # логика бота
    worker/              # напоминания
frontend/
  src/
    api/               # обёртки над fetch к бэкенду
    components/        # переиспользуемые компоненты
    hooks/             # React Query хуки
    pages/             # страницы (Login, PIN, Home, Notes, Settings)
    store/              # Zustand-стейт
  public/
    icons/              # PWA-иконки (192/512, добавить самостоятельно)
    characters/          # PNG-персонажи для прогресс-бара стрика
```

## Быстрый старт (Docker Compose)

1. Скопируйте `.env.example` в `.env` и заполните значения:

   ```bash
   cp .env.example .env
   ```

   Обязательно задайте: `POSTGRES_PASSWORD`, `DB_DSN` (пароль должен совпадать
   с `POSTGRES_PASSWORD`), `JWT_SECRET` (например `openssl rand -hex 32`),
   `TELEGRAM_BOT_TOKEN` и `BOT_USERNAME` (от [@BotFather](https://t.me/BotFather)),
   `SITE_URL` (публичный адрес фронтенда), `OPENAI_API_KEY`, а также S3-реквизиты
   для вложений.

2. Добавьте PWA-иконки в `frontend/public/icons/` (`icon-192.png`,
   `icon-512.png`) и персонажей стрика в `frontend/public/characters/`
   (`char_0.png` … `char_8.png`) — см. README в этих папках.

3. Соберите и запустите все сервисы:

   ```bash
   docker compose up -d --build
   ```

   При старте контейнера `backend` автоматически применяются все миграции из
   `migrations/` через `golang-migrate`.

4. Откройте `http://localhost` (или ваш `SITE_URL`, если он настроен через
   внешний реверс-прокси/хостинг с SSL) и нажмите «Войти через Telegram».

5. Установите вебхук боту не требуется — используется long polling
   (`GetUpdatesChan`), достаточно чтобы контейнер `bot` был запущен и имел
   исходящий доступ к api.telegram.org.

## Переменные окружения

Полный список — в [.env.example](.env.example). Ключевые:

| Переменная              | Назначение                                             |
|--------------------------|---------------------------------------------------------|
| `DB_DSN`                | Строка подключения к PostgreSQL (используется pgx)       |
| `JWT_SECRET`             | Секрет для подписи JWT (сессия, 30 дней, http-only cookie) |
| `TELEGRAM_BOT_TOKEN`     | Токен бота от BotFather                                 |
| `BOT_USERNAME`           | Юзернейм бота без `@`, для диплинков `t.me/<username>?start=...` |
| `OPENAI_API_KEY`         | Ключ для gpt-4o-mini (интенты) и Whisper (голос)          |
| `S3_*`                   | Доступ к S3-совместимому хранилищу вложений                |
| `SITE_URL`               | Публичный адрес фронтенда (CORS)                          |
| `VITE_API_BASE_URL`      | Базовый URL API для фронтенда (по умолчанию `/api`, проксируется Nginx на backend) |

## Локальная разработка без Docker

**Backend:**

```bash
cd backend
go run ./cmd/api      # API на :8080, накатывает миграции сам
go run ./cmd/bot       # Telegram-бот (long polling)
go run ./cmd/worker    # cron: стрики + напоминания
```

Перед запуском экспортируйте переменные из `.env` в окружение (например,
через `direnv` или `export $(cat .env | xargs)` в bash).

**Frontend:**

```bash
cd frontend
npm install
npm run dev            # http://localhost:5173
```

Укажите `VITE_API_BASE_URL=http://localhost:8080` в `frontend/.env.local` для
локальной разработки без Nginx-прокси.

## Основные потоки

- **Вход**: фронт запрашивает код (`POST /auth/login/start`) → пользователь
  переходит по диплинку в бота → бот подтверждает код и присылает его в чат
  (`/start auth_{code}`) → пользователь вводит код на сайте
  (`POST /auth/login/redeem`) → выдаётся JWT в http-only cookie на 30 дней.
- **PIN**: после входа запрашивается установка 4-значного PIN
  (`POST /auth/pin`, bcrypt-хеш). При следующих визитах — ввод PIN
  (`POST /auth/pin/verify`) для разблокировки сессии. «Забыли PIN?» —
  `POST /auth/pin/reset/start` → диплинк `t.me/<bot>?start=reset_{code}` →
  код в чат → `POST /auth/pin/reset/redeem` → `POST /auth/pin/reset/confirm`.
- **Стрик**: воркер каждый час (по MSK, UTC+3) проверяет задачи за вчера —
  если все выполнены, `current_streak++`, иначе сбрасывается в 0; без задач —
  без изменений. Прогресс-бар и статичная PNG-иконка персонажа на фронте
  берутся по `StatusIndex(current_streak)`.
- **Напоминания**: воркер каждую минуту ищет задачи с истёкшим
  `reminder_time` и шлёт сообщение в Telegram с кнопками «✅ Выполнить» /
  «⏱ Отложить».
- **AI в боте**: текст или голос → (для голоса — Whisper-транскрипция) →
  gpt-4o-mini возвращает JSON-интент (`create/update_status/delete/list/reschedule`)
  → бот вызывает соответствующие методы сервиса задач.

## Продакшн-заметки

- SSL терминируется на уровне хостинга/внешнего балансировщика — Nginx во
  фронтенд-контейнере отдаёт только HTTP; поставьте перед ним реверс-прокси
  с TLS (например, управляемый хостингом Certbot/ALB), либо разверните Caddy
  отдельно, если это удобнее вашей инфраструктуре — в docker-compose.yml он
  сознательно не включён по требованиям проекта.
- `COOKIE_DOMAIN` оставьте пустым для host-only cookie, либо укажите домен
  фронтенда, если API и фронтенд на разных поддоменах.
- Секреты никогда не хардкодятся — все читаются из окружения
  (`os.Getenv` в Go, `import.meta.env` во фронтенде).
