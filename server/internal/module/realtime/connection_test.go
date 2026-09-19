package realtime

import (
	"errors"
	"testing"
	"time"
)

func TestConnectionSetBoundsReplacementAndIsolation(t *testing.T) {
	set := NewConnectionSet(3, 2)
	c1 := newTestConnection(1, 10, 100)
	c2 := newTestConnection(1, 10, 101)
	c3 := newTestConnection(2, 10, 200)
	if err := set.Attach(c1); err != nil {
		t.Fatal(err)
	}
	if err := set.Attach(c2); err != nil {
		t.Fatal(err)
	}
	if err := set.Attach(c3); err != nil {
		t.Fatal(err)
	}
	if err := set.Attach(newTestConnection(1, 10, 102)); !errors.Is(err, ErrConnectionLimit) {
		t.Fatalf("per-user err=%v", err)
	}
	if err := set.Attach(newTestConnection(1, 20, 201)); !errors.Is(err, ErrConnectionLimit) {
		t.Fatalf("total err=%v", err)
	}
	replacement := newTestConnection(1, 10, 100)
	if err := set.Attach(replacement); err != nil {
		t.Fatal(err)
	}
	if !c1.Closed() || set.Len() != 3 {
		t.Fatalf("old closed=%v len=%d", c1.Closed(), set.Len())
	}
	set.PublishUser(1, 10, []byte("user"))
	expectPayload(t, replacement, "user")
	expectPayload(t, c2, "user")
	expectNoPayload(t, c3)
	set.PublishPlatform(1, 10, []byte("platform"))
	expectPayload(t, replacement, "platform")
	expectPayload(t, c2, "platform")
	expectNoPayload(t, c3)
	set.ClosePlatformUser(1, 10)
	if set.Len() != 1 || !replacement.Closed() || !c2.Closed() || c3.Closed() {
		t.Fatalf("close isolation len=%d", set.Len())
	}
	set.CloseAll()
	if set.Len() != 0 || !c3.Closed() {
		t.Fatalf("close all len=%d", set.Len())
	}
}

func TestConnectionSetDetachesBeforeNetworkCloseCompletes(t *testing.T) {
	set := NewConnectionSet(2, 2)
	closeStarted := make(chan struct{})
	releaseClose := make(chan struct{})
	connection := NewConnection(1, 10, 100, 8, func(int, string) {
		close(closeStarted)
		<-releaseClose
	})
	if err := set.Attach(connection); err != nil {
		t.Fatal(err)
	}
	go func() {
		<-connection.Done()
		connection.finalizeClose()
	}()

	returned := make(chan struct{})
	go func() {
		set.CloseAll()
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("CloseAll waited for the network close callback")
	}
	select {
	case <-closeStarted:
	case <-time.After(time.Second):
		t.Fatal("connection close was not handed to its connection loop")
	}
	if set.Len() != 0 {
		t.Fatalf("connections=%d want 0", set.Len())
	}
	if err := set.Attach(newTestConnection(1, 11, 101)); err != nil {
		t.Fatalf("subscriber work remained blocked after detach: %v", err)
	}
	close(releaseClose)
}

func TestConnectionSetSlowConsumerAndDrain(t *testing.T) {
	set := NewConnectionSet(2, 2)
	connection := newTestConnectionWithCapacity(1, 10, 100, 1)
	if err := set.Attach(connection); err != nil {
		t.Fatal(err)
	}
	set.PublishUser(1, 10, []byte("first"))
	set.PublishUser(1, 10, []byte("overflow"))
	if !connection.ClosedWith(CloseSlowConsumer) {
		t.Fatal("slow consumer was not closed with 4008")
	}
	set.BeginDrain()
	if err := set.Attach(newTestConnection(1, 10, 101)); !errors.Is(err, ErrDraining) {
		t.Fatalf("drain err=%v", err)
	}
}

func newTestConnection(platformID, userID, sessionID int64) *Connection {
	return newTestConnectionWithCapacity(platformID, userID, sessionID, 8)
}
func newTestConnectionWithCapacity(platformID, userID, sessionID int64, capacity int) *Connection {
	return NewConnection(platformID, userID, sessionID, capacity, func(code int, reason string) {})
}
func expectPayload(t *testing.T, c *Connection, want string) {
	t.Helper()
	select {
	case got := <-c.Send():
		if string(got) != want {
			t.Fatalf("payload=%q", got)
		}
	default:
		t.Fatalf("missing %q", want)
	}
}
func expectNoPayload(t *testing.T, c *Connection) {
	t.Helper()
	select {
	case got := <-c.Send():
		t.Fatalf("unexpected %q", got)
	default:
	}
}
