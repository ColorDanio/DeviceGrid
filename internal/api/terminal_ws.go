package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
)

type wsClientMsg struct {
	Type string `json:"type"`
	Data string `json:"data"`
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

type wsServerMsg struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

func (r *Router) handleTerminalWS(conn *websocket.Conn, nodeID string) {
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn.SetReadDeadline(time.Time{})
	_, msgBytes, err := conn.ReadMessage()
	if err != nil {
		return
	}

	initialCols := uint16(80)
	initialRows := uint16(24)
	var initMsg wsClientMsg
	if json.Unmarshal(msgBytes, &initMsg) == nil {
		if initMsg.Cols > 0 {
			initialCols = initMsg.Cols
		}
		if initMsg.Rows > 0 {
			initialRows = initMsg.Rows
		}
	}

	session, err := r.sshMgr.NewPTYSession(ctx, nodeID, initialCols, initialRows)
	if err != nil {
		slog.Error("ws terminal pty", "node", nodeID, "error", err)
		errJSON, _ := json.Marshal(wsServerMsg{Type: "error", Data: err.Error()})
		conn.WriteMessage(websocket.TextMessage, errJSON)
		return
	}
	defer session.Close()

	// Server-side keepalive: send ping every 30s to prevent idle disconnect
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					cancel()
					return
				}
			}
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			_, raw, err := conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}
			var msg wsClientMsg
			if json.Unmarshal(raw, &msg) != nil {
				continue
			}
			switch msg.Type {
			case "input":
				if msg.Data != "" {
					session.Write([]byte(msg.Data))
				}
			case "resize":
				if msg.Cols > 0 && msg.Rows > 0 {
					session.Resize(msg.Cols, msg.Rows)
				}
			}
		}
	}()

	go func() {
		for {
			data, err := session.Read()
			if err != nil {
				cancel()
				return
			}
			encoded := base64.StdEncoding.EncodeToString(data)
			out, _ := json.Marshal(wsServerMsg{Type: "output", Data: encoded})
			if err := conn.WriteMessage(websocket.TextMessage, out); err != nil {
				cancel()
				return
			}
		}
	}()

	select {
	case <-ctx.Done():
	case <-session.Done():
	}
}
