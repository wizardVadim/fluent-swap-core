## Общие правила

- `client_id` генерируется сервером при установке WebSocket-соединения и
  привязывается к нему; клиент не передаёт `client_id` в командах.
- `request_id` генерируется клиентом и используется для корреляции команды с
  ответами сервера.
- Для `match_found` каждый клиент получает `request_id` собственного
  `find_partner`.
- `match_id` генерируется сервером при создании матча, одинаков для обоих
  участников и идентифицирует их временную разговорную сессию.
- Transport DTO не являются domain-моделями и преобразуются в них на границе
  transport-слоя.
- `type` хранит наименование типа сообщения
- `payload` содержит данные необходимые для обработки запроса/ответа

## Подключение

EP подключения еще не описан

## Описание документов

### `find_partner` — client → server

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

### `search_waiting` — server → client

```json
{
  "type": "search_waiting",
  "request_id": "req-1234"
}
```

### `cancel_search` — client → server

```json
{
  "type": "cancel_search",
  "request_id": "req-1235"
}
```

### `search_cancelled` — server → client

```json
{
  "type": "search_cancelled",
  "request_id": "req-1235"
}
```

Отмена идемпотентна: отсутствие клиента в очереди не считается ошибкой.

### `match_found` — server → client

```json
{
  "type": "match_found",
  "request_id": "req-1234",
  "payload": {
    "match_id": "match-5678"
  }
}
```

Оба участника получают одинаковый `match_id`, но собственные `request_id`.

### `error` — server → client

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

`request_id` отсутствует, если сервер не смог разобрать JSON и извлечь envelope.

Минимальные коды ошибок: 

example: `"code"` -> `"message"` : `str` -> `str`

`"invalid_json"`          -> `"invalid JSON document"`
`"unknown_message_type"`  -> `"unknown message type"`
`"invalid_payload"`       -> `"native language code is invalid"`
`"internal_server_error"` -> `"internal server error"`