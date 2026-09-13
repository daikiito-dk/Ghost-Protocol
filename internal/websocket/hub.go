package websocket

import (
	"encoding/json"
	"errors"
	"ghost-protocol/internal/game"
	"ghost-protocol/internal/room"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Message struct {
	Type     string `json:"type"`
	PlayerID string `json:"player_id,omitempty"`
	Name     string `json:"name,omitempty"`
	RoomID   string `json:"room_id,omitempty"`
	Message  string `json:"message,omitempty"`
	Ready    *bool  `json:"ready,omitempty"`
	Node     string `json:"node,omitempty"`
	Code     string `json:"code,omitempty"`
	Answer   string `json:"answer,omitempty"`
	State    any    `json:"state,omitempty"`
}

type Client struct {
	conn   *Conn
	roomID string
	player *room.Player
}

type Hub struct {
	rooms   *room.Manager
	clients map[*Client]struct{}
	active  map[string]*Client
	pending map[string]*time.Timer
	mu      sync.RWMutex
}

const reconnectGrace = 30 * time.Second

func NewHub(rooms *room.Manager) *Hub {
	return &Hub{
		rooms:   rooms,
		clients: make(map[*Client]struct{}),
		active:  make(map[string]*Client),
		pending: make(map[string]*time.Timer),
	}
}

func (h *Hub) Handle(w http.ResponseWriter, r *http.Request) {
	conn, err := Upgrade(w, r)
	if err != nil {
		http.Error(w, "WebSocket upgrade failed", 400)
		return
	}
	c := &Client{conn: conn}
	defer func() { h.remove(c); _ = conn.Close() }()
	for {
		data, err := conn.ReadText()
		if err != nil {
			return
		}
		var msg Message
		if err = json.Unmarshal(data, &msg); err != nil {
			h.send(c, Message{Type: "error", Message: "invalid JSON"})
			continue
		}
		switch msg.Type {
		case "join_room":
			if err := h.join(c, msg); err != nil {
				h.send(c, Message{Type: "error", Message: err.Error()})
				continue
			}
			h.send(c, Message{Type: "welcome", Message: "Connection established.", RoomID: c.roomID, State: h.state(c)})
			h.broadcastRoom(c.roomID, Message{Type: "room_state", State: h.state(c)})
		case "chat":
			if c.player == nil {
				continue
			}
			msg.PlayerID = c.player.ID
			msg.Name = c.player.Name
			msg.Message = trimMessage(msg.Message)
			if msg.Message != "" {
				h.broadcastRoom(c.roomID, msg)
			}
		case "ready":
			if c.player == nil || msg.Ready == nil {
				continue
			}
			r := h.rooms.GetOrCreate(c.roomID)
			if !r.Game.SetReady(c.player.ID, *msg.Ready) {
				h.send(c, Message{Type: "error", Message: "cannot change ready state"})
				continue
			}
			h.broadcastRoom(c.roomID, Message{Type: "room_state", State: h.state(c)})
			if r.Game.ReadyToStart() {
				if r.Game.Start() {
					h.broadcastRoom(c.roomID, Message{Type: "game_started", State: h.state(c)})
					go h.runGameLoop(r)
				}
			}
		case "explore_node":
			if c.player == nil {
				continue
			}
			r := h.rooms.GetOrCreate(c.roomID)
			text, ok := r.Game.Explore(c.player.ID, strings.TrimSpace(msg.Node))
			if !ok {
				h.send(c, Message{Type: "error", Message: "cannot explore this node"})
				continue
			}
			h.send(c, Message{Type: "node_result", Node: msg.Node, Message: text, State: h.state(c)})
			h.broadcastRoom(c.roomID, Message{Type: "room_state", State: h.state(c)})
		case "solve_puzzle":
			if c.player == nil {
				continue
			}
			r := h.rooms.GetOrCreate(c.roomID)
			text, ok := r.Game.SolvePuzzle(c.player.ID, strings.TrimSpace(msg.Node), strings.TrimSpace(msg.Answer))
			if !ok {
				h.send(c, Message{Type: "puzzle_error", Node: msg.Node, Message: text, State: h.state(c)})
				continue
			}
			h.send(c, Message{Type: "puzzle_result", Node: msg.Node, Message: text, State: h.state(c)})
			h.broadcastRoom(c.roomID, Message{Type: "room_state", State: h.state(c)})
		case "submit_code":
			if c.player == nil {
				continue
			}
			r := h.rooms.GetOrCreate(c.roomID)
			if r.Game.SubmitCode(c.player.ID, strings.TrimSpace(msg.Code)) {
				h.broadcastRoom(c.roomID, Message{Type: "game_won", Message: "ESCAPE PROTOCOL ACCEPTED", State: h.state(c)})
			} else {
				h.send(c, Message{Type: "error", Message: "invalid escape code"})
			}
		default:
			h.send(c, Message{Type: "error", Message: "unknown message type"})
		}
	}
}

func (h *Hub) join(c *Client, msg Message) error {
	if c.player != nil {
		return errors.New("already joined")
	}
	if msg.PlayerID == "" || msg.Name == "" || msg.RoomID == "" {
		return errors.New("player_id, name and room_id are required")
	}
	if len(msg.Name) > 24 || len(msg.PlayerID) > 64 || len(msg.RoomID) > 16 {
		return errors.New("join request too long")
	}

	r := h.rooms.GetOrCreate(msg.RoomID)
	key := playerKey(msg.RoomID, msg.PlayerID)

	h.mu.Lock()
	if existing := h.active[key]; existing != nil {
		h.mu.Unlock()
		return errors.New("player is already connected")
	}
	if p, ok := r.Player(msg.PlayerID); ok {
		if timer := h.pending[key]; timer != nil {
			timer.Stop()
			delete(h.pending, key)
		}
		c.roomID = msg.RoomID
		c.player = p
		h.clients[c] = struct{}{}
		h.active[key] = c
		h.mu.Unlock()
		h.broadcastRoom(c.roomID, Message{Type: "player_reconnected", PlayerID: p.ID, Name: p.Name})
		return nil
	}
	h.mu.Unlock()

	p := &room.Player{ID: msg.PlayerID, Name: msg.Name}
	if !r.AddPlayer(p) {
		return errors.New("room is full")
	}

	h.mu.Lock()
	c.roomID = msg.RoomID
	c.player = p
	h.clients[c] = struct{}{}
	h.active[key] = c
	h.mu.Unlock()
	return nil
}

func (h *Hub) state(c *Client) any { return h.rooms.GetOrCreate(c.roomID).Game.State() }

func trimMessage(s string) string {
	if len(s) > 500 {
		s = s[:500]
	}
	return s
}

func playerKey(roomID, playerID string) string { return roomID + "\x00" + playerID }

func (h *Hub) remove(c *Client) {
	if c.roomID == "" || c.player == nil {
		return
	}

	key := playerKey(c.roomID, c.player.ID)
	h.mu.Lock()
	delete(h.clients, c)
	if h.active[key] == c {
		delete(h.active, key)
	}
	if old := h.pending[key]; old != nil {
		old.Stop()
	}
	h.pending[key] = time.AfterFunc(reconnectGrace, func() {
		h.expirePlayer(c.roomID, c.player.ID)
	})
	h.mu.Unlock()

	h.broadcastRoom(c.roomID, Message{Type: "player_disconnected", PlayerID: c.player.ID, Name: c.player.Name, Message: "reconnect window: 30s"})
}

func (h *Hub) expirePlayer(roomID, playerID string) {
	key := playerKey(roomID, playerID)
	h.mu.Lock()
	if _, connected := h.active[key]; connected {
		h.mu.Unlock()
		return
	}
	delete(h.pending, key)
	h.mu.Unlock()

	r := h.rooms.GetOrCreate(roomID)
	if p, ok := r.Player(playerID); ok {
		r.RemovePlayer(playerID)
		h.broadcastRoom(roomID, Message{Type: "room_state", State: r.Game.State()})
		h.broadcastRoom(roomID, Message{Type: "player_left", PlayerID: p.ID, Name: p.Name})
	}
}

func (h *Hub) send(c *Client, msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	_ = c.conn.WriteText(data)
}

func (h *Hub) broadcastRoom(roomID string, msg Message) {
	h.mu.RLock()
	clients := make([]*Client, 0)
	for c := range h.clients {
		if c.roomID == roomID {
			clients = append(clients, c)
		}
	}
	h.mu.RUnlock()
	for _, c := range clients {
		h.send(c, msg)
	}
}

func (h *Hub) runGameLoop(r *room.Room) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for range ticker.C {
		status := r.Game.Tick()
		h.broadcastRoom(r.ID, Message{Type: "game_tick", State: r.Game.State()})
		if status == game.Lost {
			h.broadcastRoom(r.ID, Message{Type: "game_lost", Message: "SYSTEM LOCKDOWN — TIME EXPIRED", State: r.Game.State()})
			return
		}
		if status == game.Won {
			return
		}
	}
}
