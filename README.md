# Сокращатель ссылок

## Требования задания

- `POST` сохраняет исходный URL и возвращает код сокращённой ссылки. `GET` принимает код и возвращает исходный URL.
- Код уникален и состоит ровно из 10 символов. Допустимы латинские буквы обоих регистров, цифры и `_`.
- Каждому исходному URL соответствует один код. Повторное сокращение того же URL возвращает этот код.
- Сервис поставляется в виде Docker-образа. При запуске выбирается PostgreSQL или собственное хранилище в памяти.
- Функциональность покрыта модульными тестами. Решение предоставляется в публичном репозитории на GitHub.

## Запуск

Все команды запускать из корня проекта.

### Запуск с in-memory хранилищем

```bash
docker build -t url-shortener .
docker run --rm -p 127.0.0.1:8080:8080 url-shortener -storage=memory
```

`memory` используется по умолчанию. После перезапуска сервиса хранилище в памяти пустое.

### Запуск с PostgreSQL

```bash
docker compose up --build -d
```

При первом запуске с пустым томом образ PostgreSQL 16 накатывает [миграцию](internal/storage/postgres/migrations/001_create_links.sql) из `/docker-entrypoint-initdb.d`. Compose ждёт, пока база начнёт принимать TCP-подключения, и запускает API с `-storage=postgres`.

```bash
docker compose down
```

Следующий `docker compose up -d` подхватит данные из именованного тома `postgres_data`.

Для внешней PostgreSQL передайте строку подключения в `DATABASE_URL` и запустите сервис с `-storage=postgres`. Если в базе ещё нет таблицы `links`, сначала накатите миграцию через `psql`.

```bash
psql -d "$DATABASE_URL" -v ON_ERROR_STOP=1 \
  -f internal/storage/postgres/migrations/001_create_links.sql
```

## API

### `POST /links`

```bash
curl -fsS http://127.0.0.1:8080/links \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example.org/articles/go"}'
```

Пример ответа `200 OK`.

```json
{"code":"aB3_45CxY9"}
```

POST принимает абсолютные `http` и `https` URL длиной до 2048 байт. Тело запроса ограничено 64 КиБ.

### `GET /links/{code}`

Подставьте код, полученный в ответе на POST.

```bash
CODE='aB3_45CxY9'
curl -fsS "http://127.0.0.1:8080/links/$CODE"
```

Пример ответа `200 OK`.

```json
{"url":"https://example.org/articles/go"}
```

Сервис возвращает `400` при некорректном JSON, исходном URL или коде, `404` при неизвестном коде, `413` при теле POST больше 64 КиБ, `415` при отсутствующем, некорректном или неподдерживаемом типе контента в POST.

## Структура проекта

```text
cmd/shortener/             # выбор хранилища, запуск и остановка сервера
internal/
├── core/
│   ├── entities/          # энтитя ссылки с которой работаем и проверка формата кода
│   └── links/             # бизнес-логика сокращения ссылки, генерации короткого кода и интерфейс репозитория
├── httpapi/               # хэндлеры
└── storage/
    ├── inmemory/
    └── postgres/
        └── migrations/
```

### Как выдаётся код

`Shorten` ищет URL по исходной строке. Для нового URL сервис генерирует код через `crypto/rand` и пробует сохранить пару. При конфликте он снова ищет исходный URL. Если его уже записал параллельный запрос, сервис возвращает выданный код. Если занят только код, сервис генерирует новый. Всего даётся до трёх попыток вставки.

В памяти проверка уникальности и запись выполняются под одним мьютексом. В PostgreSQL за это отвечают первичный ключ по коду и `UNIQUE` по URL.

## Тесты

Модульные тесты и проверки ручек запускаются командой `go test -race ./...`.

Интеграционные тесты берут строку подключения из `TEST_POSTGRES_DSN`, накатывают миграцию в отдельной схеме и удаляют схему после прогона. В примере базка поднимается во временном контейнере.

```bash
docker run --rm -d --name shortener-test-db \
  -e POSTGRES_PASSWORD=shortener -p 127.0.0.1:5433:5432 postgres:16-alpine
until docker exec shortener-test-db pg_isready -q -h 127.0.0.1 -U postgres; do sleep 1; done

TEST_POSTGRES_DSN='postgres://postgres:shortener@127.0.0.1:5433/postgres?sslmode=disable' \
  go test -tags integration ./internal/storage/postgres

docker stop shortener-test-db
```
