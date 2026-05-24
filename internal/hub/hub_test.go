package hub

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

// fakeConn is a minimal hub.Conn implementation used by the reconnect /
// stale-client tests. Send/Close behavior is controlled per-conn.
type fakeConn struct {
	device     string
	mu         sync.Mutex
	sendErr    error
	sendCalls  int32
	closed     atomic.Bool
	sendBlocks chan struct{} // if non-nil, Send waits on this before returning
}

func (c *fakeConn) Send(ctx context.Context, payload []byte) error {
	atomic.AddInt32(&c.sendCalls, 1)
	if c.sendBlocks != nil {
		select {
		case <-c.sendBlocks:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sendErr
}

func (c *fakeConn) Close() error {
	c.closed.Store(true)
	return nil
}

func (c *fakeConn) DeviceID() string { return c.device }

// TestBroadcast_EvictsConnOnSendError pins the spectator-reconnect leak:
// a connection whose underlying socket is silently dead (TCP RST never
// surfaced, NAT idle-timed-out, browser tab suspended) will fail Send,
// but prior to this fix the hub kept it registered. Every subsequent
// Broadcast would try the dead conn again, paying the write-timeout cost
// per broadcast and leaving the stale entry visible to PartyDeviceIDs
// (so the lobby UI showed phantom viewers).
//
// After the fix: any Send error during Broadcast evicts the offending
// conn from the hub. Future broadcasts skip it; PartyDeviceIDs no
// longer lists it.
func TestBroadcast_EvictsConnOnSendError(t *testing.T) {
	h := New()
	live := &fakeConn{device: "live"}
	dead := &fakeConn{device: "dead", sendErr: errors.New("write: broken pipe")}
	h.Register("party-1", live)
	h.Register("party-1", dead)

	if got := h.CountByParty("party-1"); got != 2 {
		t.Fatalf("setup: want 2 conns, got %d", got)
	}

	errs := h.Broadcast(context.Background(), "party-1", []byte("hello"))
	if len(errs) != 1 {
		t.Fatalf("want 1 send error, got %d (%v)", len(errs), errs)
	}

	// The dead conn must be evicted AND closed so the read-loop side can
	// observe the close and run its cleanup defer.
	if got := h.CountByParty("party-1"); got != 1 {
		t.Fatalf("dead conn should be evicted, got %d conns", got)
	}
	if !dead.closed.Load() {
		t.Fatal("evicted conn should have Close() called")
	}
	ids := h.PartyDeviceIDs("party-1")
	if len(ids) != 1 || ids[0] != "live" {
		t.Fatalf("PartyDeviceIDs should only show live conn, got %v", ids)
	}

	// Second broadcast must NOT re-touch the evicted conn.
	prevDeadCalls := atomic.LoadInt32(&dead.sendCalls)
	prevLiveCalls := atomic.LoadInt32(&live.sendCalls)
	_ = h.Broadcast(context.Background(), "party-1", []byte("again"))
	if got := atomic.LoadInt32(&dead.sendCalls); got != prevDeadCalls {
		t.Fatalf("dead conn should not receive subsequent broadcasts, got %d new calls", got-prevDeadCalls)
	}
	if got := atomic.LoadInt32(&live.sendCalls); got != prevLiveCalls+1 {
		t.Fatalf("live conn should have received one more broadcast, got %d new calls", got-prevLiveCalls)
	}
}

// TestBroadcast_EvictsOnlyTheFailingConn confirms a single bad conn
// doesn't take down its peers in the same party.
func TestBroadcast_EvictsOnlyTheFailingConn(t *testing.T) {
	h := New()
	live1 := &fakeConn{device: "live1"}
	dead := &fakeConn{device: "dead", sendErr: errors.New("connection reset by peer")}
	live2 := &fakeConn{device: "live2"}
	h.Register("party-2", live1)
	h.Register("party-2", dead)
	h.Register("party-2", live2)

	_ = h.Broadcast(context.Background(), "party-2", []byte("ping"))

	if got := h.CountByParty("party-2"); got != 2 {
		t.Fatalf("only the dead conn should be evicted, got %d remaining", got)
	}
	for _, c := range []*fakeConn{live1, live2} {
		if c.closed.Load() {
			t.Fatalf("live conn %s should NOT be closed", c.device)
		}
	}
	if !dead.closed.Load() {
		t.Fatal("dead conn should be closed")
	}
}

// TestBroadcast_DoesNotEvictOnReplaceRace confirms the eviction matches by
// pointer identity, not just device ID — so a fresh reconnect that races
// with a still-in-flight broadcast on the OLD conn won't accidentally
// evict the NEW conn. (Same shape as the Unregister identity check.)
func TestBroadcast_DoesNotEvictOnReplaceRace(t *testing.T) {
	h := New()
	old := &fakeConn{device: "phoenix", sendErr: errors.New("write timeout")}
	h.Register("party-3", old)

	// Snapshot in a way that mirrors the in-flight Broadcast iteration:
	// copy the conn pointer, then a reconnect replaces it before Send
	// returns the error.
	snapshot := []Conn{old}

	fresh := &fakeConn{device: "phoenix"}
	h.Register("party-3", fresh) // replaces old; closes old

	// Manually invoke the same eviction path that Broadcast uses on the
	// stale snapshot. The hub must NOT evict `fresh` just because the
	// device IDs match.
	for _, c := range snapshot {
		if err := c.Send(context.Background(), []byte("x")); err != nil {
			h.evictIfSame("party-3", c)
		}
	}

	if got := h.CountByParty("party-3"); got != 1 {
		t.Fatalf("fresh conn must survive the eviction race, got %d", got)
	}
	if h.PartyDeviceIDs("party-3")[0] != "phoenix" {
		t.Fatal("fresh conn for the device should still be registered")
	}
}
