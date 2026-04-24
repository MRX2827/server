[24.04.2026 15:01] MRX 2.0:    }
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
    ChatID string json:"chatId"
   }
   json.Unmarshal(ws.Payload, &p)
   data, _ := json.Marshal(WSMessage{Event: "typing", Payload: ws.Payload})
   broadcast(p.ChatID, userID, data)
  case "ping":
   client.send <- []byte({"event":"pong"})
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
  http.Error(w, {"error":"id and tag required"}, 400)
  return
 }
 u.Tag = strings.ToLower(strings.TrimSpace(u.Tag))
 mu.Lock()
 for _, ex := range users {
  if ex.Tag == u.Tag && ex.ID != u.ID {
   mu.Unlock()
   http.Error(w, {"error":"tag taken"}, 409)
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
 w.Write([]byte({"ok":true}))
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
 http.Error(w, {"error":"not found"}, 404)
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
 fmt.Fprintf(w, {"status":"ok","online":%d,"users":%d}, o, u)
}
[24.04.2026 15:01] MRX 2.0: func mustMarshal(v interface{}) json.RawMessage {
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
