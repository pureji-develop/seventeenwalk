package room

import (
	"testing"
	"time"

	"seventeenwalk/internal/config"
	"seventeenwalk/internal/game"
)

func TestChallengeAcceptDeclineAndList(t *testing.T) {
	h := NewHub(config.Match{TotalRounds: 4, BuildTimeoutSec: 180})
	r := h.CreateRoom(config.Match{})

	ca, cleanA := dialTestClient(t, r.ID, "Host")
	defer cleanA()
	cb, cleanB := dialTestClient(t, r.ID, "Challenger")
	defer cleanB()

	if err := r.Join(ca); err != nil {
		t.Fatal(err)
	}
	info, ok := r.PublicInfo()
	if !ok || !info.Open || info.Busy || info.HostName != "Host" {
		t.Fatalf("open listing %#v ok=%v", info, ok)
	}

	if err := r.Join(cb); err != nil {
		t.Fatal(err)
	}
	_ = recvMsgs(ca, 300*time.Millisecond)
	_ = recvMsgs(cb, 300*time.Millisecond)

	info, ok = r.PublicInfo()
	if !ok || info.Open || !info.Busy {
		t.Fatalf("busy listing %#v", info)
	}

	// Challenger cannot ready while pending.
	cb.OnMessage(cb, mustJSON(t, map[string]any{"type": "ready", "payload": map[string]any{"ready": true}}))
	errMsgs := recvMsgs(cb, 300*time.Millisecond)
	foundPending := false
	for _, m := range errMsgs {
		if m["type"] != "error" {
			continue
		}
		p, _ := m["payload"].(map[string]any)
		if p["code"] == "CHALLENGE_PENDING" {
			foundPending = true
		}
	}
	if !foundPending {
		t.Fatalf("expected CHALLENGE_PENDING, got %#v", errMsgs)
	}

	// Decline → challenger kicked, room open again.
	ca.OnMessage(ca, mustJSON(t, map[string]any{
		"type":    "respond_challenge",
		"payload": map[string]any{"accept": false},
	}))
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if r.ClientCount() == 1 && r.challenge == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if r.ClientCount() != 1 || r.challenge != nil {
		t.Fatalf("after decline count=%d challenge=%v", r.ClientCount(), r.challenge != nil)
	}
	info, _ = r.PublicInfo()
	if !info.Open || info.Busy {
		t.Fatalf("should be open again %#v", info)
	}

	// New challenger + accept.
	cb2, cleanB2 := dialTestClient(t, r.ID, "Bob")
	defer cleanB2()
	if err := r.Join(cb2); err != nil {
		t.Fatal(err)
	}
	_ = recvMsgs(ca, 200*time.Millisecond)
	acceptChallenge(t, ca)
	_ = recvMsgs(ca, 200*time.Millisecond)
	_ = recvMsgs(cb2, 200*time.Millisecond)
	if r.challenge != nil {
		t.Fatal("challenge should clear on accept")
	}
	if r.Phase() != game.PhaseLobby {
		t.Fatalf("phase=%s", r.Phase())
	}
	list := h.ListPublicRooms()
	found := false
	for _, row := range list {
		if row.RoomID == r.ID {
			found = true
			if row.Open || row.Busy {
				t.Fatalf("full lobby should not be open/busy %#v", row)
			}
		}
	}
	if !found {
		t.Fatal("room missing from list")
	}
}

func TestChallengeTimeout(t *testing.T) {
	r := NewRoom("r_ch_to", game.MatchConfig{TotalRounds: 1, BuildTimeoutSec: 180})
	ca, cleanA := dialTestClient(t, r.ID, "Host")
	defer cleanA()
	cb, cleanB := dialTestClient(t, r.ID, "Guest")
	defer cleanB()
	_ = r.Join(ca)
	_ = r.Join(cb)
	_ = recvMsgs(ca, 200*time.Millisecond)

	// Force short timeout path.
	r.mu.Lock()
	if r.challenge == nil {
		r.mu.Unlock()
		t.Fatal("expected pending challenge")
	}
	pid := r.challenge.ChallengerID
	if r.challenge.timer != nil {
		r.challenge.timer.Stop()
	}
	r.mu.Unlock()
	r.onChallengeTimeout(pid)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if r.ClientCount() == 1 && r.challenge == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout did not clear challenger count=%d", r.ClientCount())
}
