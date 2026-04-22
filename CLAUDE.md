# CLAUDE.md — Cookie Clicker (Go + WebSockets)

Authoritative reference for all implementation, extension, and testing work on this project.
Every rule here is a hard requirement. Read it fully before touching any file.

---

## 1. Project Overview

A browser-based Cookie Clicker game where the Go server is the single source of truth for
all game state. The client is a dumb terminal: it renders what the server sends and forwards
only user intent (clicks, purchases). No game logic lives in the browser.

**Stack**
- Backend: Go 1.22+, `github.com/gorilla/websocket`
- Frontend: single static HTML file (HTML + CSS + JS inline, no build step, no framework)
- Module: `github.com/msenart/cookieclicker`

---

## 2. Standard Go Project Layout

Follows [golang-standards/project-layout](https://github.com/golang-standards/project-layout) conventions.

```
cookieclicker/
├── CLAUDE.md
├── go.mod
├── go.sum
├── cmd/
│   └── cookieclicker/
│       └── main.go          ← entry point only: wire deps, start server, handle signals
├── internal/
│   ├── config/
│   │   └── config.go        ← ALL tunable constants; zero hardcoded values elsewhere
│   ├── game/
│   │   ├── state.go         ← GameState, cookie arithmetic, mutex guards
│   │   ├── ticker.go        ← server-side passive income tick loop
│   │   ├── upgrades.go      ← Upgrade definitions, purchase validation, CPS effects
│   │   ├── state_test.go
│   │   ├── ticker_test.go
│   │   └── upgrades_test.go
│   └── server/
│       ├── handler.go       ← WS upgrader, Hub, per-session goroutines, message dispatch
│       ├── ratelimit.go     ← sliding-window rate limiter (per session)
│       ├── auth.go          ← HMAC session token: issue + validate
│       ├── handler_test.go
│       ├── ratelimit_test.go
│       └── auth_test.go
└── web/
    └── static/
        └── index.html       ← complete frontend; no external dependencies
```

**Package responsibilities**

| Path | Responsibility |
|------|---------------|
| `cmd/cookieclicker/main.go` | `http.ListenAndServe`, route registration, graceful shutdown on SIGINT/SIGTERM |
| `internal/config/config.go` | Single exported `Config` struct + `Default()`. Every numeric/string constant lives here. |
| `internal/game/state.go` | `GameState` owns cookies, CPS, upgrades map. Thread-safe via embedded `sync.Mutex`. Serialises to `StateSnapshot`. |
| `internal/game/ticker.go` | `Ticker` runs a `time.Ticker` at `cfg.TickRate`, calls `GameState.ApplyTick()` each interval. |
| `internal/game/upgrades.go` | `Upgrade` type, purchase validation, sentinel errors, CPS delta application. |
| `internal/server/handler.go` | `Hub` (all sessions), `Session` (per-connection: seq counter, rate limiter, game state, send channel). |
| `internal/server/ratelimit.go` | `RateLimiter` sliding-window implementation. `Allow(sessionID)` returns bool. |
| `internal/server/auth.go` | `TokenService`: `IssueToken`, `ValidateToken` using HMAC-SHA256. |
| `web/static/index.html` | Connects to `/ws`, renders server pushes, sends `click`/`buy_upgrade` JSON. |

**Dependency direction (no circular imports)**
```
internal/config  ← (no internal deps)
internal/game    ← internal/config
internal/server  ← internal/config, internal/game
cmd/...          ← internal/config, internal/game, internal/server
```

`internal/game` must NOT import `internal/server`. `internal/server` must NOT import `cmd/`.

---

## 3. Coding Standards

### 3.1 Doc comments on every export

Every exported identifier (function, type, method, constant, variable) MUST have a Go doc
comment beginning with the identifier name. No exceptions.

```go
// Config holds all tunable parameters for the game server.
// Modify Default() to change game balance without touching logic files.
type Config struct { ... }

// TickRate is the duration between passive-income server ticks.
const TickRate = 100 * time.Millisecond
```

### 3.2 No hardcoded values in logic

Every numeric or string constant that affects game behaviour belongs in `internal/config/config.go`.
Logic files receive a `*config.Config` at construction — they never declare their own magic numbers.

Values that MUST live in config (non-exhaustive):
```
ListenAddr           string        // ":8080"
TickRate             time.Duration // 100ms
BaseCookiesPerClick  float64       // 1.0
MaxClicksPerSecond   int           // 20
RateLimitWindow      time.Duration // 1s
AnomalyThreshold     int           // 5
SequenceWindowSize   uint64        // 10
HMACKeyEnvVar        string        // "SESSION_SECRET"
HMACMinKeyLen        int           // 32
Upgrades             []UpgradeConfig
```

### 3.3 Error handling

Never ignore errors. Wrap with `fmt.Errorf("context: %w", err)`. Every `err != nil` case must
either return, log with context, or explicitly comment why discarding is safe.

### 3.4 Concurrency

`GameState` is shared across the tick goroutine and WS handler goroutine. Protect all reads
and writes with the embedded `sync.Mutex`. Use `defer mu.Unlock()` consistently.
Never hold the lock across an I/O call (WS write, log write).

### 3.5 Goroutine hygiene

Every goroutine must have a documented shutdown path. Every goroutine launched in production
code must have a corresponding cancel/stop mechanism exercised in tests.

### 3.6 Import order

Standard library, then third-party, then internal — each group separated by a blank line.

---

## 4. Testing Standards

**Rule: test alongside code.** Every feature ships with its test. A function with no test
does not exist as far as this project is concerned.

- Use `package X_test` (external) for integration-style tests.
- Use `package X` (white-box) when internal access is needed.
- Always run with the race detector: `go test -race ./...`
- Coverage target: 80% minimum across `internal/game/` and `internal/server/`.
- **No `time.Sleep` in tests.** Use channels or `time.After` with `t.Fatal` on timeout.

### game/ unit tests

| Test | Verifies |
|------|----------|
| `TestNewGameState` | upgrade map length correct, no upgrade owned, cookies = 0 |
| `TestAddCookies` | sequential adds sum correctly |
| `TestAddCookies_Race` | concurrent adds are race-free (`-race`) |
| `TestApplyTick` | one tick adds exactly `CPS * TickInterval` cookies |
| `TestSnapshot_IsCopy` | mutating snapshot does not affect game state |
| `TestPurchase_Success` | cost deducted, owned=true, CPS increased |
| `TestPurchase_InsufficientCookies` | returns `ErrInsufficientCookies`, state unchanged |
| `TestPurchase_AlreadyOwned` | returns `ErrAlreadyOwned` on second purchase |
| `TestPurchase_UnknownID` | returns `ErrUpgradeNotFound` |
| `TestPurchase_Race` | concurrent purchase of same upgrade — exactly one succeeds |
| `TestTicker_TicksApplied` | cookies grow after N ticks (channel notification, no sleep) |
| `TestTicker_StopsCleanly` | no ticks arrive after `Stop()` (timeout assertion) |
| `TestTicker_NoCookiesWithZeroCPS` | cookies remain 0 when CPS = 0 |

### server/ integration tests (`httptest.NewServer` + WS dial)

| Test | Verifies |
|------|----------|
| `TestHandshake_ValidToken` | WS upgrade succeeds, first state push received |
| `TestHandshake_InvalidToken` | HTTP 401 before upgrade |
| `TestHandshake_MissingToken` | HTTP 401 before upgrade |
| `TestClick_AccumulatesCookies` | click → next state shows +1 cookie |
| `TestClick_RateLimitEnforced` | burst > MaxClicksPerSecond → capped gain (anti-cheat) |
| `TestClick_DuplicateSeq` | same seq twice → second dropped, no extra cookie |
| `TestClick_OutOfOrderSeq` | seq 1,2,4 (skip 3) → 4 dropped |
| `TestClick_FarAheadSeq` | seq jumps > SequenceWindowSize → close 1008 |
| `TestBuyUpgrade_Success` | upgrade owned, CPS updated in next state push |
| `TestBuyUpgrade_InsufficientCookies` | error push received, state unchanged |
| `TestBuyUpgrade_UnknownID` | error push received |
| `TestBuyUpgrade_AlreadyOwned` | error push received |
| `TestAnomalyFlag` | excess clicks above AnomalyThreshold → session flagged in logs |
| `TestStatePush_OnTick` | state push arrives within 2× TickRate |
| `TestTokenForgery` | tampered HMAC signature → HTTP 401 |
| `TestReplayAttack` | valid click replayed 10× with same seq → only 1 cookie added |

---

## 5. Anti-Cheat Architecture

### Principle: server is the only source of truth

The client sends only user intent. The server ignores any field not in the schema below.
The server closes the connection with code 1008 on malformed or policy-violating messages.

### WebSocket message protocol

**Client → Server**
```json
{ "type": "click", "seq": 42 }
{ "type": "buy_upgrade", "id": "cursor", "seq": 43 }
```

**Server → Client**
```json
{ "type": "state", "cookies": 1523.75, "cps": 8.0, "upgrades": [...], "tick": 1234567890123 }
{ "type": "error", "code": "insufficient_cookies", "message": "need 100 cookies" }
{ "type": "error", "code": "invalid_seq", "message": "sequence mismatch" }
{ "type": "error", "code": "unknown_upgrade", "message": "upgrade not found" }
```

State pushes are sent: after every accepted click, after every accepted purchase, after every
server tick that produces a state change.

### Sequence numbers

Server tracks `lastAcceptedSeq uint64` per session (starts at 0).
- `msg.Seq <= lastAcceptedSeq` → drop silently (replay/duplicate)
- `msg.Seq > lastAcceptedSeq + cfg.SequenceWindowSize` → close 1008 (injection attempt)
- Otherwise: accept, set `lastAcceptedSeq = msg.Seq`

### Rate limiting

Sliding window per session. Window = `cfg.RateLimitWindow`, max = `cfg.MaxClicksPerSecond`.
Excess clicks are **silently dropped** (no error response — prevents timing side-channel confirmation).
After `cfg.AnomalyThreshold` excess drops in one window, server logs a warning with the session ID.

### Session authentication

1. Client calls `GET /session` → server issues signed token:
   `base64url(sessionID + "." + hex(HMAC-SHA256(sessionID, secret)))`
2. Client passes token as query param: `ws://host/ws?token=<token>`
3. Server validates HMAC before upgrading. Invalid → HTTP 401.
4. `SESSION_SECRET` env var (named by `cfg.HMACKeyEnvVar`) must be present and ≥ `cfg.HMACMinKeyLen` bytes.
   Server **panics at startup** if missing or too short.

---

## 6. How to Run, Build, Test

```bash
# Install deps
go mod tidy

# Required env var
export SESSION_SECRET="replace-with-32-plus-char-random-string"

# Run dev server (http://localhost:8080)
go run ./cmd/cookieclicker/

# Build binary
go build -o cookieclicker ./cmd/cookieclicker/

# All tests with race detector
go test -race ./...

# Coverage report
go test -coverprofile=cover.out ./... && go tool cover -html=cover.out

# Format + vet
gofmt -w .
go vet ./...
```

---

## 7. Hard Constraints

1. No JS framework, bundler, or npm dependency. One HTML file (`web/static/index.html`), full stop.
2. `cmd/cookieclicker/main.go` is wiring only — no game logic.
3. No numeric constants outside `internal/config/config.go`.
4. No database. State is in-memory; server restart resets the game.
5. Every goroutine must have a documented shutdown path exercised in a test.
6. No `time.Sleep` in tests. Use channels or `time.After` + `t.Fatal` on timeout.
7. `web/static/index.html` must work via `file://` and over HTTP (no hard-coded origin).
8. `internal/game` must not import `internal/server`. `internal/server` must not import `cmd/`.
