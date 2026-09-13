package game

import (
	"fmt"
	"strings"
	"testing"
)

func startTestGame(t *testing.T, names ...string) *Game {
	t.Helper()
	g := New()
	for i, n := range names {
		id := string(rune('a' + i))
		g.AddPlayer(id, n)
		g.SetReady(id, true)
	}
	if !g.Start() {
		t.Fatal("expected game to start")
	}
	return g
}

func TestRoomReadyAndStart(t *testing.T) {
	g := New()
	g.AddPlayer("a", "A")
	g.AddPlayer("b", "B")
	g.SetReady("a", true)
	if g.ReadyToStart() {
		t.Fatal("should wait for all players")
	}
	g.SetReady("b", true)
	if !g.ReadyToStart() {
		t.Fatal("should be ready")
	}
	if !g.Start() {
		t.Fatal("start failed")
	}
	if g.State().Status != Playing {
		t.Fatal("expected playing")
	}
}
func TestFragmentsRequireAll(t *testing.T) {
	g := startTestGame(t, "A", "B", "C")
	msg, ok := g.Explore("a", "SERVER-CORE")
	if !ok || msg == "" {
		t.Fatal("expected locked core response")
	}
	g.Explore("a", "NODE-17")
	g.Explore("b", "NODE-17")
	g.Explore("c", "NODE-17")
	if g.State().FragmentsFound != 3 {
		t.Fatalf("found=%d", g.State().FragmentsFound)
	}
	if ok := g.SubmitCode("a", "wrong"); ok {
		t.Fatal("wrong code accepted")
	}
}
func TestWinWithFragments(t *testing.T) {
	g := startTestGame(t, "A", "B", "C")
	g.Explore("a", "NODE-17")
	g.Explore("b", "NODE-17")
	g.Explore("c", "NODE-17")
	g.Explore("a", "SERVER-CORE")
	s := g.State()
	_ = s
	if !g.SubmitCode("a", "not-the-code") == false {
		t.Fatal("unexpected win")
	}
}
func TestDisconnectRebalancesSlots(t *testing.T) {
	g := startTestGame(t, "A", "B", "C")
	g.Explore("a", "NODE-17")
	g.Explore("b", "NODE-17")
	g.RemovePlayer("b")
	g.Explore("a", "NODE-17")
	g.Explore("c", "NODE-17")
	if g.State().FragmentsFound != 3 {
		t.Fatalf("expected remaining players to recover all fragments, got %d", g.State().FragmentsFound)
	}
}
func TestTimer(t *testing.T) {
	g := startTestGame(t, "A")
	for i := 0; i < 300; i++ {
		g.Tick()
	}
	if g.State().Status != Lost {
		t.Fatal("expected loss")
	}
	if g.State().RemainingTime != 0 {
		t.Fatal("expected zero timer")
	}
}

func TestPuzzleRequiresCorrectAnswer(t *testing.T) {
	g := startTestGame(t, "A")
	if msg, ok := g.SolvePuzzle("a", "PUZZLE-BLUE", "9"); ok || msg == "" {
		t.Fatal("incorrect puzzle answer should be rejected")
	}
	if g.State().PuzzleProgress != 0 {
		t.Fatal("incorrect answer changed puzzle progress")
	}
	if msg, ok := g.SolvePuzzle("a", "PUZZLE-BLUE", "10"); !ok || msg == "" {
		t.Fatal("correct puzzle answer should be accepted")
	}
	if g.State().PuzzleProgress != 1 {
		t.Fatal("expected one solved puzzle")
	}
}

func TestDisconnectPreservesRevealedFragments(t *testing.T) {
	g := startTestGame(t, "A", "B", "C")
	g.Explore("b", "NODE-17")
	before := g.State().FragmentsFound
	g.RemovePlayer("b")
	if got := g.State().FragmentsFound; got != before {
		t.Fatalf("disconnect should not erase revealed fragments: before=%d after=%d", before, got)
	}
	g.Explore("a", "NODE-17")
	g.Explore("c", "NODE-17")
	if g.State().FragmentsFound != 3 {
		t.Fatalf("expected all fragments to remain recoverable, got %d", g.State().FragmentsFound)
	}
}

func TestCompleteTwoPlayerMission(t *testing.T) {
	g := startTestGame(t, "Alice", "Bob")

	// The two-player setup must distribute the three fragments asymmetrically.
	first, ok := g.Explore("a", "NODE-17")
	if !ok || !strings.Contains(first, "FRAGMENT 1:") || !strings.Contains(first, "FRAGMENT 3:") {
		t.Fatalf("player a should receive fragments 1 and 3, got %q", first)
	}
	second, ok := g.Explore("b", "NODE-17")
	if !ok || !strings.Contains(second, "FRAGMENT 2:") {
		t.Fatalf("player b should receive fragment 2, got %q", second)
	}

	// A player cannot unlock the core until every puzzle is verified.
	if msg, ok := g.Explore("a", "SERVER-CORE"); !ok || !strings.Contains(msg, "ACCESS DENIED") {
		t.Fatalf("core should remain locked before verification: %q", msg)
	}

	for _, puzzle := range []struct {
		node   string
		answer string
	}{
		{"PUZZLE-BLUE", "10"},
		{"PUZZLE-RED", "2"},
		{"PUZZLE-GREEN", "9"},
	} {
		if msg, ok := g.SolvePuzzle("a", puzzle.node, puzzle.answer); !ok || msg == "" {
			t.Fatalf("failed to solve %s: %q", puzzle.node, msg)
		}
	}

	state := g.State()
	if state.FragmentsFound != 3 || state.PuzzleProgress != 3 || !state.CoreReady {
		t.Fatalf("expected fully unlocked mission, got fragments=%d puzzles=%d core=%v", state.FragmentsFound, state.PuzzleProgress, state.CoreReady)
	}

	if msg, ok := g.Explore("a", "SERVER-CORE"); !ok || !strings.Contains(msg, "ESCAPE TERMINAL READY") {
		t.Fatalf("expected ready escape terminal: %q", msg)
	}

	fragments := [3]string{}
	for _, part := range []string{first, second} {
		for _, field := range strings.Split(part, " | ") {
			if !strings.HasPrefix(field, "FRAGMENT ") {
				continue
			}
			var index int
			if _, err := fmt.Sscanf(field, "FRAGMENT %d:", &index); err != nil || index < 1 || index > 3 {
				t.Fatalf("invalid fragment field %q", field)
			}
			fragments[index-1] = strings.TrimSpace(strings.SplitN(field, ":", 2)[1])
		}
	}
	code := strings.Join(fragments[:], "")
	if len(code) != 6 {
		t.Fatalf("expected six-character code from three fragments, got %q", code)
	}
	if !g.SubmitCode("a", code) {
		t.Fatalf("expected valid escape code %q to win", code)
	}
	if g.State().Status != Won {
		t.Fatal("expected won status after valid escape code")
	}
}
