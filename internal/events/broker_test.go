package events

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/gin-gonic/gin"
	"github.com/khmerstay/gin-api/internal/auth"
)

func TestWebSocketTargetsAuthenticatedUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokens := auth.NewTokenManager("test-secret-with-enough-entropy", time.Hour)
	broker := NewBroker()
	router := gin.New()
	router.GET("/ws", broker.WebSocketHandler(tokens, nil))
	server := httptest.NewServer(router)
	defer server.Close()

	tokenA, err := tokens.Sign("user-a", "a@example.com", "A", "traveler")
	if err != nil {
		t.Fatal(err)
	}
	tokenB, err := tokens.Sign("user-b", "b@example.com", "B", "owner")
	if err != nil {
		t.Fatal(err)
	}

	connect := func(token string) *websocket.Conn {
		t.Helper()
		url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?token=" + token
		connection, _, dialErr := websocket.Dial(context.Background(), url, nil)
		if dialErr != nil {
			t.Fatalf("connect websocket: %v", dialErr)
		}
		var connected Event
		if readErr := wsjson.Read(context.Background(), connection, &connected); readErr != nil {
			t.Fatalf("read connected event: %v", readErr)
		}
		if connected.Type != "connected" {
			t.Fatalf("connected event type = %q", connected.Type)
		}
		return connection
	}

	connectionA := connect(tokenA)
	defer connectionA.CloseNow()
	connectionB := connect(tokenB)
	defer connectionB.CloseNow()

	broker.Publish("user-a", Event{Type: "new_message", Data: map[string]any{"body": "private"}})

	readCtx, cancelRead := context.WithTimeout(context.Background(), time.Second)
	defer cancelRead()
	var delivered Event
	if err = wsjson.Read(readCtx, connectionA, &delivered); err != nil {
		t.Fatalf("read targeted event: %v", err)
	}
	if delivered.Type != "new_message" {
		t.Fatalf("target event type = %q", delivered.Type)
	}

	privateCtx, cancelPrivate := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancelPrivate()
	var leaked Event
	if readErr := wsjson.Read(privateCtx, connectionB, &leaked); readErr == nil {
		t.Fatalf("event leaked to another user: %#v", leaked)
	}
}

func TestWebSocketRejectsInvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tokens := auth.NewTokenManager("test-secret-with-enough-entropy", time.Hour)
	router := gin.New()
	router.GET("/ws", NewBroker().WebSocketHandler(tokens, nil))
	server := httptest.NewServer(router)
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?token=invalid"
	connection, response, err := websocket.Dial(context.Background(), url, nil)
	if connection != nil {
		connection.CloseNow()
	}
	if err == nil {
		t.Fatal("expected invalid token to reject the websocket handshake")
	}
	if response == nil || response.StatusCode != 401 {
		t.Fatalf("status = %v, want 401", response)
	}
}
