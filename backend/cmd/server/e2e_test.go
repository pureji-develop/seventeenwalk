package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"seventeenwalk/internal/config"
	"seventeenwalk/internal/room"
)

func TestCreateRoomConfigOverride(t *testing.T) {
	hub := room.NewHub(config.Match{TotalRounds: 4, BuildTimeoutSec: 180})
	srv := httptest.NewServer(withCORS(newMux(hub)))
	defer srv.Close()

	body := `{"totalRounds":2,"buildTimeoutSec":30}`
	res, err := http.Post(srv.URL+"/rooms", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("status=%d", res.StatusCode)
	}
	var out struct {
		RoomID          string `json:"roomId"`
		TotalRounds     int    `json:"totalRounds"`
		BuildTimeoutSec int    `json:"buildTimeoutSec"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.RoomID == "" || out.TotalRounds != 2 || out.BuildTimeoutSec != 30 {
		t.Fatalf("unexpected %#v", out)
	}
}

func TestWSE2EFlowDrawLeave(t *testing.T) {
	hub := room.NewHub(config.Match{TotalRounds: 1, BuildTimeoutSec: 180})
	srv := httptest.NewServer(withCORS(newMux(hub)))
	defer srv.Close()

	roomID := createRoom(t, srv.URL, `{"totalRounds":1}`)
	a := dialWS(t, srv, roomID, "Alice")
	defer a.Close()
	b := dialWS(t, srv, roomID, "Bob")
	defer b.Close()

	_ = readUntil(t, a, "welcome")
	_ = readUntil(t, b, "welcome")

	// Host accept starts the match immediately.
	mustSend(t, a, map[string]any{"type": "respond_challenge", "payload": map[string]any{"accept": true}})

	stA := readStateUntil(t, a, "BUILD")
	stB := readStateUntil(t, b, "BUILD")
	if stA["doraIndicator"] == nil {
		t.Fatal("expected auto-selected doraIndicator")
	}
	if _, ok := stA["you"].(map[string]any)["agariAbandoned"]; !ok {
		t.Fatal("agariAbandoned missing on you")
	}
	if _, ok := stA["opponent"].(map[string]any)["agariAbandoned"]; !ok {
		t.Fatal("agariAbandoned missing on opponent")
	}

	submitFirst13(t, a, stA)
	submitFirst13(t, b, stB)

	stA = readStateUntil(t, a, "WALK")
	stB = readStateUntil(t, b, "WALK")

	// One real discard from the player to act.
	discardOnce(t, a, b, stA, stB)

	rm, ok := hub.Get(roomID)
	if !ok {
		t.Fatal("room missing")
	}
	rm.TestingFinishDraw()

	stA = readStateUntil(t, a, "ROUND_END")
	rr := stA["roundResult"].(map[string]any)
	if rr["result"] != "DRAW" {
		t.Fatalf("result=%v", rr["result"])
	}

	mustSend(t, a, map[string]any{"type": "ack_round", "payload": map[string]any{}})
	mustSend(t, b, map[string]any{"type": "ack_round", "payload": map[string]any{}})
	_ = readStateUntil(t, a, "GAME_OVER")

	mustSend(t, a, map[string]any{"type": "leave", "payload": map[string]any{}})
	mustSend(t, b, map[string]any{"type": "leave", "payload": map[string]any{}})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, ok := hub.Get(roomID); !ok {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("room %s not removed", roomID)
}

func discardOnce(t *testing.T, a, b *websocket.Conn, stA, stB map[string]any) {
	t.Helper()
	// Use both WALK snapshots so we never block on an empty socket waiting for state.
	for _, pair := range []struct {
		c  *websocket.Conn
		st map[string]any
	}{{a, stA}, {b, stB}} {
		st := pair.st
		if st["phase"] != "WALK" {
			continue
		}
		you := st["you"].(map[string]any)
		if int(st["toAct"].(float64)) != int(you["seat"].(float64)) {
			continue
		}
		walk, _ := st["walk"].(map[string]any)
		legal, _ := walk["legalTileIds"].([]any)
		if len(legal) == 0 {
			t.Fatal("no legal discard tiles")
		}
		mustSend(t, pair.c, map[string]any{
			"type":    "discard_tile",
			"payload": map[string]any{"tileId": int(legal[0].(float64))},
		})
		_ = readState(t, a)
		_ = readState(t, b)
		return
	}
	t.Fatal("could not discard")
}

func createRoom(t *testing.T, base, body string) string {
	t.Helper()
	res, err := http.Post(base+"/rooms", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var out struct {
		RoomID string `json:"roomId"`
	}
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.RoomID == "" {
		t.Fatal("empty roomId")
	}
	return out.RoomID
}

func dialWS(t *testing.T, srv *httptest.Server, roomID, name string) *websocket.Conn {
	t.Helper()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	u.Scheme = "ws"
	u.Path = "/ws"
	q := u.Query()
	q.Set("roomId", roomID)
	q.Set("name", name)
	u.RawQuery = q.Encode()
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func mustSend(t *testing.T, c *websocket.Conn, msg map[string]any) {
	t.Helper()
	if err := c.WriteJSON(msg); err != nil {
		t.Fatal(err)
	}
}

func readEnvelope(t *testing.T, c *websocket.Conn) map[string]any {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	var env map[string]any
	if err := c.ReadJSON(&env); err != nil {
		t.Fatal(err)
	}
	return env
}

func readUntil(t *testing.T, c *websocket.Conn, typ string) map[string]any {
	t.Helper()
	for i := 0; i < 40; i++ {
		env := readEnvelope(t, c)
		if env["type"] == typ {
			return env
		}
	}
	t.Fatalf("timeout waiting for %s", typ)
	return nil
}

func readState(t *testing.T, c *websocket.Conn) map[string]any {
	t.Helper()
	env := readUntil(t, c, "state")
	payload, _ := env["payload"].(map[string]any)
	return payload
}

func readStateUntil(t *testing.T, c *websocket.Conn, phase string) map[string]any {
	t.Helper()
	for i := 0; i < 60; i++ {
		st := readState(t, c)
		if st["phase"] == phase {
			return st
		}
	}
	t.Fatalf("timeout waiting phase %s", phase)
	return nil
}

func submitFirst13(t *testing.T, c *websocket.Conn, st map[string]any) {
	t.Helper()
	you := st["you"].(map[string]any)
	wall := you["wall"].([]any)
	ids := make([]int, 0, 13)
	for i := 0; i < 13 && i < len(wall); i++ {
		ids = append(ids, int(wall[i].(map[string]any)["id"].(float64)))
	}
	mustSend(t, c, map[string]any{"type": "submit_hand", "payload": map[string]any{"tileIds": ids}})
}
