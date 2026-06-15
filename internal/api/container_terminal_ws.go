package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
)

func (r *Router) handleContainerTerminalWS(conn *websocket.Conn, nodeID, containerID string) {
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

	session, err := r.sshMgr.NewContainerPTYSession(ctx, nodeID, containerID, initialCols, initialRows)
	if err != nil {
		errJSON, _ := json.Marshal(wsServerMsg{Type: "error", Data: err.Error()})
		conn.WriteMessage(websocket.TextMessage, errJSON)
		return
	}
	defer session.Close()

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
