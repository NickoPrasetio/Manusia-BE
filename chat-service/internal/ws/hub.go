package ws

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var wsActiveConnections = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "ws_active_connections",
	Help: "Current number of active WebSocket connections.",
})

func init() {
	prometheus.MustRegister(wsActiveConnections)
}

// Hub keeps track of which users currently have an open WebSocket connection
// (possibly more than one, e.g. multiple browser tabs) so messages and read
// receipts can be pushed to them in real time.
type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[*Client]bool
}

func NewHub() *Hub {
	return &Hub{clients: make(map[string]map[*Client]bool)}
}

func (h *Hub) Register(userID string, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[userID] == nil {
		h.clients[userID] = make(map[*Client]bool)
	}
	h.clients[userID][c] = true
	wsActiveConnections.Inc()
}

func (h *Hub) Unregister(userID string, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conns, ok := h.clients[userID]; ok {
		delete(conns, c)
		if len(conns) == 0 {
			delete(h.clients, userID)
		}
		wsActiveConnections.Dec()
	}
}

// SendToUser pushes a payload to every active connection of a user. It is a
// no-op (silently dropped) if the user has no open connection.
func (h *Hub) SendToUser(userID string, payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients[userID] {
		select {
		case c.send <- payload:
		default:
		}
	}
}

func (h *Hub) IsOnline(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[userID]) > 0
}
