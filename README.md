# Posiflora MVP — Telegram интеграция

MVP интеграции Telegram-бота для магазинов Posiflora: уведомления о новых заказах через Telegram.

![Скриншот](screenshot.png)

## Быстрый старт

```bash
docker compose up --build
```

После запуска:
- **Frontend**: http://localhost:3000
- **Backend API**: доступен через прокси на `:3000/shops/...`

## Как запустить backend

### Через Docker (рекомендуется)

```bash
docker compose up --build
```

Поднимаются 3 контейнера: PostgreSQL, backend (Go), frontend (nginx).

### Без Docker

Требования: Go 1.23+, PostgreSQL 16+

```bash
# Создать БД и пользователя
createuser -s posiflora
createdb -O posiflora posiflora

# Установить зависимости
go mod tidy

# Запустить
DATABASE_URL="postgres://posiflora:posiflora@localhost:5432/posiflora?sslmode=disable" \
TELEGRAM_REAL=true \
go run ./cmd/server
```

Backend слушает `:8080`.

## Как запустить frontend

### Через Docker
Фронтенд поднимается вместе с `docker compose up` на порту `:3000`.

### Без Docker (разработка)

```bash
cd frontend
npm install
npm run dev
```

Фронтенд на `:3000`, API-запросы проксируются на `:8080` (настроено в `vite.config.ts`).

## Как сидятся тестовые данные

Данные создаются автоматически при первом запуске backend (`cmd/server/main.go` → `seedData`):

- **Магазин #1**: «Магазин цветов Роза»
- **8 тестовых заказов**: A-1001 — A-1008
- **Telegram-интеграция**: настраивается через UI (кнопка «Настроить» в шапке)

Повторный запуск не дублирует данные (проверяется `SELECT COUNT(*) FROM shops`).

## Как прогнать тесты

```bash
go test -v ./...
```

Тесты используют SQLite in-memory (не требуют PostgreSQL). Покрытие:

1. **TestCreateOrder_EnabledIntegration_SendsTelegram** — заказ + включённая интеграция → TelegramClient вызван, лог SENT
2. **TestCreateOrder_Idempotency_NoDuplicateSend** — повторная отправка не создаёт дублей в telegram_send_log
3. **TestCreateOrder_TelegramFails_OrderCreated** — ошибка TelegramClient → лог FAILED, заказ всё равно создан
4. **TestCreateOrder_Validation_EmptyFields** — пустые number/total/customerName → 400, заказ не создаётся

## Реальная Telegram-отправка или мок-режим

| Режим | Переменная окружения | Поведение |
|---|---|---|
| **Мок** (по умолчанию) | `TELEGRAM_REAL` не задана | Сообщения не отправляются, вызовы логируются |
| **Реальная отправка** | `TELEGRAM_REAL=true` | Вызывается Telegram Bot API `sendMessage` |

В `docker-compose.yml` реальная отправка включена по умолчанию.

## API

| Endpoint | Method | Описание |
|---|---|---|
| `/shops/{shopId}/telegram/connect` | POST | Подключить/обновить Telegram бота (upsert) |
| `/shops/{shopId}/orders` | POST | Создать заказ (валидация + уведомление в Telegram) |
| `/shops/{shopId}/telegram/status` | GET | Статус интеграции (enabled, lastSentAt, counts за 7 дней) |
| `/shops/{shopId}/telegram/log` | GET | Лог отправок (последние 50 записей, все заказы) |

### Примеры (curl)

```bash
# Подключить бота
curl -X POST http://localhost:3000/shops/1/telegram/connect \
  -H "Content-Type: application/json" \
  -d '{"botToken":"YOUR_TOKEN","chatId":"YOUR_CHAT_ID","enabled":true}'

# Создать заказ
curl -X POST http://localhost:3000/shops/1/orders \
  -H "Content-Type: application/json" \
  -d '{"number":"A-1010","total":1500,"customerName":"Иван"}'

# Статус
curl http://localhost:3000/shops/1/telegram/status

# Лог отправок
curl http://localhost:3000/shops/1/telegram/log
```

## Архитектура

```
handler (HTTP) → service (бизнес-логика) → repository (PostgreSQL)
                                            → telegram.Client (интерфейс)
```

- **Идемпотентность**: `UNIQUE(shop_id, order_id)` в `telegram_send_log` + `ON CONFLICT DO NOTHING`
- **Отказоустойчивость**: ошибка Telegram не блокирует создание заказа
- **Валидация**: number, total > 0, customerName обязательны (400 на пустые поля)
- **Интерфейс `TelegramClient`**: `RealClient` (API) и `MockClient` (тесты)

## Frontend

- React + TypeScript + Vite
- UI: shadcn/ui + Tailwind CSS v4
- Один экран: статус интеграции, создание заказа, лог отправок
- Настройки бота скрыты за кнопкой «Настроить» в шапке
- Тост-уведомления при создании заказа (sonner)

## Список допущений и упрощений

1. Один shop_id = одна интеграция (`UNIQUE(shop_id)`)
2. Аутентификация/авторизация отсутствует (MVP)
3. Bot token хранится в открытом виде в БД (в проде — шифрование или vault)
4. Chat ID маскируется в API (`первые 3 символа + ***`)
5. Отправка синхронная (в проде — очередь/воркер с ретраями)
6. Frontend захардкожен на `shop_id=1` (MVP)
7. Нет миграций как отдельного инструмента — схема создаётся при старте (`CREATE TABLE IF NOT EXISTS`)
