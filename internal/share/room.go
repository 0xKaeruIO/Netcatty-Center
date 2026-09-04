package share

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Role string

const (
	RoleHost  Role = "host"
	RoleGuest Role = "guest"
)

type Message struct {
	Type      string          `json:"type"`
	Data      string          `json:"data,omitempty"`
	Pin       string          `json:"pin,omitempty"`
	Role      string          `json:"role,omitempty"`
	RoomID    string          `json:"roomId,omitempty"`
	Cols      int             `json:"cols,omitempty"`
	Rows      int             `json:"rows,omitempty"`
	Label     string          `json:"label,omitempty"`
	Message   string          `json:"message,omitempty"`
	ID        string          `json:"id,omitempty"`
	Source    string          `json:"source,omitempty"`
	RequestID string          `json:"requestId,omitempty"`
	Channel   string          `json:"channel,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
}

type connSlot struct {
	id   string
	conn *websocket.Conn
}

type Room struct {
	ID           string
	PIN          string
	Label        string
	Cols         int
	Rows         int
	OwnerKeyHash string
	HostToken    string
	CreatedAt    time.Time

	mu     sync.Mutex
	host   *connSlot
	guests map[string]*connSlot
	closed bool
}

func newRoom(id, pin, ownerKeyHash, hostToken, label string, cols, rows int) *Room {
	return &Room{
		ID:           id,
		PIN:          pin,
		Label:        label,
		Cols:         cols,
		Rows:         rows,
		OwnerKeyHash: ownerKeyHash,
		HostToken:    hostToken,
		CreatedAt:    time.Now(),
		guests:       make(map[string]*connSlot),
	}
}

func (r *Room) Closed() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.closed
}

func (r *Room) AttachHost(conn *websocket.Conn) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return ErrRoomClosed
	}
	if r.host != nil {
		return ErrHostAlreadyAttached
	}
	r.host = &connSlot{id: "host", conn: conn}
	return nil
}

func (r *Room) AttachGuest(guestID string, conn *websocket.Conn) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return ErrRoomClosed
	}
	if r.host == nil {
		return ErrHostNotAttached
	}
	r.guests[guestID] = &connSlot{id: guestID, conn: conn}
	return nil
}

func (r *Room) DetachGuest(guestID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.guests, guestID)
}

func (r *Room) HasHost() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.host != nil && !r.closed
}

func (r *Room) SetSize(cols, rows int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cols > 0 {
		r.Cols = cols
	}
	if rows > 0 {
		r.Rows = rows
	}
}

func (r *Room) GuestCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.guests)
}

func (r *Room) BroadcastToGuests(msg Message) {
	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}
	r.mu.Lock()
	conns := make([]*websocket.Conn, 0, len(r.guests))
	for _, guest := range r.guests {
		if guest != nil && guest.conn != nil {
			conns = append(conns, guest.conn)
		}
	}
	r.mu.Unlock()
	for _, conn := range conns {
		_ = conn.WriteMessage(websocket.TextMessage, payload)
	}
}

func (r *Room) SendToHost(msg Message) {
	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}
	r.mu.Lock()
	host := r.host
	r.mu.Unlock()
	if host == nil || host.conn == nil {
		return
	}
	_ = host.conn.WriteMessage(websocket.TextMessage, payload)
}

func (r *Room) Close() []*websocket.Conn {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	conns := make([]*websocket.Conn, 0, 1+len(r.guests))
	if r.host != nil && r.host.conn != nil {
		conns = append(conns, r.host.conn)
	}
	for _, guest := range r.guests {
		if guest != nil && guest.conn != nil {
			conns = append(conns, guest.conn)
		}
	}
	r.host = nil
	r.guests = make(map[string]*connSlot)
	return conns
}
