// Package hub maintains the in-memory registry of active WebSocket
// connections, organized by party. It provides broadcast primitives so a
// state change on one connection can be propagated to all party members.
//
// The hub is the only mutable shared state outside SQLite. All access is
// guarded by a single sync.RWMutex; for the connection counts we expect
// (4-player parties, ~10 simultaneous parties for an MVP), this is fine.
// If we hit scale issues we can shard by party_id.
package hub

import (
	"context"
	"errors"
	"sync"
)

// Conn is an opaque interface over the WebSocket connection that the hub
// stores. We keep this generic so the package doesn't depend on a specific
// WebSocket library; the ws package supplies the implementation.
type Conn interface {
	// Send delivers a message to the client. Returns an error if the
	// connection is closed or unwritable.
	Send(ctx context.Context, payload []byte) error

	// Close terminates the connection.
	Close() error

	// DeviceID returns the authenticated device ID for this connection.
	DeviceID() string
}

// Hub holds active WebSocket connections grouped by party.
type Hub struct {
	mu         sync.RWMutex
	byParty    map[string]map[string]Conn // party_id → device_id → conn
}

// New returns an empty hub.
func New() *Hub {
	return &Hub{
		byParty: make(map[string]map[string]Conn),
	}
}

// Register adds a connection to a party. If the same device is already
// connected, the previous connection is closed and replaced (which is the
// expected behavior for reconnection).
func (h *Hub) Register(partyID string, conn Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	conns, ok := h.byParty[partyID]
	if !ok {
		conns = make(map[string]Conn)
		h.byParty[partyID] = conns
	}
	if existing, found := conns[conn.DeviceID()]; found {
		_ = existing.Close()
	}
	conns[conn.DeviceID()] = conn
}

// Unregister removes a specific connection from a party. Safe to call
// even if the conn was already replaced.
func (h *Hub) Unregister(partyID string, conn Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	conns, ok := h.byParty[partyID]
	if !ok {
		return
	}
	if existing, found := conns[conn.DeviceID()]; found && existing == conn {
		delete(conns, conn.DeviceID())
	}
	if len(conns) == 0 {
		delete(h.byParty, partyID)
	}
}

// Broadcast sends a payload to every connection in a party.
// errs is a slice of any send errors (one per failed connection).
func (h *Hub) Broadcast(ctx context.Context, partyID string, payload []byte) (errs []error) {
	conns := h.partyConnsCopy(partyID)
	for _, c := range conns {
		if err := c.Send(ctx, payload); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// SendToDevice sends a payload to a specific device's connection in a
// party. Returns ErrConnNotFound if the device isn't connected.
func (h *Hub) SendToDevice(ctx context.Context, partyID, deviceID string, payload []byte) error {
	h.mu.RLock()
	conns, ok := h.byParty[partyID]
	if !ok {
		h.mu.RUnlock()
		return ErrConnNotFound
	}
	c, ok := conns[deviceID]
	h.mu.RUnlock()
	if !ok {
		return ErrConnNotFound
	}
	return c.Send(ctx, payload)
}

// CountByParty returns the number of active connections in a party.
func (h *Hub) CountByParty(partyID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.byParty[partyID])
}

// PartyDeviceIDs returns the device IDs of all connections currently in a
// party. Useful for diagnostics and lobby UI.
func (h *Hub) PartyDeviceIDs(partyID string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	conns, ok := h.byParty[partyID]
	if !ok {
		return nil
	}
	ids := make([]string, 0, len(conns))
	for id := range conns {
		ids = append(ids, id)
	}
	return ids
}

// partyConnsCopy returns a snapshot slice of connections in a party. We
// copy under read-lock so Broadcast doesn't hold the lock while sending
// (which could block on slow clients).
func (h *Hub) partyConnsCopy(partyID string) []Conn {
	h.mu.RLock()
	defer h.mu.RUnlock()
	conns := h.byParty[partyID]
	out := make([]Conn, 0, len(conns))
	for _, c := range conns {
		out = append(out, c)
	}
	return out
}

// ErrConnNotFound is returned by SendToDevice when no connection matches.
var ErrConnNotFound = errors.New("hub: no active connection for that device in that party")
