package share

import (
	"errors"
	"testing"
)

func TestGeneratePINIsSixDigits(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		pin, err := GeneratePIN()
		if err != nil {
			t.Fatal(err)
		}
		if !ValidPIN(pin) {
			t.Fatalf("invalid pin %q", pin)
		}
		seen[pin] = true
	}
	if len(seen) < 2 {
		t.Fatal("expected some PIN variety")
	}
}

func TestCreateRoomPinsAreUnique(t *testing.T) {
	hub := NewHub()
	pins := map[string]bool{}
	for i := 0; i < 20; i++ {
		room, err := hub.CreateRoom("owner", "host", 80, 24)
		if err != nil {
			t.Fatal(err)
		}
		if pins[room.PIN] {
			t.Fatalf("duplicate pin %s", room.PIN)
		}
		pins[room.PIN] = true
	}
}

func TestJoinByPINAndWrongPIN(t *testing.T) {
	hub := NewHub()
	room, err := hub.CreateRoom("owner", "lab", 100, 30)
	if err != nil {
		t.Fatal(err)
	}
	joined, token, err := hub.JoinByPIN(room.PIN, "key|127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if joined.ID != room.ID || token == "" {
		t.Fatalf("join=%+v token=%q", joined, token)
	}
	_, _, err = hub.JoinByPIN("000000", "key|127.0.0.1")
	if !errors.Is(err, ErrRoomNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestJoinRateLimit(t *testing.T) {
	hub := NewHub()
	key := "hash|10.0.0.1"
	for i := 0; i < joinLimitMaxFails; i++ {
		_, _, err := hub.JoinByPIN("111111", key)
		if !errors.Is(err, ErrRoomNotFound) {
			t.Fatalf("fail %d: %v", i, err)
		}
	}
	_, _, err := hub.JoinByPIN("111111", key)
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected rate limit, got %v", err)
	}
}

func TestCloseRoomDropsPIN(t *testing.T) {
	hub := NewHub()
	room, err := hub.CreateRoom("owner", "lab", 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	closed, ok := hub.CloseRoom(room.ID, "owner")
	if !ok || closed.ID != room.ID {
		t.Fatal("close failed")
	}
	_, _, err = hub.JoinByPIN(room.PIN, "other")
	if !errors.Is(err, ErrRoomNotFound) {
		t.Fatalf("pin should die after close, err=%v", err)
	}
}

func TestCloseRoomRejectsOtherOwner(t *testing.T) {
	hub := NewHub()
	room, err := hub.CreateRoom("owner", "lab", 80, 24)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := hub.CloseRoom(room.ID, "someone-else"); ok {
		t.Fatal("other owner should not close")
	}
	if _, _, err := hub.JoinByPIN(room.PIN, "x"); err != nil {
		t.Fatalf("room should still exist: %v", err)
	}
}
