package ws

import (
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 64 << 10
)

// Client is one WebSocket connection.
type Client struct {
	Conn      *websocket.Conn
	Send      chan []byte
	PlayerID  string
	RoomID    string
	Seat      int
	Name      string
	OnMessage func(c *Client, data []byte)
	OnClose   func(c *Client)
	closeOnce sync.Once
}

// NewClient wraps a connection.
func NewClient(conn *websocket.Conn, playerID, roomID, name string) *Client {
	return &Client{
		Conn:     conn,
		Send:     make(chan []byte, 64),
		PlayerID: playerID,
		RoomID:   roomID,
		Name:     name,
		Seat:     -1,
	}
}

// ForceClose closes the send channel and underlying connection (idempotent).
func (c *Client) ForceClose() {
	c.closeOnce.Do(func() {
		close(c.Send)
	})
	_ = c.Conn.Close()
}

// TrySend enqueues a message; drops and closes on overflow.
func (c *Client) TrySend(msg []byte) {
	select {
	case c.Send <- msg:
	default:
		log.Printf("ws send buffer full player=%s", c.PlayerID)
		go c.ForceClose()
	}
}

// ReadPump reads from the socket.
func (c *Client) ReadPump() {
	defer func() {
		if c.OnClose != nil {
			c.OnClose(c)
		}
		c.Conn.Close()
	}()
	c.Conn.SetReadLimit(maxMessageSize)
	_ = c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		return c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		_, data, err := c.Conn.ReadMessage()
		if err != nil {
			return
		}
		if c.OnMessage != nil {
			c.OnMessage(c, data)
		}
	}
}

// WritePump writes queued messages and pings.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.Send:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
