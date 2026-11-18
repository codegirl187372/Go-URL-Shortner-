# Go URL Shortener & Analytics API

A high-performance, production-ready URL shortening and analytics REST API built entirely with **Go (Golang)** standard library. Designed with thread-safe concurrency, custom middlewares, and graceful shutdown.

## 🚀 Features

- **Zero External Dependencies:** Built purely using Go's standard library (`net/http`), ensuring lightweight performance and zero dependency vulnerabilities.
- **Thread-Safe In-Memory Storage:** Utilizes `sync.RWMutex` to handle concurrent reads and writes safely without race conditions.
- **Custom Middlewares:** Includes structured request logging and panic recovery middlewares for production stability.
- **Graceful Shutdown:** Handles system interruption signals (`SIGINT`, `SIGTERM`) to process ongoing requests safely before closing.
- **Real-Time Analytics:** Tracks total click counts and last accessed timestamps for every shortened link.

---

## 🛠️ Project Structure

```text
go-url-shortener/
├── go.mod        # Module definition file
├── main.go       # Core server, routing, and storage logic
└── README.md     # Project documentation
