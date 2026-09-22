package term

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/vadremix/tiny-gather/client/conn"
	"github.com/vadremix/tiny-gather/internal/sim"
	"github.com/vadremix/tiny-gather/internal/world"
	"github.com/vadremix/tiny-gather/internal/wsserver"
)

// serve starts a real server over the named fixture and returns its URL.
func serve(t *testing.T, fixture string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "fixtures", fixture))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	w, _, err := world.LoadFixture(data)
	if err != nil {
		t.Fatalf("loading fixture: %v", err)
	}

	srv := wsserver.New(sim.NewWithWorld(w, 1), wsserver.Options{TickRate: 5 * time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = srv.Run(ctx) }()
	http := httptest.NewServer(srv.Handler())
	t.Cleanup(func() { http.Close(); cancel() })
	return "ws" + strings.TrimPrefix(http.URL, "http")
}

// session runs the real loop against a real server, injects keys, lets the
// world settle, quits, and hands back the screen as it was left.
//
// The screen is read only after run has returned. tcell's simulation screen
// has no locking, so reading it while the loop draws is a data race — in the
// test, not the renderer, which draws from one goroutine only.
func session(t *testing.T, name string, keys func(tcell.SimulationScreen), settle time.Duration) tcell.SimulationScreen {
	t.Helper()
	url := serve(t, "two-blue-chests.json")

	dial, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	c, err := conn.Dial(dial, url, conn.Options{Name: name})
	cancel()
	if err != nil {
		t.Fatalf("joining as %s: %v", name, err)
	}
	t.Cleanup(func() { _ = c.Close() })

	screen := tcell.NewSimulationScreen("")
	if err := screen.Init(); err != nil {
		t.Fatalf("simulation screen: %v", err)
	}
	screen.SetSize(100, 24)

	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	done := make(chan error, 1)
	go func() { done <- run(ctx, c, screen) }()

	// Let the first frames land before typing anything.
	time.Sleep(100 * time.Millisecond)
	if keys != nil {
		keys(screen)
	}
	time.Sleep(settle)

	screen.InjectKey(tcell.KeyRune, 'q', tcell.ModNone)
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("run returned %v on quit, want nil", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("run did not return after q")
	}
	return screen
}

// screenText flattens a simulation screen to lines of text.
func screenText(s tcell.SimulationScreen) []string {
	cells, w, h := s.GetContents()
	lines := make([]string, h)
	for y := 0; y < h; y++ {
		var b strings.Builder
		for x := 0; x < w; x++ {
			c := cells[y*w+x]
			if len(c.Runes) == 0 {
				b.WriteRune(' ')
			} else {
				b.WriteRune(c.Runes[0])
			}
		}
		lines[y] = strings.TrimRight(b.String(), " ")
	}
	return lines
}

func contains(lines []string, s string) bool {
	for _, l := range lines {
		if strings.Contains(l, s) {
			return true
		}
	}
	return false
}

// TestRendersWhatThePlayerSees checks the screen shows the player, the
// terrain, nearby objects and the side panel — and logs the frame so a human
// can look at it.
func TestRendersWhatThePlayerSees(t *testing.T) {
	screen := session(t, "tester", nil, 50*time.Millisecond)
	lines := screenText(screen)
	for _, l := range lines {
		t.Log(l)
	}

	if !contains(lines, "@") {
		t.Error("the player is not drawn")
	}
	if !contains(lines, "#") {
		t.Error("no terrain is drawn; the map from the welcome is not being used")
	}
	if !contains(lines, "=") {
		t.Error("no chest is drawn, though two are within range")
	}
	if !contains(lines, "tester") {
		t.Error("the side panel does not show the player's name")
	}
	if !contains(lines, "chest_") {
		t.Error("the nearby list does not name the chests")
	}
	if !contains(lines, "house") {
		t.Error("the house landmark is not labelled")
	}
}

// TestChatPromptSendsSay types a message through the prompt and checks it
// comes back as a chat line in the log, proving the whole input path: key
// handler, prompt, protocol, server, event, log.
func TestChatPromptSendsSay(t *testing.T) {
	screen := session(t, "talker", func(s tcell.SimulationScreen) {
		s.InjectKey(tcell.KeyRune, 't', tcell.ModNone)
		for _, r := range "hello there" {
			s.InjectKey(tcell.KeyRune, r, tcell.ModNone)
		}
		s.InjectKey(tcell.KeyEnter, 0, tcell.ModNone)
	}, 300*time.Millisecond) // sixty 5ms ticks for a loopback round trip

	lines := screenText(screen)
	if !contains(lines, "talker: hello there") {
		for _, l := range lines {
			t.Log(l)
		}
		t.Fatal("the chat line never appeared in the log")
	}
}

// TestFogDimsBeyondRange: tiles past the perception radius are drawn without
// a floor marker, so the edge of what the player can see is visible on screen.
func TestFogDimsBeyondRange(t *testing.T) {
	screen := session(t, "looker", nil, 50*time.Millisecond)
	cells, w, h := screen.GetContents()
	at := func(x, y int) rune {
		c := cells[y*w+x]
		if len(c.Runes) == 0 {
			return ' '
		}
		return c.Runes[0]
	}

	// Find the player on screen rather than assuming a spawn point.
	px, py := -1, -1
	for y := 0; y < h && px < 0; y++ {
		for x := 0; x < w; x++ {
			if at(x, y) == '@' {
				px, py = x, y
				break
			}
		}
	}
	if px < 0 {
		t.Fatal("the player is not on screen")
	}

	// The map is 24 wide; on a 100-column screen it is drawn unscrolled from
	// column 0, so screen coordinates are map coordinates.
	if at(px+1, py) == ' ' && at(px-1, py) == ' ' {
		t.Error("the tiles beside the player are drawn as unseen")
	}
	// The far corner is well past radius 12 from anywhere near the house and
	// is walkable in this fixture.
	if r := at(22, 14); r == '.' {
		t.Errorf("a tile beyond perception range is drawn as visible floor (%q)", r)
	}
}
