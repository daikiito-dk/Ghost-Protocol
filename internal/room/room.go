package room

import (
	"ghost-protocol/internal/game"
	"sync"
)

type Room struct {
	ID      string
	Players map[string]*Player
	Game    *game.Game
	mu      sync.RWMutex
}
type Player struct {
	ID   string
	Name string
}

func New(id string) *Room { return &Room{ID: id, Players: make(map[string]*Player), Game: game.New()} }
func (r *Room) AddPlayer(p *Player) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.Players) >= 4 {
		return false
	}
	r.Players[p.ID] = p
	r.Game.AddPlayer(p.ID, p.Name)
	return true
}
func (r *Room) RemovePlayer(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.Players, id)
	r.Game.RemovePlayer(id)
}
func (r *Room) Snapshot() []*Player {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Player, 0, len(r.Players))
	for _, p := range r.Players {
		c := *p
		out = append(out, &c)
	}
	return out
}

type Manager struct {
	rooms map[string]*Room
	mu    sync.RWMutex
}

func NewManager() *Manager { return &Manager{rooms: make(map[string]*Room)} }
func (m *Manager) GetOrCreate(id string) *Room {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r, ok := m.rooms[id]; ok {
		return r
	}
	r := New(id)
	m.rooms[id] = r
	return r
}
