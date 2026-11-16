package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/websocket"

	"seventeenwalk/internal/config"
	"seventeenwalk/internal/room"
	"seventeenwalk/internal/ws"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	addr := ":8080"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}
	defaults := config.FromEnv()
	hub := room.NewHub(defaults)

	mux := newMux(hub)

	log.Printf("SeventeenWalk server listening on %s (totalRounds=%d buildTimeoutSec=%d)",
		addr, defaults.TotalRounds, defaults.BuildTimeoutSec)
	if err := http.ListenAndServe(addr, withCORS(mux)); err != nil {
		log.Fatal(err)
	}
}

func newMux(hub *room.Hub) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /rooms", func(w http.ResponseWriter, r *http.Request) {
		var body config.Match
		if r.Body != nil {
			defer r.Body.Close()
			dec := json.NewDecoder(io.LimitReader(r.Body, 1<<16))
			if err := dec.Decode(&body); err != nil && err != io.EOF {
				http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
				return
			}
		}
		rm := hub.CreateRoom(body)
		cfg := rm.MatchConfig()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"roomId":          rm.ID,
			"totalRounds":     cfg.TotalRounds,
			"buildTimeoutSec": cfg.BuildTimeoutSec,
			"wallSorted":      cfg.WallSorted,
		})
	})
	mux.HandleFunc("GET /rooms", func(w http.ResponseWriter, r *http.Request) {
		list := hub.ListPublicRooms()
		if list == nil {
			list = []room.PublicRoomInfo{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"rooms": list})
	})
	mux.HandleFunc("GET /rooms/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		players, phase, err := hub.Info(id)
		if err != nil {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"roomId":  id,
			"players": players,
			"phase":   phase,
		})
	})
	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		roomID := r.URL.Query().Get("roomId")
		name := r.URL.Query().Get("name")
		if roomID == "" || name == "" {
			http.Error(w, "roomId and name required", http.StatusBadRequest)
			return
		}
		rm, ok := hub.Get(roomID)
		if !ok {
			http.Error(w, "room not found", http.StatusNotFound)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("upgrade: %v", err)
			return
		}
		playerID := r.URL.Query().Get("playerId")
		if playerID == "" || len(playerID) > 64 {
			playerID = "p_" + randomID(6)
		}
		client := ws.NewClient(conn, playerID, roomID, name)
		if err := rm.Join(client); err != nil {
			code := "ROOM_FULL"
			if re, ok := err.(*room.JoinError); ok {
				code = re.Code
			}
			_ = conn.WriteJSON(map[string]any{
				"type":    "error",
				"payload": map[string]string{"code": code, "message": err.Error()},
			})
			_ = conn.Close()
			return
		}
		go client.WritePump()
		client.ReadPump()
		hub.RemoveIfEmpty(roomID)
	})
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func randomID(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
