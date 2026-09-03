# GoShort

URL shortener built around one constraint: `GET /:code` answers before anything else happens.
The redirect reads from Redis on a warm cache, replies 302, and only then pushes the click event
onto a buffered channel that a fixed worker pool drains in batches. If that channel is full the
event is dropped and counted rather than allowed to block the person being redirected.

**Go · Fiber · GORM · PostgreSQL · Redis · Prometheus · React · Docker**

## Run it

```bash
cp .env.example .env
docker compose up
```

Then `curl localhost:8080/health`.

## Tests

```bash
docker compose up -d postgres redis
TEST_DATABASE_URL="postgres://goshort:goshort@localhost:5432/goshort?sslmode=disable" \
  go test -race -cover ./...
```

Integration tests skip themselves when `TEST_DATABASE_URL` is unset.

## Status

Work in progress. Benchmark numbers go here once they have actually been measured with k6 —
nothing in this README is an estimate.
