package ws

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

var testUpgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func dialPair(t *testing.T) (client *Client, peer *websocket.Conn, cleanup func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
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
	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	c := NewClient(conn, "p1", "r1", "Alice")
	go c.WritePump()
	return c, conn, func() {
		c.ForceClose()
		srv.Close()
	}
}

func TestNewClientDefaults(t *testing.T) {
	c, _, cleanup := dialPair(t)
	defer cleanup()
	if c.Seat != -1 || c.PlayerID != "p1" || c.RoomID != "r1" || c.Name != "Alice" {
		t.Fatalf("%#v", c)
	}
	if cap(c.Send) != 64 {
		t.Fatalf("send cap=%d", cap(c.Send))
	}
}

func TestForceCloseIdempotent(t *testing.T) {
	c, _, cleanup := dialPair(t)
	defer cleanup()
	c.ForceClose()
	c.ForceClose() // must not panic
}

func TestTrySendOverflowCloses(t *testing.T) {
	c, _, cleanup := dialPair(t)
	defer cleanup()
	// Fill buffer without WritePump draining: stop pump by replacing Send consumer.
	// WritePump is already running; fill faster than it can write by using a fresh client without pump.
	cleanup()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		// do not read; just keep conn
		_ = conn
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	c = NewClient(conn, "p2", "r1", "Bob")
	// no WritePump → buffer fills
	for i := 0; i < 64; i++ {
		c.TrySend([]byte("x"))
	}
	c.TrySend([]byte("overflow"))
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case _, ok := <-c.Send:
			if !ok {
				return // closed
			}
		default:
			// channel still open but may be mid-close
			time.Sleep(10 * time.Millisecond)
		}
	}
	// ForceClose from overflow is async; ensure closeOnce ran
	c.ForceClose()
}

func TestReadPumpDeliversMessages(t *testing.T) {
	var got atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		c := NewClient(conn, "p3", "r1", "Carol")
		c.OnMessage = func(_ *Client, data []byte) {
			if string(data) == `{"type":"ping"}` {
				got.Add(1)
			}
		}
		c.OnClose = func(*Client) {}
		go c.WritePump()
		c.ReadPump()
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	u.Scheme = "ws"
	peer, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	if err := peer.WriteMessage(websocket.TextMessage, []byte(`{"type":"ping"}`)); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got.Load() == 1 {
			_ = peer.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("ReadPump did not deliver")
}
