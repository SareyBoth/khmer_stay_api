package events

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/gin-gonic/gin"
	"github.com/khmerstay/gin-api/internal/auth"
	"github.com/khmerstay/gin-api/internal/http/middleware"
)

type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type client struct {
	userID string
	ch     chan Event
}

type Broker struct {
	mu      sync.RWMutex
	clients map[*client]bool
}

// ginWebSocketWriter lets coder/websocket mark Gin's response as written while
// bypassing Gin 1.11's guard that otherwise rejects hijacking after the 101.
type ginWebSocketWriter struct {
	gin.ResponseWriter
}

func (w ginWebSocketWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	underlying := http.ResponseWriter(w.ResponseWriter)
	for {
		unwrapper, ok := underlying.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			break
		}
		underlying = unwrapper.Unwrap()
	}
	hijacker, ok := underlying.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func NewBroker() *Broker {
	return &Broker{
		clients: make(map[*client]bool),
	}
}

func requestClaims(c *gin.Context, tokens *auth.TokenManager) (auth.Claims, bool) {
	raw := strings.TrimSpace(c.Query("token"))
	if raw == "" {
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		if strings.HasPrefix(header, "Bearer ") {
			raw = strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		}
	}
	if raw == "" {
		return auth.Claims{}, false
	}
	claims, err := tokens.Parse(raw)
	return claims, err == nil
}

func (b *Broker) subscribe(userID string) (*client, func()) {
	c := &client{
		userID: userID,
		ch:     make(chan Event, 16),
	}
	b.mu.Lock()
	b.clients[c] = true
	b.mu.Unlock()

	unsubscribe := func() {
		b.mu.Lock()
		delete(b.clients, c)
		close(c.ch)
		b.mu.Unlock()
	}
	return c, unsubscribe
}

// Publish sends an event to a specific user (or to all users if targetUserID is empty)
func (b *Broker) Publish(targetUserID string, event Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for c := range b.clients {
		if targetUserID == "" || c.userID == targetUserID {
			select {
			case c.ch <- event:
			default:
				// Avoid blocking broker if client buffer is full
			}
		}
	}
}

// Handler handles the SSE endpoint GET /events
func (b *Broker) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := middleware.Current(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("Transfer-Encoding", "chunked")

		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			c.String(http.StatusInternalServerError, "streaming unsupported")
			return
		}

		client, unsubscribe := b.subscribe(claims.UserID)
		defer unsubscribe()

		// Send initial connect event
		fmt.Fprintf(c.Writer, "event: connected\ndata: {\"status\":\"connected\",\"time\":\"%s\"}\n\n", time.Now().UTC().Format(time.RFC3339))
		flusher.Flush()

		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		ctx := c.Request.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Heartbeat ping
				fmt.Fprint(c.Writer, ": ping\n\n")
				flusher.Flush()
			case ev, open := <-client.ch:
				if !open {
					return
				}
				raw, err := json.Marshal(ev.Data)
				if err != nil {
					continue
				}
				fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", ev.Type, raw)
				flusher.Flush()
			}
		}
	}
}

// WebSocketHandler streams authenticated real-time events to one user. Clients
// send normal chat messages through the REST endpoint so validation, permissions,
// persistence, and delivery all share one code path.
func (b *Broker) WebSocketHandler(tokens *auth.TokenManager, allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := requestClaims(c, tokens)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		connection, err := websocket.Accept(ginWebSocketWriter{c.Writer}, c.Request, &websocket.AcceptOptions{
			OriginPatterns:  allowedOrigins,
			CompressionMode: websocket.CompressionContextTakeover,
		})
		if err != nil {
			log.Printf("websocket accept: %v", err)
			return
		}
		defer connection.CloseNow()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		client, unsubscribe := b.subscribe(claims.UserID)
		defer unsubscribe()
		if err = wsjson.Write(ctx, connection, Event{Type: "connected", Data: map[string]any{"time": time.Now().UTC()}}); err != nil {
			log.Printf("websocket connected event: %v", err)
			return
		}
		go func() {
			for {
				if _, _, readErr := connection.Read(ctx); readErr != nil {
					cancel()
					return
				}
			}
		}()

		ping := time.NewTicker(25 * time.Second)
		defer ping.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ping.C:
				pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
				err = connection.Ping(pingCtx)
				pingCancel()
				if err != nil {
					return
				}
			case event, open := <-client.ch:
				if !open || wsjson.Write(ctx, connection, event) != nil {
					return
				}
			}
		}
	}
}

// QueryAuthToken allows SSE connections to authenticate via ?token= query parameter as well as Bearer header
func QueryAuthToken(tokens *auth.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := requestClaims(c, tokens)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}
		c.Set(middleware.ClaimsKey, claims)
		c.Next()
	}
}
