# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Nastene — клон «стены» VK 2007-го в духе ретро. Go + Postgres + server-rendered HTML, без реалтайма. Полный план и мотивация — в `/Users/iamsalnikov/.claude/plans/optimized-jingling-journal.md`.

## Common commands

```bash
task up              # поднять Postgres (podman compose по умолчанию; COMPOSE="docker compose" task up для docker)
task migrate-up      # применить миграции
task run             # запустить HTTP-сервер (по умолчанию :8080)
task test            # go test ./... -race -count=1
task lint            # go vet
task mocks           # сгенерировать моки (mockery, EXPECTER)
task migrate-new -- add_table_x   # новая миграция
go test ./internal/service/auth/... -run TestRegister -v   # один пакет/тест
```

Требуется `.env` — см. `.env.example`.

## Architecture

Слоистая архитектура: **handler → service → repository**. Работа с БД только в репозиториях (см. user rules). Домены и ошибки — в `internal/domain`.

- `cmd/nastene` — entrypoint, `run()` собирает зависимости и запускает `server.Server`.
- `internal/config` — парсит ENV в `Config`. `DATABASE_URL` обязательна.
- `internal/server` — роутер (`go-pkgz/routegroup`), собирает middleware и роуты. Конструктор принимает `Deps` — все зависимости явно.
- `internal/handler/*` — HTTP: парс/рендер/редирект, **без бизнес-логики**. Интерфейсы зависимостей объявлены в пакете `handler`.
- `internal/service/*` — бизнес-логика. Сервисы принимают интерфейсы репозиториев (мокируются через mockery EXPECTER).
- `internal/repository/*` — pgx-запросы. SQL-ошибки оборачиваются в `domain.Err*` через `errors.As/Is`.
- `internal/session` — cookie-сессии в Postgres. `Middleware` кладёт `domain.User` в ctx, `RequireAuth` редиректит гостей на `/login`. Используй `session.FromContext(ctx)`.
- `internal/render` — обёртка `html/template`. Парсит все `pages/*.html` с `layouts/*.html` + `partials/*.html`. `CSRFMiddleware` выдаёт cookie + проверяет `_csrf` или `X-CSRF-Token` на мутирующих методах.
- `web/` — шаблоны и статика, встраиваются через `//go:embed` в `web.FS`.
- `migrations/` — goose `*.sql`, формат `NNNNN_name.sql` с `-- +goose Up/Down`.

### Ключевая абстракция: privacy/ban authorizer

Все решения «кто может смотреть/писать/комментировать стену» централизованы в `internal/service/wall.Authorizer` (будет добавлен на Этапе 3-6). Единая точка правды — не дублируй проверки приватности/банов в хендлерах и других сервисах. Методы: `CanView(ctx, viewerID, ownerID)`, `CanPost(ctx, authorID, ownerID)`, `CanComment(ctx, authorID, ownerID)`. Авторизатор читает дружбы, настройки приватности и бан-лист.

### Friendship model

- `friend_requests(from_id, to_id)` — заявки
- `friendships(user_a, user_b)` с `CHECK (user_a < user_b)` — симметричная дружба одной строкой. Утилиты для нормализации пары лежат в `service/friends`.

### Граффити

Canvas рисуется на клиенте, PNG уходит в `POST /graffiti/upload` (multipart). Файл сохраняется в `$UPLOADS_DIR/graffiti/{id}.png`, путь в `wall_posts.graffiti_path`. Для `kind=graffiti` `body_text` пустой.

## Conventions

- Always wrap errors: `fmt.Errorf("what goes wrong: %w", err)` — см. глобальные Go-правила.
- Sentinel errors только в `internal/domain/errors.go`, не плоди новые динамические ошибки.
- Тесты — black-box (`package xxx_test`), table-style `map[string]struct{...}` с `t.Run(name, ...)`, `require` для критичных проверок.
- Моки — `mockery --all` в `mocks/`, EXPECTER-паттерн `mock.EXPECT()...`.
- Никаких SPA/JS-фреймворков: HTMX для точечной интерактивности + ванильный JS для canvas-граффити.
- Стиль шаблонов — 2007: Verdana/Tahoma 11px, `#4d76a1` синий, блоки с рамкой `#dfe3e8`, никаких анимаций.

## Route map (итого, по мере готовности)

Публичные: `GET /`, `GET|POST /register`, `GET|POST /login`, `POST /logout`, `GET /healthz`, `/static/*`.
Аутентифицированные: `GET /id{n}`, `POST /id{n}/post`, `POST /id{n}/ban`, `POST /id{n}/unban`, `GET /id{n}/friends`, `POST /post/{id}/comment`, `POST /post/{id}/delete`, `POST /comment/{id}/delete`, `GET /friends`, `POST /friends/request/{id}`, `POST /friends/accept/{id}`, `POST /friends/reject/{id}`, `POST /friends/cancel/{id}`, `POST /friends/remove/{id}`, `GET /settings`, `GET|POST /settings/profile`, `GET|POST /settings/privacy/profile`, `GET|POST /settings/privacy/wall`, `GET /settings/bans`, `POST /settings/bans/remove/{id}`, `GET|POST /graffiti/{id}`.

## Что НЕ делать

- Не ходить в БД из хендлеров или сервисов напрямую — только через репозитории.
- Не дублировать privacy/ban-проверки в обход `wall.Authorizer`.
- Не вводить реалтайм (WebSocket/SSE). Это осознанное решение по UX — чувство 2007-го.
- Не добавлять фронтенд-фреймворк и bundler. SSR + HTMX, точка.
