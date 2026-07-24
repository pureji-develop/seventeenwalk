package room

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"

	"seventeenwalk/internal/config"
	"seventeenwalk/internal/game"
)

// DefaultTestRoomID is the shared local-dev room id (never deleted).
const DefaultTestRoomID = "room_test"

// Hub manages rooms.
type Hub struct {
	mu       sync.Mutex
	rooms    map[string]*Room
	defaults config.Match
}

func NewHub(defaults config.Match) *Hub {
	defaults.Normalize()
	h := &Hub{rooms: make(map[string]*Room), defaults: defaults}
	h.rooms[DefaultTestRoomID] = NewRoom(DefaultTestRoomID, game.MatchConfig{
		TotalRounds:     defaults.TotalRounds,
		BuildTimeoutSec: defaults.BuildTimeoutSec,
		WallSorted:      false,
	})
	return h
}

func (h *Hub) Defaults() config.Match {
	return h.defaults
}

func (h *Hub) CreateRoom(override config.Match) *Room {
	cfg := h.defaults.Merge(override)
	id := "r_" + randomID(8)
	r := NewRoom(id, game.MatchConfig{
		TotalRounds:     cfg.TotalRounds,
		BuildTimeoutSec: cfg.BuildTimeoutSec,
		WallSorted:      false,
	})
	h.mu.Lock()
	h.rooms[id] = r
	h.mu.Unlock()
	return r
}

func (h *Hub) Get(id string) (*Room, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	r, ok := h.rooms[id]
	return r, ok
}

func (h *Hub) RemoveIfEmpty(id string) {
	if id == DefaultTestRoomID {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	r, ok := h.rooms[id]
	if !ok {
		return
	}
	if r.ClientCount() == 0 && !r.HasPendingReconnect() {
		delete(h.rooms, id)
	}
}

func (h *Hub) RoomCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.rooms)
}

func randomID(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Info is a lightweight room status.
func (h *Hub) Info(id string) (players int, phase game.Phase, err error) {
	r, ok := h.Get(id)
	if !ok {
		return 0, "", fmt.Errorf("not found")
	}
	return r.ClientCount(), r.Phase(), nil
}
