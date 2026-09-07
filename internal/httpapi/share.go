package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"netcatty-center/internal/security"
	"netcatty-center/internal/share"
	"netcatty-center/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var shareUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (s *Server) createShareRoom(c *gin.Context) {
	auth, ok := s.requireClientAPIKey(c)
	if !ok {
		return
	}
	var body struct {
		Label string `json:"label"`
		Cols  int    `json:"cols"`
		Rows  int    `json:"rows"`
	}
	_ = c.ShouldBindJSON(&body)
	room, err := s.hub.CreateRoom(auth.Hash, strings.TrimSpace(body.Label), body.Cols, body.Rows)
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"roomId":    room.ID,
		"pin":       room.PIN,
		"hostToken": room.HostToken,
		"cols":      room.Cols,
		"rows":      room.Rows,
	})
}

func (s *Server) joinShareRoom(c *gin.Context) {
	auth, ok := s.requireClientAPIKey(c)
	if !ok {
		return
	}
	var body struct {
		Pin string `json:"pin"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, "请求无效")
		return
	}
	rateKey := auth.Hash + "|" + clientIP(c)
	room, guestToken, err := s.hub.JoinByPIN(strings.TrimSpace(body.Pin), rateKey)
	if err != nil {
		status := http.StatusUnauthorized
		if errors.Is(err, share.ErrRateLimited) {
			status = http.StatusTooManyRequests
		} else if errors.Is(err, share.ErrInvalidPIN) {
			status = http.StatusBadRequest
		}
		fail(c, status, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"roomId":     room.ID,
		"guestToken": guestToken,
		"label":      room.Label,
		"cols":       room.Cols,
		"rows":       room.Rows,
	})
}

func (s *Server) deleteShareRoom(c *gin.Context) {
	auth, ok := s.requireClientAPIKey(c)
	if !ok {
		return
	}
	room, ok := s.hub.CloseRoom(c.Param("id"), auth.Hash)
	if !ok {
		fail(c, http.StatusNotFound, "分享不存在或无权关闭")
		return
	}
	endShareConnections(room, "分享已关闭")
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) shareWS(c *gin.Context) {
	roomID := c.Param("id")
	role := strings.ToLower(strings.TrimSpace(c.Query("role")))
	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		fail(c, http.StatusUnauthorized, share.ErrInvalidToken.Error())
		return
	}
	conn, err := shareUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(8 << 20)
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	if role == string(share.RoleHost) {
		s.runShareHost(conn, roomID, token)
		return
	}
	if role == string(share.RoleGuest) {
		s.runShareGuest(conn, roomID, token)
		return
	}
	writeShareError(conn, share.ErrWrongRole.Error())
	_ = conn.Close()
}

func (s *Server) runShareHost(conn *websocket.Conn, roomID, token string) {
	room, err := s.hub.ResolveHost(roomID, token)
	if err != nil {
		writeShareError(conn, err.Error())
		_ = conn.Close()
		return
	}
	if err := room.AttachHost(conn); err != nil {
		writeShareError(conn, err.Error())
		_ = conn.Close()
		return
	}
	writeShareJSON(conn, share.Message{
		Type:   "hello",
		Role:   string(share.RoleHost),
		RoomID: room.ID,
		Cols:   room.Cols,
		Rows:   room.Rows,
		Label:  room.Label,
	})
	go pingLoop(conn)
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			closed := s.hub.CloseRoomByID(room.ID)
			if closed != nil {
				endShareConnections(closed, "分享端已断开")
			}
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		var msg share.Message
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		switch msg.Type {
		case "out", "snapshot", "resize", "sys-rpc-result":
			if msg.Type == "resize" {
				room.SetSize(msg.Cols, msg.Rows)
			}
			room.BroadcastToGuests(msg)
		case "ended":
			closed := s.hub.CloseRoomByID(room.ID)
			if closed != nil {
				endShareConnections(closed, "分享已关闭")
			}
			return
		}
	}
}

func (s *Server) runShareGuest(conn *websocket.Conn, roomID, token string) {
	room, guestID, err := s.hub.ResolveGuest(roomID, token)
	if err != nil {
		writeShareError(conn, err.Error())
		_ = conn.Close()
		return
	}
	if err := room.AttachGuest(guestID, conn); err != nil {
		writeShareError(conn, err.Error())
		_ = conn.Close()
		return
	}
	writeShareJSON(conn, share.Message{
		Type:   "hello",
		Role:   string(share.RoleGuest),
		RoomID: room.ID,
		Cols:   room.Cols,
		Rows:   room.Rows,
		Label:  room.Label,
	})
	room.SendToHost(share.Message{Type: "hello", Role: string(share.RoleGuest)})
	go pingLoop(conn)
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			room.SendToHost(share.Message{Type: "guest-left", ID: guestID})
			room.DetachGuest(guestID)
			s.hub.ForgetGuestToken(token)
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		var msg share.Message
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		if msg.Type == "in" || msg.Type == "capacity" || msg.Type == "sys-rpc" {
			if msg.Type == "capacity" {
				msg.ID = guestID
			}
			room.SendToHost(msg)
		}
	}
}

type clientAuth struct {
	ID         string
	Name       string
	Hash       string
	Permission string
}

func (s *Server) requireClientAPIKey(c *gin.Context) (clientAuth, bool) {
	plaintext := extractAPIKey(c)
	if plaintext == "" {
		fail(c, http.StatusUnauthorized, "缺少客户端密钥。请使用 Authorization: Bearer <key>")
		return clientAuth{}, false
	}
	if !security.ValidAPIKeyFormat(plaintext) {
		fail(c, http.StatusUnauthorized, "密钥格式无效")
		return clientAuth{}, false
	}
	key, ok, err := s.store.FindActiveAPIKeyByHash(security.SHA256Hex(plaintext))
	if err != nil {
		fail(c, http.StatusInternalServerError, err.Error())
		return clientAuth{}, false
	}
	if !ok {
		fail(c, http.StatusUnauthorized, "密钥无效或已吊销")
		return clientAuth{}, false
	}
	_ = s.store.TouchAPIKey(key.ID)
	return clientAuth{
		ID:         key.ID,
		Name:       key.Name,
		Hash:       security.SHA256Hex(plaintext),
		Permission: key.Permission,
	}, true
}

func (s *Server) requireClientCatalogWrite(c *gin.Context) (clientAuth, bool) {
	auth, ok := s.requireClientAPIKey(c)
	if !ok {
		return auth, false
	}
	if !store.KeyCanWrite(auth.Permission) {
		fail(c, http.StatusForbidden, "此密钥为只读，不能修改机器列表")
		return auth, false
	}
	return auth, true
}

func clientIP(c *gin.Context) string {
	if forwarded := strings.TrimSpace(c.GetHeader("X-Forwarded-For")); forwarded != "" {
		return strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	return c.ClientIP()
}

func writeShareJSON(conn *websocket.Conn, msg share.Message) {
	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}
	_ = conn.WriteMessage(websocket.TextMessage, payload)
}

func writeShareError(conn *websocket.Conn, message string) {
	writeShareJSON(conn, share.Message{Type: "error", Message: message})
}

func pingLoop(conn *websocket.Conn) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if err := conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(5*time.Second)); err != nil {
			return
		}
	}
}

func endShareConnections(room *share.Room, message string) {
	ended, _ := json.Marshal(share.Message{Type: "ended", Message: message})
	for _, conn := range room.Close() {
		_ = conn.WriteMessage(websocket.TextMessage, ended)
		_ = conn.Close()
	}
}
