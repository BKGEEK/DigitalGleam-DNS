package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"
)

func generateAPIKey() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

func apiKeyMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientKey := r.Header.Get("X-API-Key")
		if clientKey == "" {
			http.Error(w, `{"error": "缺少 X-API-Key 请求头"}`, http.StatusUnauthorized)
			return
		}
		clientHash := hashKey(clientKey)
		var dbHash string
		var keyID int
		err := db.QueryRow("SELECT id, key_hash FROM api_keys WHERE key_hash = ?", clientHash).Scan(&keyID, &dbHash)
		if err != nil || subtle.ConstantTimeCompare([]byte(clientHash), []byte(dbHash)) != 1 {
			http.Error(w, `{"error": "无效的 API 密钥"}`, http.StatusUnauthorized)
			return
		}
		_, _ = db.Exec("UPDATE api_keys SET last_used_at = ? WHERE id = ?", time.Now(), keyID)
		next(w, r)
	}
}

func handleGetAPIKeys(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, name, created_at, last_used_at FROM api_keys ORDER BY id DESC")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	var keys []map[string]interface{}
	for rows.Next() {
		var id int
		var name string
		var createdAt, lastUsedAt time.Time
		_ = rows.Scan(&id, &name, &createdAt, &lastUsedAt)
		keys = append(keys, map[string]interface{}{"id": id, "name": name, "created_at": createdAt, "last_used_at": lastUsedAt})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(keys)
}

func handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var req struct{ Name string `json:"name"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
		http.Error(w, "名称不能为空", 400)
		return
	}
	plainKey, err := generateAPIKey()
	if err != nil {
		http.Error(w, "生成密钥失败", 500)
		return
	}
	keyHash := hashKey(plainKey)
	res, err := db.Exec("INSERT INTO api_keys (name, key_hash) VALUES (?, ?)", req.Name, keyHash)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	id, _ := res.LastInsertId()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "name": req.Name, "api_key": plainKey, "warning": "请立即复制并保存此密钥，关闭后将无法再次查看"})
}

func handleDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	var req struct{ ID int `json:"id"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID <= 0 {
		http.Error(w, "ID 无效", 400)
		return
	}
	_, _ = db.Exec("DELETE FROM api_keys WHERE id = ?", req.ID)
	_, _ = w.Write([]byte(`{"status": "success"}`))
}

func StartAPI() {
	http.HandleFunc("/api/records", apiKeyMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		rows, err := db.Query("SELECT id, zone, host, record_type, value, ttl, priority FROM dns_records ORDER BY id DESC")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer rows.Close()
		var records []map[string]interface{}
		for rows.Next() {
			var id int
			var zone, host, rType, value string
			var ttl, priority int
			_ = rows.Scan(&id, &zone, &host, &rType, &value, &ttl, &priority)
			records = append(records, map[string]interface{}{"id": id, "zone": zone, "host": host, "record_type": rType, "value": value, "ttl": ttl, "priority": priority})
		}
		_ = json.NewEncoder(w).Encode(records)
	}))

	http.HandleFunc("/api/records/add", apiKeyMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "请求体无效", 400)
			return
		}
		zone, _ := req["zone"].(string)
		host, _ := req["host"].(string)
		rType, _ := req["record_type"].(string)
		value, _ := req["value"].(string)
		ttl := 0
		priority := 0
		if raw, ok := req["ttl"].(float64); ok {
			ttl = int(raw)
		}
		if raw, ok := req["priority"].(float64); ok {
			priority = int(raw)
		}
		if zone == "" || rType == "" || value == "" {
			http.Error(w, "zone、record_type 和 value 不能为空", 400)
			return
		}
		if len(zone) > 0 && zone[len(zone)-1] != '.' {
			zone += "."
		}
		_, _ = db.Exec("INSERT INTO dns_records (zone, host, record_type, value, ttl, priority) VALUES (?, ?, ?, ?, ?, ?)", zone, host, rType, value, ttl, priority)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status": "success"}`))
	}))

	http.HandleFunc("/api/records/delete", apiKeyMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", 405)
			return
		}
		var req struct{ ID int `json:"id"` }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID <= 0 {
			http.Error(w, "ID 无效", 400)
			return
		}
		_, _ = db.Exec("DELETE FROM dns_records WHERE id = ?", req.ID)
		_, _ = w.Write([]byte(`{"status": "success"}`))
	}))

	go func() {
		println("🔌 安全 API 服务已启动: http://0.0.0.0:8081")
		_ = http.ListenAndServe(":8081", nil)
	}()
}
