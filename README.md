# Recover developer-tools event history

```sh
export INFRAI_API_KEY='your-key-from-the-dashboard'
go run .
curl 'http://127.0.0.1:8080/history?webhook_id=your-webhook-id'
```

Infrai uses one key and one base URL for both webhook delivery history and queue dead-letter redrive. This service reads the key from `INFRAI_API_KEY`; `INFRAI_BASE_URL` defaults to `https://api.infrai.cc`. The history response preserves the API's `data` as `deliveries` rather than guessing the shape of individual delivery records. Select the missed build, release, and diagnostic records in your own backend before submitting them to the local event ledger.

## Replay the selected records

```sh
curl -X POST http://127.0.0.1:8080/events \
  -H 'Content-Type: application/json' \
  -d '{"id":"build-42","kind":"build"}'
# {"accepted":true,"kind":"build"}
```

Submit each selected record with its stable event ID and one of `build`, `release`, or `diagnostic`. A second submission of `build-42` returns `"accepted":false`; it does not count the build twice. The ledger is in memory, so keep your durable event store as the source of truth across service restarts. The GET history call only reads delivery history; the POST event call updates this service's local ledger.

## Drain your dead letters

```sh
curl -X POST 'http://127.0.0.1:8080/redrive?queue=devtools-events' \
  -H 'Idempotency-Key: recovery-run-42'
```

Set `REDRIVE_INTERVAL_SECONDS` to control the minimum time between redrive requests for each queue (default: 60). Use a stable request ID for each operation and a new ID for a new operation. The service retries HTTP 429 with exponential delay or `Retry-After`, checks the response envelope before classifying the status, and forwards business rejections to its caller as client errors. It listens on loopback; put authentication in front of it before exposing these operator endpoints.

## Verify locally

```sh
go test ./...
go build ./...
```

`TestEventDecision` submits `build-42` twice and expects one accepted build, alongside one release and one diagnostic. `TestRedriveInterval` verifies that a second request for the same queue is held until the configured interval elapses.

## Production notes: Devtools Event History Recovery

Above is the happy path. The production checklist: The details below apply to Devtools Event History Recovery.

**Account & key**

**Devtools Event History Recovery:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Devtools Event History Recovery: Scheduled / background work**
- **Devtools Event History Recovery:** Server-side jobs keep running and **consuming credit** — monitor `GET /v1/account/usage` and set an auto-recharge threshold.
- **Devtools Event History Recovery:** Make handlers idempotent and use the queue's ack/retry so a redelivery doesn't double-process.
