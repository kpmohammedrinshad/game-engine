# 🎮 Game Engine with User Simulator

A concurrent Go backend that simulates **1000 users** answering a game question simultaneously, evaluates responses in real-time, and declares exactly one winner — with full metrics tracking.

---

## 📁 Project Structure

```
game-engine/
├── main.go                 # Entry point & CLI flags
├── go.mod                  # Module definition
├── engine/
│   ├── engine.go           # Game Engine — winner logic & metrics
│   └── engine_test.go      # Concurrency-safety unit tests
├── api/
│   └── server.go           # HTTP server — POST /submit endpoint
└── simulator/
    └── simulator.go        # Mock User Engine — 1000 concurrent users
```

---

## ⚙️ Components

### 1. Mock User Engine (`simulator/`)
- Spawns **N goroutines** concurrently (default: 1000)
- Each user randomly gets a correct or wrong answer
- Simulates **network lag** with a random delay between 10ms – 1000ms
- Tracks exact count of correct vs incorrect answers sent
- Uses a shared `http.Client` with connection pooling to avoid exhaustion

### 2. API Server (`api/`)
- Exposes `POST /submit` to receive user responses in JSON
- Stamps each response with a receive timestamp
- Forwards to the Game Engine via a **non-blocking channel**
- Returns `202 Accepted` immediately — evaluation is asynchronous
- Also exposes `GET /health` for liveness checks

### 3. Game Engine (`engine/`)
- Evaluates responses in real-time — **no batching**
- Declares the **first correct answer** as the winner using `atomic.CompareAndSwap`
- Tracks correct/incorrect counts using `atomic.Int64`
- Measures and prints **time taken to find the winner**
- Ignores all subsequent correct answers once a winner is found
- Fully **channel-driven** — single drain goroutine, zero lock contention

---

## 🚀 Getting Started

### Prerequisites
- [Go 1.21+](https://golang.org/dl/)

### Clone the Repository

```bash
git clone https://github.com/kpmohammedrinshad/game-engine.git
cd game-engine
```

### Run (Server + Simulator together)

```bash
go run .
```

### Run on a different port (if 8080 is busy)

```bash
go run . -addr :9090
```

### Run with custom user count

```bash
go run . -addr :9090 -n 500
```

---

## 🖥️ Sample Output

```
2026/03/21 18:57:11 [API] Listening on :9090
2026/03/21 18:57:11 [Main] Server ready. Launching 1000 users.
2026/03/21 18:57:11 [Simulator] Spawning 1000 users against http://localhost:9090

╔══════════════════════════════════════════╗
║           🏆  WINNER DECLARED            ║
╠══════════════════════════════════════════╣
║  User ID      : user-00482               ║
║  Answered At  : 18:57:11.023             ║
║  Time to Win  : 13ms                     ║
╠══════════════════════════════════════════╣
║           📊  METRICS SNAPSHOT           ║
╠══════════════════════════════════════════╣
║  ✅ Correct    : 1                        ║
║  ❌ Incorrect  : 0                        ║
║  📨 Total Rcvd : 1                        ║
╚══════════════════════════════════════════╝

2026/03/21 18:57:12 [Simulator] Done. Total=1000  ✅CorrectSent=506  ❌WrongSent=494  OK=1000  Err=0  Duration=1.001s
2026/03/21 18:57:12 [Main] Game over – winner has been declared.
```

---

## 🧪 Running Tests

```bash
# Run unit tests
go test ./engine/... -v

# Run with race detector (requires CGO enabled)
CGO_ENABLED=1 go test ./engine/... -race -v
```

### Test Coverage

| Test | Type | What it verifies |
|---|---|---|
| `TestOnlyOneWinner` | Concurrency unit test | 500 correct + 500 wrong fired concurrently — exactly one winner declared |
| `TestNoWinnerWithAllWrongAnswers` | Edge case unit test | All wrong answers — Done channel never closes |
| `TestMetricsAccuracy` | Metrics unit test | Correct/incorrect counts are accurate under controlled input |

### Expected Test Output

```
=== RUN   TestOnlyOneWinner
🏆  WINNER DECLARED ...
--- PASS: TestOnlyOneWinner (0.00s)
=== RUN   TestNoWinnerWithAllWrongAnswers
--- PASS: TestNoWinnerWithAllWrongAnswers (0.10s)
=== RUN   TestMetricsAccuracy
--- PASS: TestMetricsAccuracy (0.05s)
PASS
ok      game-engine/engine      1.070s
```

---

## 🔧 CLI Flags

| Flag | Default | Description |
|---|---|---|
| `-mode` | `all` | `all` \| `server` \| `simulate` |
| `-addr` | `:8080` | Address the API server listens on |
| `-n` | `1000` | Total number of simulated users |
| `-url` | `http://localhost:8080` | API server base URL (simulate mode) |

---

## 🌐 API Reference

### `POST /submit`

**Request:**
```json
{
  "user_id": "user-00042",
  "correct_answer": true
}
```

**Response — 202 Accepted:**
```json
{
  "status": "accepted",
  "user_id": "user-00042",
  "received_at": "2026-03-21T18:57:11.023Z"
}
```

### `GET /health`

**Response — 200 OK:**
```json
{ "status": "ok" }
```

---

## 🏗️ Concurrency Architecture

```
HTTP Handlers (1000 goroutines)
        │
        │  engine.Submit()  ← non-blocking send
        ▼
  chan Response (buffered, size 2048)
        │
        │  single drain goroutine
        ▼
     evaluate()
        │
        ▼  atomic.CompareAndSwap(0 → 1)
   First correct answer wins
        │
        ▼
   close(done)  ← broadcasts game over to entire system
```

### Concurrency Guarantees

| Requirement | Mechanism |
|---|---|
| Exactly one winner | `atomic.Int32.CompareAndSwap(0→1)` |
| No deadlock | Buffered channel + `select` with `done` guard |
| No race conditions | Single consumer goroutine for evaluation |
| Real-time metrics | `atomic.Int64` counters — no mutex on hot path |
| 1000 concurrent requests | Pooled `http.Client` with `MaxIdleConnsPerHost=N` |

---

## 📦 Dependencies

None — uses **Go standard library only**.

```
net/http       – HTTP server and client
sync           – WaitGroup for simulator
sync/atomic    – Race-free counters and winner flag
encoding/json  – Request/response serialization
math/rand      – Random delays and answer assignment
time           – Timestamps and elapsed time tracking
```

---

## 👤 Author

**Mohammed Rinshad K P**
GitHub: [@kpmohammedrinshad](https://github.com/kpmohammedrinshad)
