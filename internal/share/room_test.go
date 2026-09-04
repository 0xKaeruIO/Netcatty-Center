package share

import (
	"encoding/json"
	"testing"

	"github.com/gorilla/websocket"
)

func TestRoomCloseReturnsHostAndGuests(t *testing.T) {
	room := newRoom("r1", "123456", "owner", "htok", "lab", 80, 24)
	hostConn := &websocket.Conn{}
	guestConn := &websocket.Conn{}
	if err := room.AttachHost(hostConn); err != nil {
		t.Fatal(err)
	}
	if err := room.AttachGuest("g1", guestConn); err != nil {
		t.Fatal(err)
	}
	if room.GuestCount() != 1 {
		t.Fatalf("guests=%d", room.GuestCount())
	}
	conns := room.Close()
	if len(conns) != 2 {
		t.Fatalf("close returned %d conns", len(conns))
	}
	if room.GuestCount() != 0 || room.HasHost() {
		t.Fatal("room should be empty after close")
	}
	if extra := room.Close(); extra != nil {
		t.Fatal("second close should be a no-op")
	}
}

func TestGuestDetachDoesNotCloseHost(t *testing.T) {
	room := newRoom("r1", "123456", "owner", "htok", "lab", 80, 24)
	if err := room.AttachHost(&websocket.Conn{}); err != nil {
		t.Fatal(err)
	}
	if err := room.AttachGuest("g1", &websocket.Conn{}); err != nil {
		t.Fatal(err)
	}
	room.DetachGuest("g1")
	if !room.HasHost() {
		t.Fatal("host should remain after guest leave")
	}
	if room.GuestCount() != 0 {
		t.Fatalf("guests=%d", room.GuestCount())
	}
}

func TestShareSystemRpcMessageRoundTrip(t *testing.T) {
	raw := []byte(`{"type":"sys-rpc","requestId":"r1","channel":"netcatty:system:listProcesses","payload":{"sessionId":"guest-1"}}`)
	var msg Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Type != "sys-rpc" || msg.RequestID != "r1" || msg.Channel != "netcatty:system:listProcesses" {
		t.Fatalf("decoded %#v", msg)
	}
	out, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	var again map[string]any
	if err := json.Unmarshal(out, &again); err != nil {
		t.Fatal(err)
	}
	payload, _ := again["payload"].(map[string]any)
	if payload["sessionId"] != "guest-1" {
		t.Fatalf("payload=%v", again["payload"])
	}

	resultRaw := []byte(`{"type":"sys-rpc-result","requestId":"r1","result":{"success":true,"processes":[{"pid":1}]}}`)
	var result Message
	if err := json.Unmarshal(resultRaw, &result); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	body, _ := decoded["result"].(map[string]any)
	if body["success"] != true {
		t.Fatalf("result=%v", decoded["result"])
	}
}
