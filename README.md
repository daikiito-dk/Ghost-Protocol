# Ghost Protocol

> **Three fragments. Three puzzles. One escape code. Five minutes before lockdown.**

Ghost Protocol is a real-time cooperative browser puzzle game built to explore **Go concurrency, WebSocket communication, server-authoritative state, and fault-tolerant multiplayer design**.

Players enter a mysterious network, explore nodes, discover asymmetric fragments, solve server-validated puzzles, and combine their information to unlock the SERVER-CORE before the 5-minute lockdown expires.

## Features

- Up to **4 operatives** per room
- **Solo play** supported
- Real-time communication over WebSocket
- Server-authoritative game state
- Three hidden fragments distributed between players
- Fragment ownership is kept private until discovered
- Three server-validated puzzles
- Six-character escape code generated for each run
- 5-minute server-side lockdown timer
- Cooperative chat and live state synchronization
- Disconnect recovery with a **30-second grace period**
- Browser session preserves player identity for reconnection
- Random fragment/code generation using `crypto/rand`
- Automated tests and race-detector checks
- GitHub Actions CI

## How to Play

1. Join a room with a player name and room ID.
2. Wait for all operatives to become ready.
3. Explore the network nodes.
4. Find your assigned fragment(s) in `NODE-17`.
5. Solve the three puzzles together.
6. Share information through chat — no single player necessarily has everything.
7. Unlock `SERVER-CORE` once all fragments and puzzles are complete.
8. Submit the six-character escape code before the timer reaches zero.
9. Escape before **SYSTEM LOCKDOWN**.

### Network Nodes

| Node | Purpose |
|---|---|
| `NODE-01` | Archive information |
| `NODE-07` | Relay information |
| `NODE-13` | Recovery telemetry |
| `NODE-17` | Fragment discovery |
| `PUZZLE-BLUE` | Sequence puzzle |
| `PUZZLE-RED` | Switch puzzle |
| `PUZZLE-GREEN` | Relay calculation |
| `SERVER-CORE` | Final access point |

The important multiplayer mechanic is **asymmetric information**: fragment values are not exposed in the shared game state. Players must communicate what they discover.

## Tech Stack

- **Go** — HTTP server, game logic, concurrency, timers
- **WebSocket** — real-time bidirectional communication
- **HTML / CSS / JavaScript** — browser client
- **Go standard library only** — no external runtime dependencies
- **GitHub Actions** — automated test/build checks

## Architecture

```text
┌──────────────────────────────┐
│       Browser Client         │
│   HTML / CSS / JavaScript    │
└──────────────┬───────────────┘
               │ WebSocket
               ▼
┌──────────────────────────────┐
│          Go Server            │
│                              │
│  HTTP + WebSocket Hub        │
│          │                   │
│          ▼                   │
│     Room Manager             │
│          │                   │
│          ▼                   │
│     Authoritative Game       │
│          │                   │
│   ┌──────┼─────────┐         │
│   ▼      ▼         ▼         │
│ timer  puzzles  fragments    │
│                              │
│ reconnect / recovery         │
└──────────────────────────────┘
```

The server owns the game state. Clients send intentions such as `explore_node`, `solve_puzzle`, and `submit_code`; the server validates them and broadcasts the resulting state.

## Reconnect & Recovery

Ghost Protocol treats a temporary network failure differently from a permanent player departure.

When an operative disconnects during a game:

- Their player state remains available for **30 seconds**.
- Reconnecting with the same browser session restores the player identity.
- Revealed fragments remain associated with that player.
- If the grace period expires, the player is removed normally.

This makes multiplayer recovery part of the game's server design rather than relying entirely on the browser connection.

## Local Development

### Requirements

- Go 1.22+
- Modern browser with WebSocket support

### Run tests

```bash
go test ./...
```

### Run the race detector

```bash
go test -race ./...
```

### Build

```bash
go build ./cmd/server
```

### Start the server

```bash
go run ./cmd/server
```

Then open:

```text
http://localhost:8080
```

## Project Structure

```text
.
├── cmd/server/          # HTTP server entry point
├── internal/game/       # Authoritative game rules and state
├── internal/room/       # Room and player management
├── internal/websocket/  # WebSocket connection and message hub
├── web/                 # Browser client
├── .github/workflows/   # CI checks
└── go.mod
```

## Development Roadmap

The roadmap is intentionally focused on turning Ghost Protocol from a technical multiplayer prototype into a small, polished cooperative game while continuing to demonstrate progressively stronger Go engineering skills.

### v0.1 — Foundation ✅

- [x] Go HTTP server
- [x] WebSocket communication
- [x] Basic room and player model
- [x] Real-time chat

### v0.2 — Lobby & Synchronization ✅

- [x] Waiting room
- [x] Ready system
- [x] Room state synchronization
- [x] Initial automated tests

### v0.3 — Core Game Loop ✅

- [x] Game state machine
- [x] Fragment system
- [x] SERVER-CORE
- [x] Server-side countdown timer

### v0.4 — Randomized Missions ✅

- [x] Random fragment generation
- [x] Per-player fragment distribution
- [x] Random escape code generation
- [x] Server-side validation

### v0.5 — Multiplayer Resilience ✅

- [x] Disconnect handling
- [x] Fragment ownership recovery
- [x] Stronger game-state tests

### v0.6 — Game Experience ✅

- [x] Mission briefing
- [x] Clear objectives
- [x] Improved terminal-style UI
- [x] Responsive browser layout

### v0.7 — Puzzle Layer ✅

- [x] Three puzzle nodes
- [x] Server-side puzzle validation
- [x] Puzzle progress synchronization
- [x] Private fragment information

### v0.8 — Input & Verification ✅

- [x] Interactive puzzle overlay
- [x] Correct / incorrect answer feedback
- [x] Puzzle-specific tests
- [x] Improved fragment recovery behavior

### v0.9 — Reconnect & Recovery ✅

- [x] Persistent player identity
- [x] Reconnect grace period
- [x] Session recovery
- [x] Automatic client reconnect attempts
- [x] CI workflow

### v1.0 — Playable Release 🚧

**Goal: make the game genuinely fun to play with another person.**

- [ ] Full end-to-end multiplayer playtest
- [ ] Better puzzle variety and procedural puzzle generation
- [ ] More meaningful node exploration
- [ ] Improved win / loss feedback
- [ ] Sound and event feedback polish
- [ ] Better mobile controls
- [ ] Room lifecycle / cleanup
- [ ] Robust WebSocket close and ping/pong handling
- [ ] Expanded integration tests
- [ ] Release documentation

### v1.1 — Production Hardening 🔜

- [ ] Stronger WebSocket protocol handling
- [ ] Connection heartbeat / timeout detection
- [ ] Graceful server shutdown
- [ ] Structured logging
- [ ] Metrics and basic observability
- [ ] Load/concurrency testing
- [ ] Security review of client-supplied input

### v2.0 — Ghost Protocol: Blackout 🔮

A larger version of the concept.

- [ ] Multiple mission layouts
- [ ] Procedurally generated network maps
- [ ] More asymmetric information mechanics
- [ ] Additional puzzle types
- [ ] Persistent statistics / run history
- [ ] Spectator or replay mode
- [ ] Optional public matchmaking

## Engineering Goals

Ghost Protocol is also a learning and portfolio project. Each milestone is designed to demonstrate a different engineering concern:

| Area | What the project demonstrates |
|---|---|
| Go | Interfaces, packages, mutexes, goroutines, channels/timers |
| Networking | HTTP, WebSocket framing, real-time messaging |
| Concurrency | Shared state protection and race detection |
| Multiplayer | Server-authoritative state and synchronization |
| Reliability | Disconnect recovery and session restoration |
| Testing | Unit tests, race detector, CI |
| Frontend | Responsive browser UI without a framework |
| Security thinking | Server-side validation and untrusted client input |

## Design Notes

The project intentionally uses a small custom WebSocket implementation instead of a third-party package. This is primarily educational: understanding the protocol and connection lifecycle is part of the project itself.

It is **not intended to replace a production WebSocket library**. Production hardening is tracked in the v1.1 roadmap.

## License

MIT License
