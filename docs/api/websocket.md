# WebSocket protocol

Документ описывает публичный WebSocket-контракт matchmaking в V1.

## Подключение

URL WebSocket endpoint будет определён при реализации transport в
[Issue #5](https://github.com/wizardVadim/fluent-swap-core/issues/5).

При установке соединения сервер генерирует `client_id` и привязывает его к
соединению. Клиент не передаёт `client_id` в командах.

## Общий формат сообщения

```json
{
  "type": "message_type",
  "request_id": "req-1234",
  "payload": {}
}
```

- `type` — обязательный строковый тип сообщения;
- `request_id` — идентификатор команды, который генерирует клиент; обязателен в
  клиентских командах и ответах на них, кроме ошибки с неразбираемым envelope;
- `payload` — данные конкретного сообщения; присутствует только у типов, которым
  нужны дополнительные данные.

Для `match_found` каждый клиент получает `request_id` собственного
`find_partner`. Сервер генерирует общий `match_id` для найденной пары и отправляет
его обоим участникам.

## Client → Server

### `find_partner`

Клиент отправляет команду, чтобы начать поиск партнёра с зеркальной языковой
парой. Обязательны `type`, `request_id`, `payload.native_language_code` и
`payload.learning_language_code`.

```json
{
  "type": "find_partner",
  "request_id": "req-1234",
  "payload": {
    "native_language_code": "ru",
    "learning_language_code": "en"
  }
}
```

### `cancel_search`

Клиент отправляет команду, чтобы прекратить активный поиск. Обязательны `type` и
`request_id`; `payload` отсутствует.

```json
{
  "type": "cancel_search",
  "request_id": "req-1235"
}
```

Отмена идемпотентна: отсутствие клиента в очереди не считается ошибкой.

## Server → Client

### `search_waiting`

Сервер отправляет ответ, когда пользователь успешно добавлен в очередь, но
совместимый партнёр ещё не найден. Обязательны `type` и `request_id` исходного
`find_partner`; `payload` отсутствует.

```json
{
  "type": "search_waiting",
  "request_id": "req-1234"
}
```

### `search_cancelled`

Сервер подтверждает успешную отмену поиска. Обязательны `type` и `request_id`
исходного `cancel_search`; `payload` отсутствует.

```json
{
  "type": "search_cancelled",
  "request_id": "req-1235"
}
```

### `match_found`

Сервер отправляет событие обоим участникам, когда найдена совместимая пара.
Обязательны `type`, `request_id` соответствующего `find_partner` и
`payload.match_id`. Оба участника получают одинаковый `match_id`, но собственные
`request_id`.

```json
{
  "type": "match_found",
  "request_id": "req-1234",
  "payload": {
    "match_id": "match-5678"
  }
}
```

### `error`

Сервер отправляет сообщение, если не может обработать клиентскую команду.
Обязательны `type`, `payload.code` и `payload.message`. Поле `request_id`
обязательно, если сервер смог извлечь его из envelope, и отсутствует, если JSON
невозможно разобрать.

```json
{
  "type": "error",
  "request_id": "req-1234",
  "payload": {
    "code": "invalid_payload",
    "message": "native language code is invalid"
  }
}
```

## Коды ошибок

| Code | Когда возвращается |
| --- | --- |
| `invalid_json` | Входящее сообщение не является валидным JSON. |
| `unknown_message_type` | Поле `type` содержит неподдерживаемый тип сообщения. |
| `invalid_payload` | Payload отсутствует, имеет неверную структуру или нарушает domain-правила. |
| `internal_server_error` | Сервер не смог выполнить корректную команду из-за внутренней ошибки. |
