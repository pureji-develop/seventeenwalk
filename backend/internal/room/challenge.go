package room

import (
	"encoding/json"
	"time"

	"seventeenwalk/internal/game"
)

// parseAcceptPayload reads { "accept": true|false } from a client payload.
func parseAcceptPayload(raw json.RawMessage) (accept bool, ok bool) {
	if len(raw) == 0 {
		return false, false
	}
	var p struct {
		Accept *bool `json:"accept"`
	}
	if err := json.Unmarshal(raw, &p); err != nil || p.Accept == nil {
		return false, false
	}
	return *p.Accept, true
}

// ChallengeTimeout is how long the host may accept/decline a challenger.
const ChallengeTimeout = 60 * time.Second

// PublicRoomInfo is a lobby listing entry (in-memory, no DB).
type PublicRoomInfo struct {
	RoomID          string `json:"roomId"`
	HostName        string `json:"hostName"`
	Players         int    `json:"players"`
	Phase           string `json:"phase"`
	TotalRounds     int    `json:"totalRounds"`
	BuildTimeoutSec int    `json:"buildTimeoutSec"`
	WallSorted      bool   `json:"wallSorted"`
	// Open: 1 host in LOBBY, no pending challenge → challengable.
	Open bool `json:"open"`
	// Busy: challenge pending approval.
	Busy bool `json:"busy"`
}

type pendingChallenge struct {
	ChallengerSeat game.Seat
	ChallengerID   string
	Name           string
	ExpiresAt      time.Time
	timer          *time.Timer
}

// ChallengeView is sent in state / events.
type ChallengeView struct {
	FromName   string `json:"fromName"`
	FromSeat   int    `json:"fromSeat"`
	ExpiresAtMs int64 `json:"expiresAtMs"`
	// Role for the viewer: "host" | "challenger"
	Role string `json:"role"`
}

func (h *Hub) ListPublicRooms() []PublicRoomInfo {
	h.mu.Lock()
	rooms := make([]*Room, 0, len(h.rooms))
	for _, r := range h.rooms {
		rooms = append(rooms, r)
	}
	h.mu.Unlock()

	out := make([]PublicRoomInfo, 0, len(rooms))
	for _, r := range rooms {
		if info, ok := r.PublicInfo(); ok {
			out = append(out, info)
		}
	}
	return out
}

func (r *Room) PublicInfo() (PublicRoomInfo, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.game.Phase != game.PhaseLobby {
		return PublicRoomInfo{}, false
	}
	cfg := game.MatchConfig{
		TotalRounds:     r.game.TotalRounds,
		BuildTimeoutSec: r.game.BuildTimeoutSec,
		WallSorted:      r.game.WallSorted,
	}
	n := len(r.clients)
	if n == 0 {
		return PublicRoomInfo{}, false
	}
	hostName := ""
	for i := 0; i < 2; i++ {
		p := r.game.Players[i]
		if p.ID == "" {
			continue
		}
		// Prefer non-challenger as host for listing.
		if r.challenge != nil && game.Seat(i) == r.challenge.ChallengerSeat {
			continue
		}
		hostName = p.Name
		break
	}
	if hostName == "" {
		for i := 0; i < 2; i++ {
			if r.game.Players[i].ID != "" {
				hostName = r.game.Players[i].Name
				break
			}
		}
	}
	busy := r.challenge != nil
	open := n == 1 && !busy
	if !open && !busy {
		// Full lobby (2 approved) — still list as not open.
		return PublicRoomInfo{
			RoomID:          r.ID,
			HostName:        hostName,
			Players:         n,
			Phase:           string(r.game.Phase),
			TotalRounds:     cfg.TotalRounds,
			BuildTimeoutSec: cfg.BuildTimeoutSec,
			WallSorted:      cfg.WallSorted,
			Open:            false,
			Busy:            false,
		}, true
	}
	return PublicRoomInfo{
		RoomID:          r.ID,
		HostName:        hostName,
		Players:         n,
		Phase:           string(r.game.Phase),
		TotalRounds:     cfg.TotalRounds,
		BuildTimeoutSec: cfg.BuildTimeoutSec,
		WallSorted:      cfg.WallSorted,
		Open:            open,
		Busy:            busy,
	}, true
}

func (r *Room) clearChallengeLocked() {
	if r.challenge == nil {
		return
	}
	if r.challenge.timer != nil {
		r.challenge.timer.Stop()
	}
	r.challenge = nil
}

func (r *Room) startChallengeLocked(challengerSeat game.Seat, playerID, name string) {
	r.clearChallengeLocked()
	exp := time.Now().Add(ChallengeTimeout)
	ch := &pendingChallenge{
		ChallengerSeat: challengerSeat,
		ChallengerID:   playerID,
		Name:           name,
		ExpiresAt:      exp,
	}
	ch.timer = time.AfterFunc(ChallengeTimeout, func() {
		r.onChallengeTimeout(playerID)
	})
	r.challenge = ch
}

func (r *Room) onChallengeTimeout(playerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.challenge == nil || r.challenge.ChallengerID != playerID {
		return
	}
	r.rejectChallengeLocked("timeout")
}

func (r *Room) rejectChallengeLocked(reason string) {
	if r.challenge == nil {
		return
	}
	ch := r.challenge
	r.clearChallengeLocked()
	c, ok := r.clients[ch.ChallengerID]
	if ok {
		msg := "ホストが挑戦を拒否しました"
		if reason == "timeout" {
			msg = "挑戦の承認がタイムアウトしました（1分）"
		}
		r.sendErrorLocked(c, "CHALLENGE_DENIED", msg, "")
		delete(r.clients, ch.ChallengerID)
		_ = r.game.LeaveSeat(ch.ChallengerSeat)
		c.Seat = -1
		go c.ForceClose()
	}
	s := ch.ChallengerSeat
	r.emitEventsLocked([]game.Event{{
		Kind:  "CHALLENGE_DENIED",
		Seat:  &s,
		Phase: game.PhaseLobby,
		Extra: map[string]any{"reason": reason, "fromName": ch.Name},
	}})
	r.broadcastStateLocked()
}

// RespondChallenge is called by the host (accept / decline).
func (r *Room) respondChallengeLocked(hostSeat game.Seat, accept bool) error {
	if r.game.Phase != game.PhaseLobby {
		return game.ErrIllegalAction
	}
	if r.challenge == nil {
		return game.ErrIllegalAction
	}
	if game.Seat(hostSeat) == r.challenge.ChallengerSeat {
		return game.ErrIllegalAction
	}
	// Host must be the other occupied seat.
	host := r.game.Players[hostSeat]
	if host.ID == "" {
		return game.ErrIllegalAction
	}
	ch := r.challenge
	if !accept {
		r.rejectChallengeLocked("declined")
		return nil
	}
	// Start first; only then clear challenge so a failed start keeps the prompt.
	startEv, err := r.game.StartMatchFromLobby()
	if err != nil {
		return err
	}
	r.clearChallengeLocked()
	s := ch.ChallengerSeat
	r.emitEventsLocked([]game.Event{{
		Kind:  "CHALLENGE_ACCEPTED",
		Seat:  &s,
		Phase: r.game.Phase,
		Extra: map[string]any{"fromName": ch.Name},
	}})
	r.emitEventsLocked(startEv)
	r.broadcastStateLocked()
	return nil
}

func (r *Room) challengePending() bool {
	return r.challenge != nil
}

func (r *Room) challengeViewFor(viewer game.Seat) *ChallengeView {
	if r.challenge == nil {
		return nil
	}
	role := "host"
	if viewer == r.challenge.ChallengerSeat {
		role = "challenger"
	}
	return &ChallengeView{
		FromName:    r.challenge.Name,
		FromSeat:    int(r.challenge.ChallengerSeat),
		ExpiresAtMs: r.challenge.ExpiresAt.UnixMilli(),
		Role:        role,
	}
}

func (r *Room) isChallengerSeat(seat game.Seat) bool {
	return r.challenge != nil && r.challenge.ChallengerSeat == seat
}

func (r *Room) bothSeatsFilledLocked() bool {
	return r.game.Players[0].ID != "" && r.game.Players[1].ID != ""
}

// maybeAutoStartLocked starts the match when both seats are filled and no
// challenge is pending. This recovers the stuck "both entered, still LOBBY" state.
func (r *Room) maybeAutoStartLocked() bool {
	if r.game.Phase != game.PhaseLobby || r.challenge != nil || !r.bothSeatsFilledLocked() {
		return false
	}
	ev, err := r.game.StartMatchFromLobby()
	if err != nil {
		return false
	}
	r.emitEventsLocked(ev)
	r.broadcastStateLocked()
	return true
}
