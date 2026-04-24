package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type User struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Tag      string `json:"tag"`
	Bio      string `json:"bio"`
	PhotoURL string `json:"photoUrl"`
	Online   bool   `json:"online"`
	LastSeen int64  `json:"lastSeen"`
	FCMToken string `json:"fcmToken,omitempty"`
}

type Message struct {
	ID       string      `json:"id"`
	ChatID   string      `json:"chatId"`
	SenderID string      `json:"senderId"`
	Author   string      `json:"author"`
	Type     string      `json:"type"`
	Text     string      `json:"text"`
	FileURL  string      `json:"fileUrl,omitempty"`
	FileData string      `json:"fileData,omitempty"`
	FileName string      `json:"fileName,omitempty"`
	FileType string      `json:"fileType,omitempty"`
	FileSize int64       `json:"fileSize,omitempty"`
	Duration string      `json:"duration,omitempty"`
	VideoURL string      `json:"videoUrl,omitempty"`
	ReplyTo  interface{} `json:"replyTo,omitempty"`
	Time     string      `json:"time"`
	UnixMs   int64       `json:"unixMs"`
}

type WSMessage struct {
	Event   string          `json:"event"`
	Payload json.RawMessage `json:"payload"`
}

type Client struct {
	userID string
	conn   *websocket.Conn
	send   chan []byte
}

var (
	mu          sync.RWMutex
	users       = make(map[string]*User)
	messages    = make(map[string][]Message)
	clients     = make(map[string]*Client)
	chatMembers = make(map[string][]string)
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func broadcast(chatID, senderID string, data []byte) {
	mu.RLock()
	members := chatMembers[chatID]
	mu.RUnlock()
	for _, uid := range members {
		if uid == senderID {
			continue
		}
		mu.RLock()
		c := clients[uid]
		mu.RUnlock()
		if c != nil {
			select {
			case c.send <- data:
			default:
			}
		}
	}
}

func cors(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("userId")
	if userID == "" {
		http.Error(w, "userId required", 400)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := &Client{userID: userID, conn: conn, send: make(chan []byte, 512)}
	mu.Lock()
	clients[userID] = client
	if u, ok := users[userID]; ok {
		u.Online = true
	}
	mu.Unlock()
	log.Printf("Connected: %s | Online: %d", userID, len(clients))

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer func() { ticker.Stop(); conn.Close() }()
		for {
			select {
			case msg, ok := <-client.send:
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if !ok {
					conn.WriteMessage(websocket.CloseMessage, nil)
					return
				}
				conn.WriteMessage(websocket.TextMessage, msg)
			case <-ticker.C:
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()

	conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var ws WSMessage
		if err := json.Unmarshal(raw, &ws); err != nil {
			continue
		}
		switch ws.Event {
		case "join":
			var p struct {
				ChatID  string   `json:"chatId"`
				Members []string `json:"members"`
			}
			json.Unmarshal(ws.Payload, &p)
			mu.Lock()
			if len(p.Members) > 0 {
				chatMembers[p.ChatID] = p.Members
			} else {
				found := false
				for _, uid := range chatMembers[p.ChatID] {
					if uid == userID {
						found = true
						break
					}
				}
				if !found {
					chatMembers[p.ChatID] = append(chatMembers[p.ChatID], userID)
				}
			}
			mu.Unlock()
		case "message":
			var msg Message
			json.Unmarshal(ws.Payload, &msg)
			msg.SenderID = userID
			msg.UnixMs = time.Now().UnixMilli()
			if msg.Time == "" {
				msg.Time = time.Now().Format("15:04")
			}
			mu.Lock()
			messages[msg.ChatID] = append(messages[msg.ChatID], msg)
			if len(messages[msg.ChatID]) > 1000 {
				messages[msg.ChatID] = messages[msg.ChatID][len(messages[msg.ChatID])-1000:]
			}
			mu.Unlock()
			data, _ := json.Marshal(WSMessage{Event: "message", Payload: mustMarshal(msg)})
			broadcast(msg.ChatID, userID, data)
		case "typing":
			var p struct {
				ChatID string `json:"chatId"`
			}
			json.Unmarshal(ws.Payload, &p)
			data, _ := json.Marshal(WSMessage{Event: "typing", Payload: ws.Payload})
			broadcast(p.ChatID, userID, data)
		case "ping":
			client.send <- []byte(`{"event":"pong"}`)
		}
	}

	mu.Lock()
	delete(clients, userID)
	if u, ok := users[userID]; ok {
		u.Online = false
		u.LastSeen = time.Now().UnixMilli()
	}
	mu.Unlock()
	log.Printf("Disconnected: %s | Online: %d", userID, len(clients))
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	cors(w)
	if r.Method == "OPTIONS" {
		return
	}
	var u User
	json.NewDecoder(r.Body).Decode(&u)
	if u.ID == "" || u.Tag == "" {
		http.Error(w, `{"error":"id and tag required"}`, 400)
		return
	}
	u.Tag = strings.ToLower(strings.TrimSpace(u.Tag))
	mu.Lock()
	for _, ex := range users {
		if ex.Tag == u.Tag && ex.ID != u.ID {
			mu.Unlock()
			http.Error(w, `{"error":"tag taken"}`, 409)
			return
		}
	}
	users[u.ID] = &u
	mu.Unlock()
	log.Printf("Registered: %s (@%s)", u.Name, u.Tag)
	json.NewEncoder(w).Encode(u)
}

func updateUserHandler(w http.ResponseWriter, r *http.Request) {
	cors(w)
	if r.Method == "OPTIONS" {
		return
	}
	userID := strings.TrimPrefix(r.URL.Path, "/user/")
	var update map[string]interface{}
	json.NewDecoder(r.Body).Decode(&update)
	mu.Lock()
	if u, ok := users[userID]; ok {
		if fcm, ok := update["fcmToken"].(string); ok {
			u.FCMToken = fcm
		}
		if name, ok := update["name"].(string); ok {
			u.Name = name
		}
		if photo, ok := update["photoUrl"].(string); ok {
			u.PhotoURL = photo
		}
	}
	mu.Unlock()
	w.Write([]byte(`{"ok":true}`))
}

func userHandler(w http.ResponseWriter, r *http.Request) {
	cors(w)
	id := r.URL.Query().Get("id")
	tag := strings.ToLower(r.URL.Query().Get("tag"))
	mu.RLock()
	defer mu.RUnlock()
	if id != "" {
		if u, ok := users[id]; ok {
			json.NewEncoder(w).Encode(u)
			return
		}
	}
	if tag != "" {
		for _, u := range users {
			if u.Tag == tag {
				json.NewEncoder(w).Encode(u)
				return
			}
		}
	}
	http.Error(w, `{"error":"not found"}`, 404)
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	cors(w)
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if q == "" {
		fmt.Fprint(w, "[]")
		return
	}
	mu.RLock()
	defer mu.RUnlock()
	var result []*User
	for _, u := range users {
		if strings.Contains(strings.ToLower(u.Tag), q) ||
			strings.Contains(strings.ToLower(u.Name), q) {
			result = append(result, u)
			if len(result) >= 20 {
				break
			}
		}
	}
	if result == nil {
		fmt.Fprint(w, "[]")
		return
	}
	json.NewEncoder(w).Encode(result)
}

func historyHandler(w http.ResponseWriter, r *http.Request) {
	cors(w)
	chatID := r.URL.Query().Get("chatId")
	mu.RLock()
	msgs := messages[chatID]
	mu.RUnlock()
	if msgs == nil {
		fmt.Fprint(w, "[]")
		return
	}
	start := 0
	if len(msgs) > 100 {
		start = len(msgs) - 100
	}
	json.NewEncoder(w).Encode(msgs[start:])
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	cors(w)
	mu.RLock()
	o, u := len(clients), len(users)
	mu.RUnlock()
	fmt.Fprintf(w, `{"status":"ok","online":%d,"users":%d}`, o, u)
}

func mustMarshal(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func main() {
	http.HandleFunc("/ws", wsHandler)
	http.HandleFunc("/register", registerHandler)
	http.HandleFunc("/search", searchHandler)
	http.HandleFunc("/history", historyHandler)
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/user/", updateUserHandler)
	http.HandleFunc("/user", userHandler)
	log.Println("MRX Server on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
