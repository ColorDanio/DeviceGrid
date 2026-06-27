package api

import (
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// authenticateWS upgrades the WebSocket connection, then reads the first message
// which must contain a JWT token. This prevents token leakage in proxy logs.
// Returns the upgraded connection, or sends a 401 and returns nil.
func (r *Router) authenticateWS(c *gin.Context) (*websocket.Conn, bool) {
	// Step 1: Upgrade without auth check
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return nil, false
	}

	// Step 2: Read first message within 10 seconds — must contain token
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, msgBytes, err := conn.ReadMessage()
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","data":"auth timeout"}`))
		conn.Close()
		return nil, false
	}
	conn.SetReadDeadline(time.Time{}) // Reset deadline

	// Step 3: Parse token from first message
	// Accept both JSON format {"token":"xxx"} and plain text token
	token := ""
	var authMsg struct {
		Token string `json:"token"`
		Type  string `json:"type"`
	}
	if json.Unmarshal(msgBytes, &authMsg) == nil && authMsg.Token != "" {
		token = authMsg.Token
	} else {
		// Try plain text
		token = string(msgBytes)
	}

	if token == "" {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","data":"token required in first message"}`))
		conn.Close()
		return nil, false
	}

	// Step 4: Validate JWT
	if _, err := r.jm.Parse(token); err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"error","data":"invalid or expired token"}`))
		conn.Close()
		return nil, false
	}

	return conn, true
}
