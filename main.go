// RedMrxGram WebSocket Server
// Запуск: go mod init redmrxgram && go get github.com/gorilla/websocket && go run main.go

package main

import (
 "encoding/json"
 "fmt"
 "log"
 "net/http"
 "sync"
 "time"

 "github.com/gorilla/websocket"
)

// ── Типы сообщений ──────────────────────────────────────────────────────────

type Message struct {
 Type    string json:"type"
 ChatID  string json:"chatId"
 UserID  string json:"userId"
 Content string json:"content"
 Time    int64  json:"time"
}

// ── Hub — сердце сервера ────────────────────────────────────────────────────

type Client struct {
 id   string
 conn *websocket.Conn
 send chan []byte
 hub  *Hub
}

type Hub struct {
 clients    map[string]*Client  // userId -> Client
 chats      map[string][]*Client // chatId -> []Clients
 register   chan *Client
 unregister chan *Client
 broadcast  chan *Message
 mu         sync.RWMutex
}

func newHub() *Hub {
 return &Hub{
  clients:    make(map[string]*Client),
  chats:      make(map[string][]*Client),
  register:   make(chan *Client),
  unregister: make(chan *Client),
  broadcast:  make(chan *Message, 256),
 }
}

func (h *Hub) run() {
 for {
  select {
  case client := <-h.register:
   h.mu.Lock()
   h.clients[client.id] = client
   h.mu.Unlock()
   log.Printf("✅ User connected: %s | Total: %d", client.id, len(h.clients))

  case client := <-h.unregister:
   h.mu.Lock()
   if _, ok := h.clients[client.id]; ok {
    delete(h.clients, client.id)
    close(client.send)
   }
   h.mu.Unlock()
   log.Printf("❌ User disconnected: %s | Total: %d", client.id, len(h.clients))

  case msg := <-h.broadcast:
   h.mu.RLock()
   members := h.chats[msg.ChatID]
   h.mu.RUnlock()

   data, _ := json.Marshal(msg)
   for _, c := range members {
    if c.id != msg.UserID { // не отправляем себе
     select {
     case c.send <- data:
     default:
      close(c.send)
     }
    }
   }
  }
 }
}

// ── Client goroutines ───────────────────────────────────────────────────────

var upgrader = websocket.Upgrader{
 ReadBufferSize:  1024,
 WriteBufferSize: 1024,
 CheckOrigin:     func(r *http.Request) bool { return true }, // TODO: проверять origin в проде
}

func (c *Client) readPump() {
 defer func() {
  c.hub.unregister <- c
  c.conn.Close()
 }()

 c.conn.SetReadLimit(512 * 1024) // 512KB max message
 c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
 c.conn.SetPongHandler(func(string) error {
  c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
  return nil
 })

 for {
  _, data, err := c.conn.ReadMessage()
  if err != nil {
   break
  }

  var msg Message
  if err := json.Unmarshal(data, &msg); err != nil {
   continue
  }

  msg.UserID = c.id
  msg.Time = time.Now().UnixMilli()

  // TODO: сохранить в PostgreSQL/TimescaleDB
  log.Printf("📨 [%s] %s: %s", msg.ChatID, msg.UserID, msg.Content)

  c.hub.broadcast <- &msg
 }
}

func (c *Client) writePump() {
 ticker := time.NewTicker(54 * time.Second)
 defer func() {
  ticker.Stop()
  c.conn.Close()
 }()

 for {
  select {
  case message, ok := <-c.send:
   c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
   if !ok {
    c.conn.WriteMessage(websocket.CloseMessage, []byte{})
    return
   }
   if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
    return
   }

  case <-ticker.C:
   // Ping каждые 54 секунды чтобы держать соединение живым
   c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
   if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
    return
   }
  }
 }
}

// ── HTTP Handlers ──────────────────────────────────────────────────────────

func wsHandler(hub *Hub, w http.ResponseWriter, r *http.Request) {
 userID := r.URL.Query().Get("userId")
 chatID := r.URL.Query().Get("chatId")

 if userID == "" || chatID == "" {
  http.Error(w, "userId and chatId required", http.StatusBadRequest)
  return
 }

 conn, err := upgrader.Upgrade(w, r, nil)
 if err != nil {
  log.Printf("Upgrade error: %v", err)
  return
 }

 client := &Client{id: userID, conn: conn, send: make(chan []byte, 256), hub: hub}
[26.04.2026 06:09] MRX 2.0:  // Добавляем в чат
 hub.mu.Lock()
 hub.chats[chatID] = append(hub.chats[chatID], client)
 hub.mu.Unlock()

 hub.register <- client

 // Каждый клиент — 2 горутины (читаем и пишем параллельно)
 go client.writePump()
 go client.readPump()
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
 fmt.Fprintf(w, {"status":"ok","server":"RedMrxGram v0.1"})
}

// ── Main ───────────────────────────────────────────────────────────────────

func main() {
 hub := newHub()
 go hub.run()

 http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
  wsHandler(hub, w, r)
 })
 http.HandleFunc("/health", healthHandler)

 port := ":8080"
 log.Printf("🚀 RedMrxGram WebSocket Server started on %s", port)
 log.Printf("   Connect: ws://localhost%s/ws?userId=USER_ID&chatId=CHAT_ID", port)
 log.Printf("   Health:  http://localhost%s/health", port)

 if err := http.ListenAndServe(port, nil); err != nil {
  log.Fatal("Server error:", err)
 }
}
