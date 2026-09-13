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
	mu      sync.RWMutex
}

func NewHub(rooms *room.Manager) *Hub { return &Hub{rooms: rooms, clients: make(map[*Client]struct{})} }

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
	p := &room.Player{ID: msg.PlayerID, Name: msg.Name}
	if !r.AddPlayer(p) {
		return errors.New("room is full")
	}
	c.roomID = msg.RoomID
	c.player = p
	h.mu.Lock()
	h.clients[c] = struct{}{}
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
func (h *Hub) remove(c *Client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	if c.roomID != "" && c.player != nil {
		r := h.rooms.GetOrCreate(c.roomID)
		r.RemovePlayer(c.player.ID)
		h.broadcastRoom(c.roomID, Message{Type: "room_state", State: r.Game.State()})
		h.broadcastRoom(c.roomID, Message{Type: "player_left", PlayerID: c.player.ID, Name: c.player.Name})
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
