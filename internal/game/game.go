package game

import (
	"crypto/rand"
	"fmt"
	"sort"
	"strings"
	"sync"
)

type Status string

const (
	Waiting Status = "waiting"
	Playing Status = "playing"
	Won     Status = "won"
	Lost    Status = "lost"
)

type PlayerInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Ready bool   `json:"ready"`
}
type Puzzle struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Prompt string `json:"prompt"`
	Solved bool   `json:"solved"`
}
type Node struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Hint   string `json:"hint,omitempty"`
	Locked bool   `json:"locked"`
	Type   string `json:"type,omitempty"`
}
type State struct {
	Status         Status       `json:"status"`
	RemainingTime  int          `json:"remaining_time"`
	Players        []PlayerInfo `json:"players"`
	Nodes          []Node       `json:"nodes"`
	Discovered     []string     `json:"discovered"`
	FragmentsFound int          `json:"fragments_found"`
	FragmentTotal  int          `json:"fragment_total"`
	Objective      string       `json:"objective"`
	CoreReady      bool         `json:"core_ready"`
	PuzzleProgress int          `json:"puzzle_progress"`
	PuzzleTotal    int          `json:"puzzle_total"`
	Puzzles        []Puzzle     `json:"puzzles"`
}
type player struct {
	PlayerInfo
	slots []int
}
type Game struct {
	mu            sync.RWMutex
	status        Status
	remaining     int
	players       map[string]*player
	discovered    map[string]bool
	fragments     [3]string
	revealed      [3]bool
	startedOnce   bool
	puzzleSolved  [3]bool
	puzzleAnswers [3]string
}

func New() *Game {
	return &Game{status: Waiting, remaining: 300, players: make(map[string]*player), discovered: make(map[string]bool)}
}
func (g *Game) AddPlayer(id, name string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if id == "" || name == "" {
		return false
	}
	if _, ok := g.players[id]; ok {
		return false
	}
	g.players[id] = &player{PlayerInfo: PlayerInfo{ID: id, Name: name}}
	return true
}
func (g *Game) HasPlayer(id string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	_, ok := g.players[id]
	return ok
}
func (g *Game) RemovePlayer(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	p, ok := g.players[id]
	if !ok {
		return
	}
	orphans := append([]int(nil), p.slots...)
	delete(g.players, id)
	if g.status == Playing {
		// Keep existing fragment ownership stable. Only unrevealed orphaned
		// slots are handed to remaining operatives.
		for _, slot := range orphans {
			if slot < 0 || slot >= len(g.revealed) || g.revealed[slot] {
				continue
			}
			if target := g.firstPlayerWithoutSlot(); target != nil {
				target.slots = append(target.slots, slot)
			}
		}
	}
}
func (g *Game) SetReady(id string, ready bool) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	p, ok := g.players[id]
	if !ok || g.status != Waiting {
		return false
	}
	p.Ready = ready
	return true
}
func (g *Game) ReadyToStart() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if len(g.players) == 0 || g.startedOnce {
		return false
	}
	for _, p := range g.players {
		if !p.Ready {
			return false
		}
	}
	return true
}
func (g *Game) Start() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.status != Waiting || g.startedOnce || len(g.players) == 0 {
		return false
	}
	for _, p := range g.players {
		if !p.Ready {
			return false
		}
	}
	g.status = Playing
	g.remaining = 300
	g.startedOnce = true
	g.assignFragments()
	g.assignPuzzleAnswers()
	return true
}
func (g *Game) assignFragments() {
	ids := g.sortedIDs()
	code := make([]byte, 6)
	alphabet := []byte("ABCDEFGHJKLMNPQRSTUVWXYZ23456789")
	for i := range code {
		var b [1]byte
		if _, err := rand.Read(b[:]); err != nil {
			code[i] = alphabet[(i*7+3)%len(alphabet)]
		} else {
			code[i] = alphabet[int(b[0])%len(alphabet)]
		}
	}
	for i := range g.fragments {
		g.fragments[i] = string(code[i*2 : i*2+2])
		g.revealed[i] = false
		g.puzzleSolved[i] = false
	}
	g.assignSlots(ids)
}
func (g *Game) assignPuzzleAnswers() {
	g.puzzleAnswers = [3]string{"10", "2", "9"}
}
func (g *Game) sortedIDs() []string {
	ids := make([]string, 0, len(g.players))
	for id := range g.players {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
func (g *Game) assignSlots(ids []string) {
	for _, p := range g.players {
		p.slots = nil
	}
	if len(ids) == 0 {
		return
	}
	for i, id := range ids {
		for slot := i; slot < 3; slot += len(ids) {
			g.players[id].slots = append(g.players[id].slots, slot)
		}
	}
}
func (g *Game) firstPlayerWithoutSlot() *player {
	var target *player
	for _, p := range g.players {
		if target == nil || len(p.slots) < len(target.slots) || (len(p.slots) == len(target.slots) && p.ID < target.ID) {
			target = p
		}
	}
	return target
}

func (g *Game) rebalanceSlots() { g.assignSlots(g.sortedIDs()) }
func (g *Game) Tick() Status {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.status != Playing {
		return g.status
	}
	if g.remaining > 0 {
		g.remaining--
	}
	if g.remaining == 0 {
		g.status = Lost
	}
	return g.status
}

func (g *Game) Explore(id, node string) (string, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.status != Playing {
		return "", false
	}
	p, ok := g.players[id]
	if !ok {
		return "", false
	}
	node = strings.ToUpper(strings.TrimSpace(node))
	switch node {
	case "NODE-01":
		g.discovered[node] = true
		return "ARCHIVE: the protocol has three fragments and three verification puzzles. Search, compare, and transmit what you learn.", true
	case "NODE-07":
		g.discovered[node] = true
		return "RELAY: fragments are distributed across operatives. Puzzle answers reveal the ordering rules.", true
	case "NODE-13":
		g.discovered[node] = true
		return fmt.Sprintf("LOGS: %d/%d fragments recovered. Verification progress: %d/%d.", g.fragmentCount(), len(g.fragments), g.puzzleCount(), 3), true
	case "NODE-17":
		g.discovered[node] = true
		if len(p.slots) == 0 {
			return "FRAGMENT: no fragment assigned.", true
		}
		parts := make([]string, 0, len(p.slots))
		for _, slot := range p.slots {
			g.revealed[slot] = true
			parts = append(parts, fmt.Sprintf("FRAGMENT %d: %s", slot+1, g.fragments[slot]))
		}
		return "PASSWORD DATA: " + strings.Join(parts, " | "), true
	case "PUZZLE-BLUE", "PUZZLE-RED", "PUZZLE-GREEN":
		g.discovered[node] = true
		return g.puzzlePrompt(node), true
	case "SERVER-CORE":
		if g.fragmentCount() < len(g.fragments) || g.puzzleCount() < 3 {
			return fmt.Sprintf("ACCESS DENIED: fragments %d/%d, puzzles %d/3.", g.fragmentCount(), len(g.fragments), g.puzzleCount()), true
		}
		g.discovered[node] = true
		return "ESCAPE TERMINAL READY — submit the six-character code.", true
	default:
		return "", false
	}
}
func (g *Game) puzzlePrompt(node string) string {
	switch node {
	case "PUZZLE-BLUE":
		return "BLUE CHANNEL // SEQUENCE: 2, 4, 6, 8, ? — transmit the missing number."
	case "PUZZLE-RED":
		return "RED CHANNEL // RELAY: three switches are numbered 1–3. Only the middle switch is active. Transmit its position."
	case "PUZZLE-GREEN":
		return "GREEN CHANNEL // DECODER: the relay subtracts 3 from its input. 12 becomes ? — transmit the output."
	default:
		return "UNKNOWN PUZZLE"
	}
}

func (g *Game) SolvePuzzle(id, node, answer string) (string, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.status != Playing || id == "" {
		return "cannot solve puzzle right now", false
	}
	if _, ok := g.players[id]; !ok {
		return "ACCESS DENIED: operative not recognized.", false
	}
	node = strings.ToUpper(strings.TrimSpace(node))
	idx, ok := map[string]int{"PUZZLE-BLUE": 0, "PUZZLE-RED": 1, "PUZZLE-GREEN": 2}[node]
	if !ok {
		return "unknown puzzle", false
	}
	g.discovered[node] = true
	if g.puzzleSolved[idx] {
		return "PUZZLE ALREADY VERIFIED.", true
	}
	if strings.EqualFold(strings.TrimSpace(answer), g.puzzleAnswers[idx]) {
		g.puzzleSolved[idx] = true
		messages := [3]string{
			"PUZZLE 1 VERIFIED: BLUE CHANNEL = 10.",
			"PUZZLE 2 VERIFIED: RED CHANNEL = 2.",
			"PUZZLE 3 VERIFIED: GREEN CHANNEL = 9. ALL VERIFICATION COMPLETE.",
		}
		return messages[idx], true
	}
	return "PUZZLE REJECTED: incorrect transmission. Try again.", false
}

func (g *Game) SubmitCode(id, code string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.status != Playing || id == "" || g.fragmentCount() < 3 || g.puzzleCount() < 3 {
		return false
	}
	expected := strings.Join(g.fragments[:], "")
	if strings.EqualFold(strings.TrimSpace(code), expected) {
		g.status = Won
		return true
	}
	return false
}
func (g *Game) fragmentCount() int {
	n := 0
	for _, v := range g.revealed {
		if v {
			n++
		}
	}
	return n
}
func (g *Game) puzzleCount() int {
	n := 0
	for _, v := range g.puzzleSolved {
		if v {
			n++
		}
	}
	return n
}
func (g *Game) State() State {
	g.mu.RLock()
	defer g.mu.RUnlock()
	ps := make([]PlayerInfo, 0, len(g.players))
	for _, p := range g.players {
		ps = append(ps, p.PlayerInfo)
	}
	sort.Slice(ps, func(i, j int) bool { return ps[i].ID < ps[j].ID })
	d := make([]string, 0, len(g.discovered))
	for n := range g.discovered {
		d = append(d, n)
	}
	sort.Strings(d)
	locked := g.fragmentCount() < 3 || g.puzzleCount() < 3
	objective := "READY UP — all operatives must be ready."
	if g.status == Playing {
		if locked {
			objective = "EXPLORE NODES, VERIFY ALL PUZZLES, RECOVER ALL FRAGMENTS, THEN ACCESS SERVER-CORE."
		} else {
			objective = "SERVER-CORE UNLOCKED — assemble the six-character escape code."
		}
	} else if g.status == Won {
		objective = "ESCAPE SUCCESSFUL — GHOST NETWORK CONNECTION TERMINATED."
	} else if g.status == Lost {
		objective = "LOCKDOWN COMPLETE — THE NETWORK HAS GONE DARK."
	}
	nodes := []Node{{"NODE-01", "ARCHIVE", "Protocol structure", false, "intel"}, {"NODE-07", "RELAY", "Operative network", false, "intel"}, {"NODE-13", "LOGS", "Recovery telemetry", false, "intel"}, {"NODE-17", "FRAGMENT", "Encrypted data", false, "fragment"}, {"PUZZLE-BLUE", "BLUE CHANNEL", "Verify the blue channel", false, "puzzle"}, {"PUZZLE-RED", "RED CHANNEL", "Verify the red channel", false, "puzzle"}, {"PUZZLE-GREEN", "GREEN CHANNEL", "Verify the green channel", false, "puzzle"}, {"SERVER-CORE", "EXIT", "Final terminal", locked, "core"}}
	puzzles := []Puzzle{
		{ID: "PUZZLE-BLUE", Title: "BLUE CHANNEL", Prompt: "2, 4, 6, 8, ?", Solved: g.puzzleSolved[0]},
		{ID: "PUZZLE-RED", Title: "RED CHANNEL", Prompt: "Middle switch position (1–3)?", Solved: g.puzzleSolved[1]},
		{ID: "PUZZLE-GREEN", Title: "GREEN CHANNEL", Prompt: "12 - 3 = ?", Solved: g.puzzleSolved[2]},
	}
	return State{Status: g.status, RemainingTime: g.remaining, Players: ps, Nodes: nodes, Discovered: d, FragmentsFound: g.fragmentCount(), FragmentTotal: 3, Objective: objective, CoreReady: !locked && g.status == Playing, PuzzleProgress: g.puzzleCount(), PuzzleTotal: 3, Puzzles: puzzles}
}
