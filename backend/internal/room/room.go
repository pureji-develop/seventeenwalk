package room

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"seventeenwalk/internal/game"
	"seventeenwalk/internal/protocol"
	"seventeenwalk/internal/ws"
)

// ReconnectGrace is how long a mid-match disconnect may reclaim the seat.
const ReconnectGrace = 45 * time.Second

// Room is a 2-player match room with serialized message handling.
type Room struct {
	ID      string
	mu      sync.Mutex
	game    *game.Game
	clients map[string]*ws.Client // playerID → client
	grace   map[string]*time.Timer
}

func NewRoom(id string, cfg game.MatchConfig) *Room {
	r := &Room{
		ID:      id,
		game:    game.NewGameWithConfig(cfg),
		clients: make(map[string]*ws.Client),
		grace:   make(map[string]*time.Timer),
	}
	r.attachBuildHook()
	return r
}

func (r *Room) attachBuildHook() {
	r.game.SetUpdateHook(func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		events := r.game.ForceBuildTimeout()
		if len(events) == 0 {
			return
		}
		r.emitEventsLocked(events)
		r.broadcastStateLocked()
	})
}

func (r *Room) MatchConfig() game.MatchConfig {
	r.mu.Lock()
	defer r.mu.Unlock()
	return game.MatchConfig{
		TotalRounds:     r.game.TotalRounds,
		BuildTimeout:    r.game.BuildTimeout,
		BuildTimeoutSec: r.game.BuildTimeoutSec,
		InfiniteBuild:   r.game.BuildTimeoutSec <= 0,
		WallSorted:      r.game.WallSorted,
	}
}

// TestingFinishDraw ends WALK as DRAW and broadcasts (integration tests).
func (r *Room) TestingFinishDraw() {
	r.mu.Lock()
	defer r.mu.Unlock()
	events := r.game.FinishDrawForTest()
	if len(events) == 0 {
		return
	}
	r.emitEventsLocked(events)
	r.broadcastStateLocked()
}

func (r *Room) ClientCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.clients)
}

// HasPendingReconnect reports whether a grace timer is keeping the room alive.
func (r *Room) HasPendingReconnect() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.grace) > 0
}

func (r *Room) Phase() game.Phase {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.game.Phase
}

// Join registers a websocket client into the room (new seat or reconnect reclaim).
func (r *Room) Join(c *ws.Client) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Takeover / reclaim existing player identity.
	if seat, ok := r.game.SeatByPlayerID(c.PlayerID); ok {
		r.cancelGraceLocked(c.PlayerID)
		if old, exists := r.clients[c.PlayerID]; exists && old != c {
			old.OnClose = nil
			old.OnMessage = nil
			go old.ForceClose()
			delete(r.clients, c.PlayerID)
		}
		if err := r.game.ReclaimSeat(seat, c.PlayerID, c.Name); err != nil {
			return err
		}
		c.Seat = int(seat)
		c.OnMessage = r.handleRaw
		c.OnClose = r.handleClose
		r.clients[c.PlayerID] = c
		r.sendWelcomeLocked(c)
		r.broadcastStateLocked()
		return nil
	}

	if len(r.clients) >= 2 {
		return errRoomFull
	}
	// Persistent rooms (e.g. room_test) may keep stale mid-match state after everyone left.
	if len(r.clients) == 0 && len(r.grace) == 0 && r.game.Phase != game.PhaseLobby {
		r.resetGameLocked()
	}
	seat, err := r.game.JoinSeat(c.PlayerID, c.Name)
	if err != nil {
		return err
	}
	c.Seat = int(seat)
	c.OnMessage = r.handleRaw
	c.OnClose = r.handleClose
	r.clients[c.PlayerID] = c

	r.sendWelcomeLocked(c)
	r.broadcastStateLocked()
	return nil
}

func (r *Room) sendWelcomeLocked(c *ws.Client) {
	welcome := protocol.ServerEnvelope{
		Type: "welcome",
		Payload: protocol.WelcomePayload{
			PlayerID: c.PlayerID,
			Seat:     c.Seat,
			RoomID:   r.ID,
		},
	}
	c.TrySend(protocol.MustEncode(welcome))
}

func (r *Room) resetGameLocked() {
	cfg := game.MatchConfig{
		TotalRounds:     r.game.TotalRounds,
		BuildTimeoutSec: r.game.BuildTimeoutSec,
		InfiniteBuild:   r.game.BuildTimeoutSec <= 0,
		WallSorted:      r.game.WallSorted,
	}
	for id, t := range r.grace {
		t.Stop()
		delete(r.grace, id)
	}
	r.game = game.NewGameWithConfig(cfg)
	r.attachBuildHook()
}

var errRoomFull = &roomError{"ROOM_FULL", "room is full"}

type roomError struct{ Code, Msg string }

func (e *roomError) Error() string { return e.Msg }

func (r *Room) handleClose(c *ws.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cur, ok := r.clients[c.PlayerID]; !ok || cur != c {
		return // already removed / replaced
	}
	delete(r.clients, c.PlayerID)
	if c.Seat < 0 {
		return
	}
	seat := game.Seat(c.Seat)
	c.Seat = -1

	if r.game.Phase == game.PhaseLobby {
		_ = r.game.LeaveSeat(seat)
		r.broadcastStateLocked()
		return
	}
	if r.game.Phase == game.PhaseGameOver {
		r.broadcastStateLocked()
		return
	}

	// Mid-match: keep seat, start reconnect grace.
	r.game.MarkDisconnected(seat)
	s := seat
	r.emitEventsLocked([]game.Event{{Kind: "OPPONENT_DISCONNECT", Seat: &s, Phase: r.game.Phase}})
	r.broadcastStateLocked()
	r.armGraceLocked(c.PlayerID, seat)
}

func (r *Room) cancelGraceLocked(playerID string) {
	if t, ok := r.grace[playerID]; ok {
		t.Stop()
		delete(r.grace, playerID)
	}
}

func (r *Room) armGraceLocked(playerID string, seat game.Seat) {
	r.cancelGraceLocked(playerID)
	r.grace[playerID] = time.AfterFunc(ReconnectGrace, func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		delete(r.grace, playerID)
		if _, online := r.clients[playerID]; online {
			return
		}
		if r.game.Phase == game.PhaseLobby || r.game.Phase == game.PhaseGameOver {
			return
		}
		events := r.game.LeaveSeat(seat)
		r.emitEventsLocked(events)
		r.broadcastStateLocked()
	})
}

func (r *Room) handleRaw(c *ws.Client, data []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var env protocol.ClientEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		r.sendErrorLocked(c, "INVALID_PAYLOAD", "invalid json", env.RequestID)
		return
	}
	if env.Type == "ping" {
		c.TrySend(protocol.MustEncode(protocol.ServerEnvelope{Type: "pong", Payload: map[string]any{}}))
		return
	}

	seat := game.Seat(c.Seat)
	var events []game.Event
	var err error
	var closeAfter bool

	switch env.Type {
	case "ready":
		var p protocol.ReadyPayload
		_ = json.Unmarshal(env.Payload, &p)
		events, err = r.game.ApplyReady(seat, p.Ready)
	case "set_options", "setOptions":
		var p protocol.SetOptionsPayload
		if len(env.Payload) > 0 {
			if err2 := json.Unmarshal(env.Payload, &p); err2 != nil {
				r.sendErrorLocked(c, "INVALID_PAYLOAD", "bad payload", env.RequestID)
				return
			}
		}
		sec, ok := game.ParseTimeoutPreset(p.TimeoutPreset, p.BuildTimeoutSec)
		if !ok {
			r.sendErrorLocked(c, "INVALID_PAYLOAD", "invalid timeout preset", env.RequestID)
			return
		}
		rounds := p.TotalRounds
		if rounds == 0 {
			rounds = r.game.TotalRounds
		}
		events, err = r.game.ApplySetOptions(seat, sec, rounds, p.WallSorted)
	case "select_dora":
		var p protocol.SelectDoraPayload
		if err2 := json.Unmarshal(env.Payload, &p); err2 != nil {
			r.sendErrorLocked(c, "INVALID_PAYLOAD", "bad payload", env.RequestID)
			return
		}
		events, err = r.game.ApplySelectDora(seat, p.TileID)
	case "submit_hand":
		var p protocol.SubmitHandPayload
		if err2 := json.Unmarshal(env.Payload, &p); err2 != nil {
			r.sendErrorLocked(c, "INVALID_PAYLOAD", "bad payload", env.RequestID)
			return
		}
		events, err = r.game.ApplySubmitHand(seat, p.TileIDs)
	case "preview_waits", "previewWaits":
		var p protocol.SubmitHandPayload
		if len(env.Payload) > 0 {
			if err2 := json.Unmarshal(env.Payload, &p); err2 != nil {
				c.TrySend(protocol.MustEncode(protocol.ServerEnvelope{
					Type:    "waits",
					Payload: map[string]any{"waits": []string{}},
				}))
				return
			}
		}
		waits, err2 := r.game.PreviewWaits(seat, p.TileIDs)
		if err2 != nil {
			waits = nil
		}
		c.TrySend(protocol.MustEncode(protocol.ServerEnvelope{
			Type: "waits",
			Payload: map[string]any{
				"waits": protocol.KindsToStrings(waits),
			},
		}))
		return
	case "discard_tile":
		var p protocol.DiscardTilePayload
		if err2 := json.Unmarshal(env.Payload, &p); err2 != nil {
			r.sendErrorLocked(c, "INVALID_PAYLOAD", "bad payload", env.RequestID)
			return
		}
		events, err = r.game.ApplyDiscard(seat, p.TileID)
	case "ack_round":
		events, err = r.game.ApplyAckRound(seat)
	case "rematch":
		events, err = r.game.ApplyRematch(seat)
	case "leave":
		if _, ok := r.clients[c.PlayerID]; ok {
			r.cancelGraceLocked(c.PlayerID)
			events = r.game.LeaveSeat(seat)
			delete(r.clients, c.PlayerID)
			c.Seat = -1
			closeAfter = true
		}
	case "join":
		return
	default:
		r.sendErrorLocked(c, "ILLEGAL_ACTION", "unknown type: "+env.Type, env.RequestID)
		return
	}

	if err != nil {
		code := "ILLEGAL_ACTION"
		switch err {
		case game.ErrNotYourTurn:
			code = "NOT_YOUR_TURN"
		case game.ErrInvalidPayload:
			code = "INVALID_PAYLOAD"
		case game.ErrGameOver:
			code = "GAME_OVER"
		}
		r.sendErrorLocked(c, code, err.Error(), env.RequestID)
		return
	}
	r.emitEventsLocked(events)
	r.broadcastStateLocked()
	if closeAfter {
		go c.ForceClose()
	}
}

func (r *Room) emitEventsLocked(events []game.Event) {
	for _, ev := range events {
		seq := r.game.BumpSeq()
		payload := protocol.BuildEvent(r.game, ev)
		msg := protocol.MustEncode(protocol.ServerEnvelope{Type: "event", Seq: seq, Payload: payload})
		for _, c := range r.clients {
			c.TrySend(msg)
		}
	}
}

func (r *Room) broadcastStateLocked() {
	if len(r.clients) == 0 {
		return
	}
	seq := r.game.BumpSeq()
	for _, c := range r.clients {
		r.sendStateLocked(c, seq)
	}
}

func (r *Room) sendStateLocked(c *ws.Client, seq int) {
	if c.Seat < 0 {
		return
	}
	state := protocol.BuildState(r.game, r.ID, game.Seat(c.Seat))
	c.TrySend(protocol.MustEncode(protocol.ServerEnvelope{Type: "state", Seq: seq, Payload: state}))
}

func (r *Room) sendErrorLocked(c *ws.Client, code, msg, reqID string) {
	c.TrySend(protocol.MustEncode(protocol.ServerEnvelope{
		Type: "error",
		Payload: protocol.ErrorPayload{Code: code, Message: msg, RequestID: reqID},
	}))
	log.Printf("room=%s player=%s error=%s %s", r.ID, c.PlayerID, code, msg)
}
