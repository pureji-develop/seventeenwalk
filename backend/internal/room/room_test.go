package room

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"seventeenwalk/internal/config"
	"seventeenwalk/internal/game"
	"seventeenwalk/internal/ws"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

// dialTestClient creates a Client whose outbound messages stay on Send (no WritePump).
// Conn exists so ForceClose works.
func dialTestClient(t *testing.T, roomID, name string) (*ws.Client, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		go func() {
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					_ = conn.Close()
					return
				}
			}
		}()
	}))
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	u.Scheme = "ws"
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := ws.NewClient(conn, name+"-id", roomID, name)
	cleanup := func() {
		c.ForceClose()
		srv.Close()
	}
	return c, cleanup
}

func recvMsgs(c *ws.Client, wait time.Duration) []map[string]any {
	var out []map[string]any
	deadline := time.After(wait)
	idle := time.NewTimer(40 * time.Millisecond)
	defer idle.Stop()
	for {
		select {
		case b, ok := <-c.Send:
			if !ok {
				return out
			}
			var m map[string]any
			if json.Unmarshal(b, &m) == nil {
				out = append(out, m)
			}
			if !idle.Stop() {
				select {
				case <-idle.C:
				default:
				}
			}
			idle.Reset(40 * time.Millisecond)
		case <-idle.C:
			if len(out) > 0 {
				return out
			}
			idle.Reset(40 * time.Millisecond)
		case <-deadline:
			return out
		}
	}
}

func TestRoomReconnectReclaimsSeat(t *testing.T) {
	r := NewRoom("r_rc", game.MatchConfig{TotalRounds: 1, BuildTimeoutSec: 180})
	ca, cleanA := dialTestClient(t, r.ID, "A")
	defer cleanA()
	cb, cleanB := dialTestClient(t, r.ID, "B")
	defer cleanB()
	if err := r.Join(ca); err != nil {
		t.Fatal(err)
	}
	if err := r.Join(cb); err != nil {
		t.Fatal(err)
	}
	_ = recvMsgs(ca, 300*time.Millisecond)
	_ = recvMsgs(cb, 300*time.Millisecond)
	ca.OnMessage(ca, mustJSON(t, map[string]any{"type": "ready", "payload": map[string]any{"ready": true}}))
	cb.OnMessage(cb, mustJSON(t, map[string]any{"type": "ready", "payload": map[string]any{"ready": true}}))
	if r.Phase() != game.PhaseBuild {
		t.Fatalf("phase=%s", r.Phase())
	}
	playerID := ca.PlayerID
	seat := ca.Seat
	r.handleClose(ca)
	if r.ClientCount() != 1 {
		t.Fatalf("count=%d", r.ClientCount())
	}
	if !r.HasPendingReconnect() {
		t.Fatal("expected grace timer")
	}
	ca2, cleanA2 := dialTestClient(t, r.ID, "A")
	defer cleanA2()
	ca2.PlayerID = playerID
	if err := r.Join(ca2); err != nil {
		t.Fatal(err)
	}
	if ca2.Seat != seat {
		t.Fatalf("seat=%d want %d", ca2.Seat, seat)
	}
	if r.HasPendingReconnect() {
		t.Fatal("grace should clear")
	}
	if r.Phase() != game.PhaseBuild {
		t.Fatalf("phase=%s after reclaim", r.Phase())
	}
}

func TestRoomBuildTimeoutForcesWalk(t *testing.T) {
	r := NewRoom("r_to", game.MatchConfig{TotalRounds: 1, BuildTimeoutSec: 1})
	ca, cleanA := dialTestClient(t, r.ID, "A")
	defer cleanA()
	cb, cleanB := dialTestClient(t, r.ID, "B")
	defer cleanB()
	if err := r.Join(ca); err != nil {
		t.Fatal(err)
	}
	if err := r.Join(cb); err != nil {
		t.Fatal(err)
	}
	_ = recvMsgs(ca, 300*time.Millisecond)
	_ = recvMsgs(cb, 300*time.Millisecond)
	ca.OnMessage(ca, mustJSON(t, map[string]any{"type": "ready", "payload": map[string]any{"ready": true}}))
	cb.OnMessage(cb, mustJSON(t, map[string]any{"type": "ready", "payload": map[string]any{"ready": true}}))
	if r.Phase() != game.PhaseBuild {
		t.Fatalf("phase=%s", r.Phase())
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if r.Phase() == game.PhaseWalk {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("phase=%s want WALK after build timeout", r.Phase())
}

func TestHubCreateGetRemove(t *testing.T) {
	h := NewHub(config.Match{TotalRounds: 4, BuildTimeoutSec: 180})
	if h.RoomCount() != 1 {
		t.Fatalf("want test room only, got %d", h.RoomCount())
	}
	if _, ok := h.Get(DefaultTestRoomID); !ok {
		t.Fatal("missing room_test")
	}
	r := h.CreateRoom(config.Match{TotalRounds: 6, BuildTimeoutSec: 120})
	if r.ID == "" || h.RoomCount() != 2 {
		t.Fatalf("id=%s count=%d", r.ID, h.RoomCount())
	}
	got, ok := h.Get(r.ID)
	if !ok || got != r {
		t.Fatal("Get mismatch")
	}
	players, phase, err := h.Info(r.ID)
	if err != nil || players != 0 || phase != game.PhaseLobby {
		t.Fatalf("info players=%d phase=%s err=%v", players, phase, err)
	}
	h.RemoveIfEmpty(r.ID)
	if _, ok := h.Get(r.ID); ok {
		t.Fatal("empty room should be removed")
	}
	h.RemoveIfEmpty(DefaultTestRoomID)
	if _, ok := h.Get(DefaultTestRoomID); !ok {
		t.Fatal("room_test must persist")
	}
}

func TestRoomJoinReadySubmitWalk(t *testing.T) {
	h := NewHub(config.Match{TotalRounds: 1, BuildTimeoutSec: 180})
	r := h.CreateRoom(config.Match{TotalRounds: 1, BuildTimeoutSec: 180})

	ca, cleanA := dialTestClient(t, r.ID, "Alice")
	defer cleanA()
	cb, cleanB := dialTestClient(t, r.ID, "Bob")
	defer cleanB()

	if err := r.Join(ca); err != nil {
		t.Fatal(err)
	}
	if err := r.Join(cb); err != nil {
		t.Fatal(err)
	}
	if r.ClientCount() != 2 || r.Phase() != game.PhaseLobby {
		t.Fatalf("count=%d phase=%s", r.ClientCount(), r.Phase())
	}
	_ = recvMsgs(ca, 500*time.Millisecond)
	_ = recvMsgs(cb, 500*time.Millisecond)

	ca.OnMessage(ca, mustJSON(t, map[string]any{"type": "ready", "payload": map[string]any{"ready": true}}))
	cb.OnMessage(cb, mustJSON(t, map[string]any{"type": "ready", "payload": map[string]any{"ready": true}}))
	if r.Phase() != game.PhaseBuild {
		t.Fatalf("phase=%s", r.Phase())
	}

	stA := lastState(t, recvMsgs(ca, time.Second))
	stB := lastState(t, recvMsgs(cb, time.Second))
	submitWall(t, ca, stA)
	submitWall(t, cb, stB)
	if r.Phase() != game.PhaseWalk {
		t.Fatalf("phase=%s", r.Phase())
	}

	cfg := r.MatchConfig()
	if cfg.TotalRounds != 1 || cfg.BuildTimeoutSec != 180 {
		t.Fatalf("%#v", cfg)
	}
}

func TestRoomPingAndSetOptions(t *testing.T) {
	h := NewHub(config.Match{TotalRounds: 4, BuildTimeoutSec: 180})
	r := h.CreateRoom(config.Match{})
	c, cleanup := dialTestClient(t, r.ID, "Solo")
	defer cleanup()
	if err := r.Join(c); err != nil {
		t.Fatal(err)
	}
	_ = recvMsgs(c, 500*time.Millisecond)

	c.OnMessage(c, mustJSON(t, map[string]any{"type": "ping", "payload": map[string]any{}}))
	msgs := recvMsgs(c, time.Second)
	foundPong := false
	for _, m := range msgs {
		if m["type"] == "pong" {
			foundPong = true
		}
	}
	if !foundPong {
		t.Fatalf("no pong in %#v", msgs)
	}

	c.OnMessage(c, mustJSON(t, map[string]any{
		"type": "set_options",
		"payload": map[string]any{
			"timeoutPreset": "120",
			"totalRounds":   10,
			"wallSorted":    true,
		},
	}))
	cfg := r.MatchConfig()
	if cfg.TotalRounds != 10 || cfg.BuildTimeoutSec != 120 || !cfg.WallSorted {
		t.Fatalf("%#v", cfg)
	}
}

func TestRoomPreviewWaitsAndIllegal(t *testing.T) {
	h := NewHub(config.Match{TotalRounds: 1, BuildTimeoutSec: 180})
	r := h.CreateRoom(config.Match{TotalRounds: 1})
	ca, cleanA := dialTestClient(t, r.ID, "A")
	defer cleanA()
	cb, cleanB := dialTestClient(t, r.ID, "B")
	defer cleanB()
	_ = r.Join(ca)
	_ = r.Join(cb)
	_ = recvMsgs(ca, 500*time.Millisecond)
	ca.OnMessage(ca, mustJSON(t, map[string]any{"type": "ready", "payload": map[string]any{"ready": true}}))
	cb.OnMessage(cb, mustJSON(t, map[string]any{"type": "ready", "payload": map[string]any{"ready": true}}))
	st := lastState(t, recvMsgs(ca, time.Second))
	ids := wallIDsFromState(st)
	ca.OnMessage(ca, mustJSON(t, map[string]any{"type": "preview_waits", "payload": map[string]any{"tileIds": ids}}))
	msgs := recvMsgs(ca, time.Second)
	found := false
	for _, m := range msgs {
		if m["type"] == "waits" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected waits")
	}

	ca.OnMessage(ca, mustJSON(t, map[string]any{"type": "nope", "payload": map[string]any{}}))
	errMsgs := recvMsgs(ca, time.Second)
	foundErr := false
	for _, m := range errMsgs {
		if m["type"] == "error" {
			foundErr = true
		}
	}
	if !foundErr {
		t.Fatal("expected error for unknown type")
	}
}

func TestRoomFullAndLeaveCleanup(t *testing.T) {
	h := NewHub(config.Match{TotalRounds: 4, BuildTimeoutSec: 180})
	r := h.CreateRoom(config.Match{})
	c1, clean1 := dialTestClient(t, r.ID, "A")
	defer clean1()
	c2, clean2 := dialTestClient(t, r.ID, "B")
	defer clean2()
	c3, clean3 := dialTestClient(t, r.ID, "C")
	defer clean3()
	if err := r.Join(c1); err != nil {
		t.Fatal(err)
	}
	if err := r.Join(c2); err != nil {
		t.Fatal(err)
	}
	if err := r.Join(c3); err == nil {
		t.Fatal("expected room full")
	}
	c1.OnMessage(c1, mustJSON(t, map[string]any{"type": "leave", "payload": map[string]any{}}))
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if r.ClientCount() == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if r.ClientCount() != 1 {
		t.Fatalf("count=%d", r.ClientCount())
	}
	h.RemoveIfEmpty(r.ID)
	if _, ok := h.Get(r.ID); !ok {
		t.Fatal("room still has a client; should remain")
	}
	c2.OnMessage(c2, mustJSON(t, map[string]any{"type": "leave", "payload": map[string]any{}}))
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if r.ClientCount() == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	h.RemoveIfEmpty(r.ID)
	if _, ok := h.Get(r.ID); ok {
		t.Fatal("empty room should be gone")
	}
}

func TestRoomResetWhenRejoiningEmptyMidMatch(t *testing.T) {
	h := NewHub(config.Match{TotalRounds: 1, BuildTimeoutSec: 180})
	r, ok := h.Get(DefaultTestRoomID)
	if !ok || r == nil {
		t.Fatal("missing")
	}
	r.mu.Lock()
	r.game.Phase = game.PhaseWalk
	r.clients = map[string]*ws.Client{}
	r.mu.Unlock()

	c, cleanup := dialTestClient(t, r.ID, "Rejoin")
	defer cleanup()
	if err := r.Join(c); err != nil {
		t.Fatal(err)
	}
	if r.Phase() != game.PhaseLobby {
		t.Fatalf("phase=%s want LOBBY after reset", r.Phase())
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func lastState(t *testing.T, msgs []map[string]any) map[string]any {
	t.Helper()
	var st map[string]any
	for _, m := range msgs {
		if m["type"] != "state" {
			continue
		}
		p, _ := m["payload"].(map[string]any)
		st = p
	}
	if st == nil {
		t.Fatal("no state")
	}
	return st
}

func wallIDsFromState(st map[string]any) []int {
	you := st["you"].(map[string]any)
	wall := you["wall"].([]any)
	ids := make([]int, 0, 13)
	for i := 0; i < 13 && i < len(wall); i++ {
		ids = append(ids, int(wall[i].(map[string]any)["id"].(float64)))
	}
	return ids
}

func submitWall(t *testing.T, c *ws.Client, st map[string]any) {
	t.Helper()
	ids := wallIDsFromState(st)
	c.OnMessage(c, mustJSON(t, map[string]any{
		"type":    "submit_hand",
		"payload": map[string]any{"tileIds": ids},
	}))
}
