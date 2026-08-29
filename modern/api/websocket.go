package api

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var socketUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(*http.Request) bool { return true },
}

func (h *Handler) serveWebSocket(w http.ResponseWriter, r *http.Request) {
	connection, err := socketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer connection.Close()
	connection.SetReadLimit(64 << 10)
	_ = connection.SetReadDeadline(time.Now().Add(90 * time.Second))
	connection.SetPongHandler(func(string) error {
		return connection.SetReadDeadline(time.Now().Add(90 * time.Second))
	})

	events, unsubscribe := h.direct.Subscribe(256)
	defer unsubscribe()
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		for {
			if _, _, err := connection.ReadMessage(); err != nil {
				return
			}
		}
	}()

	ping := time.NewTicker(30 * time.Second)
	defer ping.Stop()
	for {
		select {
		case message, ok := <-events:
			if !ok {
				return
			}
			_ = connection.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := connection.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ping.C:
			_ = connection.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := connection.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-closed:
			return
		case <-r.Context().Done():
			return
		}
	}
}
