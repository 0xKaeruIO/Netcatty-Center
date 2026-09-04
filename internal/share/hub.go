package share

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	ErrRoomClosed          = errors.New("分享已结束")
	ErrHostAlreadyAttached = errors.New("分享端已连接")
	ErrHostNotAttached     = errors.New("分享端尚未连接")
	ErrRoomNotFound        = errors.New("分享不存在或密码错误")
	ErrRateLimited         = errors.New("尝试过于频繁，请稍后再试")
	ErrInvalidPIN          = errors.New("密码必须是 6 位数字")
	ErrInvalidToken        = errors.New("连接令牌无效")
	ErrWrongRole           = errors.New("角色不匹配")
)

const (
	joinLimitWindow   = time.Minute
	joinLimitMaxFails = 5
	roomTTL           = 12 * time.Hour
	orphanHostTTL     = 2 * time.Minute
)

type joinBucket struct {
	fails []time.Time
}

type Hub struct {
	mu         sync.Mutex
	rooms      map[string]*Room
	byPIN      map[string]*Room
	hostTokens map[string]string
	guestTokens map[string]guestToken
	joinFails  map[string]*joinBucket
}

type guestToken struct {
	roomID string
	id     string
}

func NewHub() *Hub {
	h := &Hub{
		rooms:       make(map[string]*Room),
		byPIN:       make(map[string]*Room),
		hostTokens:  make(map[string]string),
		guestTokens: make(map[string]guestToken),
		joinFails:   make(map[string]*joinBucket),
	}
	go h.gcLoop()
	return h
}

func randomToken() string {
	var buf [24]byte
	_, _ = rand.Read(buf[:])
	return hex.EncodeToString(buf[:])
}

func randomID() string {
	var buf [16]byte
	_, _ = rand.Read(buf[:])
	return hex.EncodeToString(buf[:])
}

func (h *Hub) CreateRoom(ownerKeyHash, label string, cols, rows int) (*Room, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	var pin string
	for i := 0; i < 64; i++ {
		candidate, err := GeneratePIN()
		if err != nil {
			return nil, err
		}
		if _, taken := h.byPIN[candidate]; !taken {
			pin = candidate
			break
		}
	}
	if pin == "" {
		return nil, errors.New("无法分配分享密码，请稍后重试")
	}
	if cols < 1 {
		cols = 80
	}
	if rows < 1 {
		rows = 24
	}
	room := newRoom(randomID(), pin, ownerKeyHash, randomToken(), label, cols, rows)
	h.rooms[room.ID] = room
	h.byPIN[pin] = room
	h.hostTokens[room.HostToken] = room.ID
	return room, nil
}

func (h *Hub) JoinByPIN(pin, rateKey string) (*Room, string, error) {
	if !ValidPIN(pin) {
		return nil, "", ErrInvalidPIN
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.isRateLimitedLocked(rateKey) {
		return nil, "", ErrRateLimited
	}
	room := h.byPIN[pin]
	if room == nil || room.Closed() {
		h.recordFailLocked(rateKey)
		return nil, "", ErrRoomNotFound
	}
	token := randomToken()
	guestID := randomID()
	h.guestTokens[token] = guestToken{roomID: room.ID, id: guestID}
	return room, token, nil
}

func (h *Hub) ResolveHost(roomID, token string) (*Room, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	id := h.hostTokens[token]
	if id == "" || id != roomID {
		return nil, ErrInvalidToken
	}
	room := h.rooms[id]
	if room == nil || room.Closed() {
		return nil, ErrRoomNotFound
	}
	return room, nil
}

func (h *Hub) ResolveGuest(roomID, token string) (*Room, string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	info, ok := h.guestTokens[token]
	if !ok || info.roomID != roomID {
		return nil, "", ErrInvalidToken
	}
	room := h.rooms[info.roomID]
	if room == nil || room.Closed() {
		return nil, "", ErrRoomNotFound
	}
	return room, info.id, nil
}

func (h *Hub) CloseRoom(roomID, ownerKeyHash string) (*Room, bool) {
	h.mu.Lock()
	room := h.rooms[roomID]
	if room == nil {
		h.mu.Unlock()
		return nil, false
	}
	if ownerKeyHash != "" && room.OwnerKeyHash != ownerKeyHash {
		h.mu.Unlock()
		return nil, false
	}
	h.removeRoomLocked(room)
	h.mu.Unlock()
	return room, true
}

func (h *Hub) CloseRoomByID(roomID string) *Room {
	h.mu.Lock()
	room := h.rooms[roomID]
	if room == nil {
		h.mu.Unlock()
		return nil
	}
	h.removeRoomLocked(room)
	h.mu.Unlock()
	return room
}

func (h *Hub) removeRoomLocked(room *Room) {
	delete(h.rooms, room.ID)
	delete(h.byPIN, room.PIN)
	delete(h.hostTokens, room.HostToken)
	for token, info := range h.guestTokens {
		if info.roomID == room.ID {
			delete(h.guestTokens, token)
		}
	}
}

func (h *Hub) ForgetGuestToken(token string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.guestTokens, token)
}

func (h *Hub) isRateLimitedLocked(key string) bool {
	if key == "" {
		return false
	}
	bucket := h.joinFails[key]
	if bucket == nil {
		return false
	}
	h.pruneFailsLocked(bucket)
	return len(bucket.fails) >= joinLimitMaxFails
}

func (h *Hub) recordFailLocked(key string) {
	if key == "" {
		return
	}
	bucket := h.joinFails[key]
	if bucket == nil {
		bucket = &joinBucket{}
		h.joinFails[key] = bucket
	}
	h.pruneFailsLocked(bucket)
	bucket.fails = append(bucket.fails, time.Now())
}

func (h *Hub) pruneFailsLocked(bucket *joinBucket) {
	cutoff := time.Now().Add(-joinLimitWindow)
	kept := bucket.fails[:0]
	for _, at := range bucket.fails {
		if at.After(cutoff) {
			kept = append(kept, at)
		}
	}
	bucket.fails = kept
}

func (h *Hub) gcLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		h.gc()
	}
}

func (h *Hub) gc() {
	now := time.Now()
	h.mu.Lock()
	stale := make([]*Room, 0)
	for _, room := range h.rooms {
		age := now.Sub(room.CreatedAt)
		if age > roomTTL || (!room.HasHost() && age > orphanHostTTL) || room.Closed() {
			stale = append(stale, room)
		}
	}
	for _, room := range stale {
		h.removeRoomLocked(room)
	}
	h.mu.Unlock()
	ended, _ := json.Marshal(Message{Type: "ended", Message: "分享已结束"})
	for _, room := range stale {
		for _, conn := range room.Close() {
			_ = conn.WriteMessage(websocket.TextMessage, ended)
			_ = conn.Close()
		}
	}
}
