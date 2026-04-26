package main

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	fcmProjectID   = "redmrxgram"
	fcmClientEmail = "firebase-adminsdk-fbsvc@redmrxgram.iam.gserviceaccount.com"
	fcmPrivateKey  = `-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCeVP/rBRKYebmE
vBOAFP7umqu8KL57BVHtzHRIq1Pvh2gFRyks+T9KnaadxHjv0/y/WAjG55rNEsIk
iazAl1STdpBChtWEXYdlEnNkk/UZLOIReRfCq+u6z9C2MdhXuqShN5E0H3HJs5R5
7sCAj5nJhNiV6vm2F3hf2BIKqIm2dDHZQoSRFwbm04Ht1zTfgeb/lK1BGeReuBWC
vw/CGqv8mEQdY67luWlm8Tm51ZjZZDbvNw53YMbR0nBEPt2MwGl5W0OYB9sNgb58
L9zeuXF2/gFj1EGOsIIn5QxaXdxdnybzAzw/XZJPOeoemM9COZ27Pv0GnhzFzFCB
2EeOUItpAgMBAAECggEACdfmOKveL/dKWLWSH0l+32HjiR092VtXcHmgZZesLlUs
Wv6lcU+2FvsV3ZNWQu+6lAFCu23w7myq//dEs4z887WAarhj/BiN9zNGVAOMTnI/
RD+TYfWLAFhU5QzUgsC+ZSz1lKhbTENDOPDmTB/RW2lMR0a+W0saf1fpKDs544s8
+Mx/3A3YtxyXr5gknlnxwsQZplVjyax5onn0m6DQDHEKgItPq99ZoJFnqnoyqAZe
aTy4ORfd0jpqotdMJmGEjlI3pr7Ux3EAQfRxLw5AFVSeu2vuOzjquqDrjpUC879V
wbfIUI+D2qbQ7P36/du0yOHGYO4MN8qa2byIaeprkQKBgQDLbe76UArvA4VxWApd
4FOi3wDlYaG8CuPGoG2rLUogufnRB2u2VtLQuWsR3RNeuWTFdD+Rpj4THi82ANmL
SSfHrclIrXLLtwdQM/29N1gyjHOKAj9lwBROZESjSB2lwoJGpKNxWseeiiJaT3Wd
DXVQs1CloR9e3FIN/Nvs/wCtWQKBgQDHP5roLSHhBdHJ2zdlnAU84WqjnYZloatO
iJ63vd9OYkXMKwFj1vWlBYSR+hADHhZ1nNszOKkw10GQb75Ss1T3WcfQHVPs6ZbH
aV3CaWSFHiDvv5lTZcwrX47GRCyXifsgoHYciXF92w6GW25P/ZqUWuuNdUlDREnU
yzkmU4m8kQKBgQCkfgssSobcx+siQH95c0gNvebak/yUsfWGifjD3oY/OkJ/vFFj
iodDEXs7YZklEiIi66HvYw50pQal00AVOZ06ABNvehkGEsSOHMxDMTpW/Wz7nl+n
Jg8FaFx773dRrptrBfvHUBFz59xpTDEdQmGnVKeUllloehy7hMhMtdHVWQKBgHpV
VRjkTP7KAep7y+F5D8Y3aLAYUaIoxvMq8rhBvc90iwI2DV5tZtjxMFooPJiNaqC/
s94ZFdhE8Z5q3WINdUeBOitPNm4pZUf+K3DoIK2SuAo7izonMFoZC6IzzWUldKit
nJcc1C+/xYU8sdgvDy+zOxjZQCgqz2H1fJtJgzHhAoGAH2PXJ/xN2ao04udM54NB
DCZfPvh/X1HwLtaj7jmGKn6/+mULuPkpuePwsrqBjtd8n0R13H9wkn/w901aFdaK
1kNsPfEPzsmshbEU0dz/EEkOcasIbwiPApFf+2A9zc/unKqY9oH1tR3RIYq1fWDB
9hpNo9Prp7vWyIilzlrKqZQ=
-----END PRIVATE KEY-----`
	uploadDir = "/tmp/uploads"
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

var (
	fcmTokenMu     sync.Mutex
	fcmAccessToken string
	fcmTokenExpiry time.Time
)

func b64url(data []byte) string {
	const alpha = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	out := make([]byte, 0, len(data)*4/3+4)
	for i := 0; i < len(data); i += 3 {
		b0 := data[i]
		var b1, b2 byte
		if i+1 < len(data) {
			b1 = data[i+1]
		}
		if i+2 < len(data) {
			b2 = data[i+2]
		}
		out = append(out, alpha[b0>>2])
		out = append(out, alpha[(b0&0x3)<<4|(b1>>4)])
		if i+1 < len(data) {
			out = append(out, alpha[(b1&0xf)<<2|(b2>>6)])
		}
		if i+2 < len(data) {
			out = append(out, alpha[b2&0x3f])
		}
	}
	return string(out)
}

func getAccessToken() (string, error) {
	fcmTokenMu.Lock()
	defer fcmTokenMu.Unlock()
	if fcmAccessToken != "" && time.Now().Before(fcmTokenExpiry) {
		return fcmAccessToken, nil
	}
	block, _ := pem.Decode([]byte(fcmPrivateKey))
	if block == nil {
		return "", fmt.Errorf("bad PEM")
	}
	keyIface, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}
	rsaKey := keyIface.(*rsa.PrivateKey)
	now := time.Now().Unix()
	headerJSON, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	claimsJSON, _ := json.Marshal(map[string]interface{}{
		"iss":   fcmClientEmail,
		"scope": "https://www.googleapis.com/auth/firebase.messaging",
		"aud":   "https://oauth2.googleapis.com/token",
		"iat":   now,
		"exp":   now + 3600,
	})
	sigInput := b64url(headerJSON) + "." + b64url(claimsJSON)
	h := sha256.Sum256([]byte(sigInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, rsaKey, crypto.SHA256, h[:])
	if err != nil {
		return "", err
	}
	jwt := sigInput + "." + b64url(sig)
	body := "grant_type=urn%3Aietf%3Aparams%3Aoauth%3Agrant-type%3Ajwt-bearer&assertion=" + jwt
	resp, err := http.Post("https://oauth2.googleapis.com/token", "application/x-www-form-urlencoded", strings.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	token, ok := result["access_token"].(string)
	if !ok {
		return "", fmt.Errorf("no token: %v", result)
	}
	fcmAccessToken = token
	fcmTokenExpiry = time.Now().Add(55 * time.Minute)
	return token, nil
}

func sendFCM(token, title, body, chatID, senderID string) {
	accessToken, err := getAccessToken()
	if err != nil {
		log.Println("FCM auth error:", err)
		return
	}
	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"token":        token,
			"notification": map[string]string{"title": title, "body": body},
			"data":         map[string]string{"chatId": chatID, "senderId": senderID},
			"android": map[string]interface{}{
				"priority": "high",
				"notification": map[string]interface{}{
					"channel_id":              "messages",
					"default_sound":           true,
					"default_vibrate_timings": true,
				},
			},
		},
	}
	payloadBytes, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", fcmProjectID)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(payloadBytes))
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("FCM send error:", err)
		return
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == 200 {
		log.Printf("🔔 FCM sent: %s", title)
	} else {
		log.Printf("FCM error %d: %s", resp.StatusCode, string(b))
	}
}

func notifyOffline(chatID string, msg Message) {
	mu.RLock()
	members := chatMembers[chatID]
	mu.RUnlock()
	title := msg.Author
	body := msg.Text
	switch msg.Type {
	case "voice":
		body = "🎙 Голосовое сообщение"
	case "circle":
		body = "⭕ Видео-кружок"
	case "image":
		body = "🖼 Фото"
	case "video":
		body = "🎥 Видео"
	case "file":
		body = "📎 " + msg.FileName
	case "audio":
		body = "🎵 Аудио"
	}
	if len(body) > 100 {
		body = body[:100]
	}
	for _, uid := range members {
		if uid == msg.SenderID {
			continue
		}
		mu.RLock()
		_, online := clients[uid]
		u := users[uid]
		mu.RUnlock()
		if !online && u != nil && u.FCMToken != "" {
			go sendFCM(u.FCMToken, title, body, chatID, msg.SenderID)
		}
	}
}

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
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
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
	log.Printf("✅ Connected: %s | Online: %d", userID, len(clients))

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
			go notifyOffline(msg.ChatID, msg)
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
	log.Printf("❌ Disconnected: %s | Online: %d", userID, len(clients))
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
	if r.Method == "OPTIONS" {
		return
	}
	r.ParseMultipartForm(500 << 20)
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"error":"no file"}`, 400)
		return
	}
	defer file.Close()
	os.MkdirAll(uploadDir, 0755)
	filename := fmt.Sprintf("%d_%s", time.Now().UnixMilli(), header.Filename)
	filename = strings.ReplaceAll(filename, " ", "_")
	savePath := filepath.Join(uploadDir, filename)
	dst, err := os.Create(savePath)
	if err != nil {
		http.Error(w, `{"error":"save failed"}`, 500)
		return
	}
	defer dst.Close()
	written, err := io.Copy(dst, file)
	if err != nil {
		http.Error(w, `{"error":"write failed"}`, 500)
		return
	}
	host := r.Host
	scheme := "https"
	if strings.Contains(host, "localhost") {
		scheme = "http"
	}
	fileURL := fmt.Sprintf("%s://%s/files/%s", scheme, host, filename)
	log.Printf("📁 Uploaded: %s (%d bytes)", filename, written)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": fileURL, "filename": filename})
}

func serveFileHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	filename := strings.TrimPrefix(r.URL.Path, "/files/")
	if filename == "" || strings.Contains(filename, "..") {
		http.Error(w, "invalid", 400)
		return
	}
	http.ServeFile(w, r, filepath.Join(uploadDir, filename))
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
	log.Printf("👤 Registered: %s (@%s)", u.Name, u.Tag)
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
			log.Printf("📱 FCM saved for %s", userID)
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
		if strings.Contains(strings.ToLower(u.Tag), q) || strings.Contains(strings.ToLower(u.Name), q) {
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
	fmt.Fprintf(w, `{"status":"ok","online":%d,"users":%d,"fcm":"enabled","upload":"enabled"}`, o, u)
}

func mustMarshal(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	os.MkdirAll(uploadDir, 0755)
	http.HandleFunc("/ws", wsHandler)
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/files/", serveFileHandler)
	http.HandleFunc("/register", registerHandler)
	http.HandleFunc("/search", searchHandler)
	http.HandleFunc("/history", historyHandler)
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/user/", updateUserHandler)
	http.HandleFunc("/user", userHandler)
	log.Printf("🚀 MRX Server + FCM + Upload on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
