# Ghost Protocol

Real-time cooperative browser network puzzle game built with Go, WebSocket, and vanilla HTML/CSS/JavaScript.

## Gameplay

- Up to 4 operatives per room
- 5-minute lockdown timer
- Three hidden fragments distributed among players
- Three server-validated puzzles
- Server-authoritative game state
- Cooperative chat and real-time state sync
- Six-character escape code generated for each run

## Run locally

```bash
go test ./...
go test -race ./...
go run ./cmd/server
```

Open `http://localhost:8080`.

## Architecture

```text
Browser (HTML/CSS/JS)
        |
     WebSocket
        |
   Go HTTP server
        |
 Room Manager -> Game state
```

The project intentionally uses the Go standard library only. The WebSocket layer is a small educational implementation for this portfolio project.

## Project status

Current prototype: v0.8 — input-based puzzle verification and improved fragment recovery.
