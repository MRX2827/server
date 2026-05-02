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
	"mime/multipart"
	"net/http"
	"os"
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

	tgBotToken = "8757655447:AAEMIWQxvldjXDyCpiXUt2_pUGpEh2Kw3L4"
	tgChatID   = "-1003906305436"
)

// ── Types ─────────────────────────────────────────────────────────────────────

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

// ── Storage ───────────────────────────────────────────────────────────────────

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

// ── Telegram Upload ───────────────────────────────────────────────────────────

type TGResponse struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result"`
}

// uploadToTelegram sends file to Telegram channel and returns a direct URL
func uploadToTelegram(fileData []byte, fileName, mimeType string) (string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// chat_id field
	w.WriteField("chat_id", tgChatID)

	// Determine method and field name
	method := "sendDocument"
	field := "document"
	if strings.HasPrefix(mimeType, "video/") {
		method = "sendVideo"
		field = "video"
	} else if strings.HasPrefix(mimeType, "image/") {
		method = "sendPhoto"
		field = "photo"
	} else if strings.HasPrefix(mimeType, "audio/") {
		method = "sendAudio"
		field = "audio"
	}

	// File field
	part, err := w.CreateFormFile(field, fileName)
	if err != nil {
		return "", err
	}
	part.Write(fileData)
	w.Close()

	// Send to Telegram
	url := fmt.Sprintf("https://api.telegram.org/bot%s/%s", tgBotToken, method)
	resp, err := http.Post(url, w.FormDataContentType(), &buf)
	if err != nil {
		return "", fmt.Errorf("telegram request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var tgResp TGResponse
	if err := json.Unmarshal(body, &tgResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if !tgResp.OK {
		return "", fmt.Errorf("telegram error: %s", string(body))
	}

	// Extract file_id from result
	var result map[string]interface{}
	json.Unmarshal(tgResp.Result, &result)

	fileID := ""
	// Try different result structures
	for _, key := range []string{"video", "document", "audio"} {
		if obj, ok := result[key].(map[string]interface{}); ok {
			if id, ok := obj["file_id"].(string); ok {
				fileID = id
				break
			}
		}
	}
	// For photos, it's an array
	if fileID == "" {
		if photos, ok := result["photo"].([]interface{}); ok && len(photos) > 0 {
			if photo, ok := photos[len(photos)-1].(map[string]interface{}); ok {
				fileID, _ = photo["file_id"].(string)
			}
		}
	}

	if fileID == "" {
		return "", fmt.Errorf("no file_id in response: %s", string(body))
	}

	// Get file path from Telegram
	fileURL, err := getTelegramFileURL(fileID)
	if err != nil {
		// Return proxy URL as fallback
		return fmt.Sprintf("https://server-production-ecd3.up.railway.app/tgfile/%s", fileID), nil
	}
	return fileURL, nil
}

func getTelegramFileURL(fileID string) (string, error) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/getFile?file_id=%s", tgBotToken, fileID)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var tgResp TGResponse
	json.NewDecoder(resp.Body).Decode(&tgResp)
	if !tgResp.OK {
		return "", fmt.Errorf("getFile failed")
	}
	var result map[string]interface{}
	json.Unmarshal(tgResp.Result, &result)
	filePath, _ := result["file_path"].(string)
	if filePath == "" {
		return "", fmt.Errorf("no file_path")
	}
	return fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", tgBotToken, filePath), nil
}

// Proxy handler for large files (Telegram URLs expire)
func tgFileProxyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	fileID := strings.TrimPrefix(r.URL.Path, "/tgfile/")
	if fileID == "" {
		http.Error(w, "no file id", 400)
		return
	}
	fileURL, err := getTelegramFileURL(fileID)
	if err != nil {
		http.Error(w, "file not found", 404)
		return
	}
	http.Redirect(w, r, fileURL, http.StatusFound)
}

// ── FCM ───────────────────────────────────────────────────────────────────────

func b64url(data []byte) string {
	const alpha = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	out := make([]byte, 0, len(data)*4/3+4)
	for i := 0; i < len(data); i += 3 {
		b0 := data[i]
		var b1, b2 byte
		if i+1 < len(data) { b1 = data[i+1] }
		if i+2 < len(data) { b2 = data[i+2] }
		out = append(out, alpha[b0>>2])
		out = append(out, alpha[(b0&0x3)<<4|(b1>>4)])
		if i+1 < len(data) { out = append(out, alpha[(b1&0xf)<<2|(b2>>6)]) }
		if i+2 < len(data) { out = append(out, alpha[b2&0x3f]) }
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
	if block == nil { return "", fmt.Errorf("bad PEM") }
	keyIface, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil { return "", err }
	rsaKey := keyIface.(*rsa.PrivateKey)
	now := time.Now().Unix()
	headerJSON, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	claimsJSON, _ := json.Marshal(map[string]interface{}{
		"iss": fcmClientEmail, "scope": "https://www.googleapis.com/auth/firebase.messaging",
		"aud": "https://oauth2.googleapis.com/token", "iat": now, "exp": now + 3600,
	})
	sigInput := b64url(headerJSON) + "." + b64url(claimsJSON)
	h := sha256.Sum256([]byte(sigInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, rsaKey, crypto.SHA256, h[:])
	if err != nil { return "", err }
	jwt := sigInput + "." + b64url(sig)
	body := "grant_type=urn%3Aietf%3Aparams%3Aoauth%3Agrant-type%3Ajwt-bearer&assertion=" + jwt
	resp, err := http.Post("https://oauth2.googleapis.com/token", "application/x-www-form-urlencoded", strings.NewReader(body))
	if err != nil { return "", err }
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	token, ok := result["access_token"].(string)
	if !ok { return "", fmt.Errorf("no token: %v", result) }
	fcmAccessToken = token
	fcmTokenExpiry = time.Now().Add(55 * time.Minute)
	return token, nil
}

func sendFCM(token, title, body, chatID, senderID string) {
	accessToken, err := getAccessToken()
	if err != nil { log.Println("FCM auth error:", err); return }
	payload := map[string]interface{}{
		"message": map[string]interface{}{
			"token": token,
			"notification": map[string]string{"title": title, "body": body},
			"data": map[string]string{"chatId": chatID, "senderId": senderID},
			"android": map[string]interface{}{
				"priority": "high",
				"notification": map[string]interface{}{
					"channel_id": "messages", "default_sound": true, "default_vibrate_timings": true,
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
	if err != nil { log.Println("FCM send error:", err); return }
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
	case "voice":  body = "🎙 Голосовое сообщение"
	case "circle": body = "⭕ Видео-кружок"
	case "image":  body = "🖼 Фото"
	case "video":  body = "🎥 Видео"
	case "file":   body = "📎 " + msg.FileName
	case "audio":  body = "🎵 Аудио"
	}
	if len(body) > 100 { body = body[:100] }
	for _, uid := range members {
		if uid == msg.SenderID { continue }
		mu.RLock()
		_, online := clients[uid]
		u := users[uid]
		mu.RUnlock()
		if !online && u != nil && u.FCMToken != "" {
			go sendFCM(u.FCMToken, title, body, chatID, msg.SenderID)
		}
	}
}

// ── WebSocket ─────────────────────────────────────────────────────────────────

var upgrader = websocket.Upgrader{
	ReadBufferSize: 4096, WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool { return true },
}

func broadcast(chatID, senderID string, data []byte) {
	mu.RLock()
	members := chatMembers[chatID]
	mu.RUnlock()
	for _, uid := range members {
		if uid == senderID { continue }
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
	if userID == "" { http.Error(w, "userId required", 400); return }
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil { return }
	client := &Client{userID: userID, conn: conn, send: make(chan []byte, 512)}
	mu.Lock()
	clients[userID] = client
	if u, ok := users[userID]; ok { u.Online = true }
	mu.Unlock()
	log.Printf("✅ Connected: %s | Online: %d", userID, len(clients))

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer func() { ticker.Stop(); conn.Close() }()
		for {
			select {
			case msg, ok := <-client.send:
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if !ok { conn.WriteMessage(websocket.CloseMessage, nil); return }
				conn.WriteMessage(websocket.TextMessage, msg)
			case <-ticker.C:
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil { return }
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
		if err != nil { break }
		var ws WSMessage
		if err := json.Unmarshal(raw, &ws); err != nil { continue }
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
					if uid == userID { found = true; break }
				}
				if !found { chatMembers[p.ChatID] = append(chatMembers[p.ChatID], userID) }
			}
			mu.Unlock()
		case "message":
			var msg Message
			json.Unmarshal(ws.Payload, &msg)
			msg.SenderID = userID
			msg.UnixMs = time.Now().UnixMilli()
			if msg.Time == "" { msg.Time = time.Now().Format("15:04") }
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
			var p struct{ ChatID string `json:"chatId"` }
			json.Unmarshal(ws.Payload, &p)
			data, _ := json.Marshal(WSMessage{Event: "typing", Payload: ws.Payload})
			broadcast(p.ChatID, userID, data)
		case "ping":
			client.send <- []byte(`{"event":"pong"}`)
		}
	}

	mu.Lock()
	delete(clients, userID)
	if u, ok := users[userID]; ok { u.Online = false; u.LastSeen = time.Now().UnixMilli() }
	mu.Unlock()
	log.Printf("❌ Disconnected: %s | Online: %d", userID, len(clients))
}

// ── HTTP Handlers ─────────────────────────────────────────────────────────────

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" { return }

	// Max 500MB
	r.ParseMultipartForm(500 << 20)
	file, header, err := r.FormFile("file")
	if err != nil { http.Error(w, `{"error":"no file"}`, 400); return }
	defer file.Close()

	// Read file into memory
	fileData, err := io.ReadAll(file)
	if err != nil { http.Error(w, `{"error":"read failed"}`, 500); return }

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" { mimeType = "application/octet-stream" }

	log.Printf("📤 Uploading to Telegram: %s (%d bytes)", header.Filename, len(fileData))

	fileURL, err := uploadToTelegram(fileData, header.Filename, mimeType)
	if err != nil {
		log.Println("Telegram upload error:", err)
		http.Error(w, `{"error":"`+err.Error()+`"}`, 500)
		return
	}

	log.Printf("✅ Uploaded to Telegram: %s", fileURL)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": fileURL})
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	cors(w)
	if r.Method == "OPTIONS" { return }
	var u User
	json.NewDecoder(r.Body).Decode(&u)
	if u.ID == "" || u.Tag == "" { http.Error(w, `{"error":"id and tag required"}`, 400); return }
	u.Tag = strings.ToLower(strings.TrimSpace(u.Tag))
	mu.Lock()
	for _, ex := range users {
		if ex.Tag == u.Tag && ex.ID != u.ID { mu.Unlock(); http.Error(w, `{"error":"tag taken"}`, 409); return }
	}
	users[u.ID] = &u
	mu.Unlock()
	log.Printf("👤 Registered: %s (@%s)", u.Name, u.Tag)
	json.NewEncoder(w).Encode(u)
}

func updateUserHandler(w http.ResponseWriter, r *http.Request) {
	cors(w)
	if r.Method == "OPTIONS" { return }
	userID := strings.TrimPrefix(r.URL.Path, "/user/")
	var update map[string]interface{}
	json.NewDecoder(r.Body).Decode(&update)
	mu.Lock()
	if u, ok := users[userID]; ok {
		if fcm, ok := update["fcmToken"].(string); ok { u.FCMToken = fcm }
		if name, ok := update["name"].(string); ok { u.Name = name }
		if photo, ok := update["photoUrl"].(string); ok { u.PhotoURL = photo }
	}
	mu.Unlock()
	w.Write([]byte(`{"ok":true}`))
}

func userHandler(w http.ResponseWriter, r *http.Request) {
	cors(w)
	id := r.URL.Query().Get("id")
	tag := strings.ToLower(r.URL.Query().Get("tag"))
	mu.RLock(); defer mu.RUnlock()
	if id != "" { if u, ok := users[id]; ok { json.NewEncoder(w).Encode(u); return } }
	if tag != "" { for _, u := range users { if u.Tag == tag { json.NewEncoder(w).Encode(u); return } } }
	http.Error(w, `{"error":"not found"}`, 404)
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	cors(w)
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if q == "" { fmt.Fprint(w, "[]"); return }
	mu.RLock(); defer mu.RUnlock()
	var result []*User
	for _, u := range users {
		if strings.Contains(strings.ToLower(u.Tag), q) || strings.Contains(strings.ToLower(u.Name), q) {
			result = append(result, u)
			if len(result) >= 20 { break }
		}
	}
	if result == nil { fmt.Fprint(w, "[]"); return }
	json.NewEncoder(w).Encode(result)
}

func historyHandler(w http.ResponseWriter, r *http.Request) {
	cors(w)
	chatID := r.URL.Query().Get("chatId")
	mu.RLock(); msgs := messages[chatID]; mu.RUnlock()
	if msgs == nil { fmt.Fprint(w, "[]"); return }
	start := 0
	if len(msgs) > 100 { start = len(msgs) - 100 }
	json.NewEncoder(w).Encode(msgs[start:])
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	cors(w)
	mu.RLock(); o, u := len(clients), len(users); mu.RUnlock()
	fmt.Fprintf(w, `{"status":"ok","online":%d,"users":%d,"storage":"telegram","fcm":"enabled"}`, o, u)
}

func mustMarshal(v interface{}) json.RawMessage { b, _ := json.Marshal(v); return b }

// POST /notify — called by app after sending message to Firebase
// Sends FCM push to offline users
func notifyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" { return }

	var req struct {
		ChatID     string   `json:"chatId"`
		SenderID   string   `json:"senderId"`
		SenderName string   `json:"senderName"`
		Members    []string `json:"members"`
		Text       string   `json:"text"`
		Type       string   `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"bad request"}`, 400)
		return
	}

	// Build notification body
	body := req.Text
	switch req.Type {
	case "voice":  body = "🎙 Голосовое сообщение"
	case "circle": body = "⭕ Видео-кружок"
	case "image":  body = "🖼 Фото"
	case "video":  body = "🎥 Видео"
	case "file":   body = "📎 Файл"
	case "audio":  body = "🎵 Аудио"
	}
	if len(body) > 100 { body = body[:100] }
	if body == "" { body = "Новое сообщение" }

	sent := 0
	mu.RLock()
	for _, uid := range req.Members {
		if uid == req.SenderID { continue }
		u := users[uid]
		_, online := clients[uid]
		if u != nil && u.FCMToken != "" && !online {
			go sendFCM(u.FCMToken, req.SenderName, body, req.ChatID, req.SenderID)
			sent++
		}
	}
	mu.RUnlock()

	log.Printf("🔔 Notify: chat=%s sender=%s sent=%d", req.ChatID, req.SenderName, sent)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"sent":%d}`, sent)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" { port = "8080" }

	http.HandleFunc("/ws",       wsHandler)
	http.HandleFunc("/upload",   uploadHandler)
	http.HandleFunc("/tgfile/",  tgFileProxyHandler)
	http.HandleFunc("/register", registerHandler)
	http.HandleFunc("/search",   searchHandler)
	http.HandleFunc("/history",  historyHandler)
	http.HandleFunc("/health",   healthHandler)
	http.HandleFunc("/user/",    updateUserHandler)
	http.HandleFunc("/user",     userHandler)

	log.Printf("🚀 MRX Server | Telegram Storage | FCM | Port: %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
