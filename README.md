# fluent-swap-core
This is a core of the application helps you find a native speaker to practice your fluency

## API docs

### Russian

[WebSocket protocol](docs/api/websocket.md)
[Redis architecture](docs/architecture/redis-matchmaking.md)

### Initialization

```bash
docker compose up -d redis
docker compose exec redis redis-cli -a test_pass PING
docker compose down
```

### Integration test

```bash
docker compose up -d --wait redis
docker compose exec redis redis-cli --no-auth-warning -a test_pass PING
go test -race -tags=integration ./internal/features/matchmaking/repository
docker compose down
```
