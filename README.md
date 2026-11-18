# Go URL Shortener & Analytics Server ⚡

A high-performance, thread-safe REST API backend built in **Go (Golang)** using only the standard library. Designed to demonstrate concurrent data handling, custom HTTP middlewares, structured JSON responses, and graceful server shutdown.

## Key Engineering Highlights
- **Zero External Dependencies:** Built purely with Go's standard `net/http` (Go 1.22+ method-based routing).
- **Concurrency & Thread-Safety:** Uses `sync.RWMutex` to prevent race conditions during high-concurrency read/write operations.
- **Custom Middleware Chain:** Includes request latency logging (`loggingMiddleware`) and panic recovery (`recoveryMiddleware`) to ensure zero downtime.
- **Graceful Shutdown:** Listens to OS signals (`SIGINT`, `SIGTERM`) to finish active requests before shutting down cleanly.
- **Real-Time Click Analytics:** Tracks total clicks and last accessed timestamps for every shortened link.

---

## Project Structure

```text
go-url-shortener/
├── go.mod       # Go module definition
├── main.go      # Core server, store engine, middlewares, and handlers
└── README.md    # Project documentation and API reference
