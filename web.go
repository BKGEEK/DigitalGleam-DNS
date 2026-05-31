package main

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

var store = sessions.NewCookieStore([]byte("my_super_secret_key_change_it_in_production"))

type DNSRecord struct {
	ID         int
	Zone       string
	Host       string
	RecordType string
	Value      string
	TTL        int
	Priority   int
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, "admin-session")
		if auth, ok := session.Values["authenticated"].(bool); !ok || !auth {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func hasAdmin() bool {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM admins").Scan(&count)
	return err == nil && count > 0
}

func StartWebUI() {
	const baseTmpl = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>专属 DNS 管理系统</title>
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css" rel="stylesheet">
    <style>body { padding-top: 40px; background-color: #f8f9fa; }</style>
</head>
<body>
    <div class="container">{{.}}</div>
</body>
</html>`

	const initTmplStr = `...`
	const loginTmplStr = `...`
	const mainTmplStr = `
<div class="d-flex justify-content-between align-items-center mb-4">
    <h2>DNS 记录管理</h2>
    <a href="/logout" class="btn btn-sm btn-outline-danger">退出登录</a>
</div>
<div class="card mb-4 shadow-sm"><div class="card-body"><h5>新增记录</h5><form action="/add" method="POST" class="row g-3"><div class="col-md-3"><input name="zone" class="form-control" placeholder="域名，如 example.com" required></div><div class="col-md-2"><input name="host" class="form-control" placeholder="主机，如 www"></div><div class="col-md-2"><select name="type" class="form-select"><option value="A">A</option><option value="AAAA">AAAA</option><option value="CNAME">CNAME</option><option value="MX">MX</option><option value="NS">NS</option><option value="TXT">TXT</option><option value="SOA">SOA</option><option value="SRV">SRV</option><option value="PTR">PTR</option></select></div><div class="col-md-1"><input name="ttl" type="number" class="form-control" placeholder="TTL"></div><div class="col-md-1"><input name="priority" type="number" class="form-control" placeholder="优先级"></div><div class="col-md-3"><input name="value" class="form-control" placeholder="记录值" required></div><div class="col-md-12"><button type="submit" class="btn btn-success">添加</button></div></form></div></div>
<div class="card shadow-sm"><div class="card-body"><h5>现有记录</h5><table class="table table-striped"><thead><tr><th>ID</th><th>Zone</th><th>Host</th><th>Type</th><th>Value</th><th>TTL</th><th>Priority</th><th>操作</th></tr></thead><tbody>{{range .}}<tr><td>{{.ID}}</td><td>{{.Zone}}</td><td>{{.Host}}</td><td>{{.RecordType}}</td><td>{{.Value}}</td><td>{{.TTL}}</td><td>{{.Priority}}</td><td><form action="/delete" method="POST"><input type="hidden" name="id" value="{{.ID}}"><button class="btn btn-sm btn-danger">删除</button></form></td></tr>{{end}}</tbody></table></div></div>`

	baseTmplParsed := template.Must(template.New("base").Parse(baseTmpl))
	initTmpl := template.Must(template.New("init").Parse(initTmplStr))
	loginTmpl := template.Must(template.New("login").Parse(loginTmplStr))
	mainTmpl := template.Must(template.New("main").Parse(mainTmplStr))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { if r.URL.Path != "/" { http.NotFound(w, r); return }; if !hasAdmin() { _ = baseTmplParsed.Execute(w, template.HTML(initTmplStr)) } else { http.Redirect(w, r, "/login", http.StatusSeeOther) } })
	http.HandleFunc("/init-admin", func(w http.ResponseWriter, r *http.Request) { if r.Method != "POST" { http.Error(w, "Method not allowed", 405); return }; username := r.FormValue("username"); password := r.FormValue("password"); if username == "" || password == "" { http.Error(w, "用户名和密码不能为空", 400); return }; hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost); if err != nil { http.Error(w, "创建失败", 500); return }; _, _ = db.Exec("INSERT INTO admins (username, password_hash) VALUES (?, ?)", username, string(hashedPassword)); http.Redirect(w, r, "/login", http.StatusSeeOther) })
	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) { if r.Method == "GET" { _ = baseTmplParsed.Execute(w, template.HTML(loginTmplStr)); return }; username := r.FormValue("username"); password := r.FormValue("password"); var storedHash string; err := db.QueryRow("SELECT password_hash FROM admins WHERE username = ?", username).Scan(&storedHash); if err == nil && bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)) == nil { session, _ := store.Get(r, "admin-session"); session.Values["authenticated"] = true; _ = session.Save(r, w); http.Redirect(w, r, "/dashboard", http.StatusSeeOther); return }; http.Error(w, "用户名或密码错误", http.StatusUnauthorized) })
	http.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) { session, _ := store.Get(r, "admin-session"); session.Values["authenticated"] = false; _ = session.Save(r, w); http.Redirect(w, r, "/login", http.StatusSeeOther) })

	http.HandleFunc("/dashboard", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query("SELECT id, zone, host, record_type, value, ttl, priority FROM dns_records ORDER BY id DESC")
		if err != nil { http.Error(w, err.Error(), 500); return }
		defer rows.Close()
		var records []DNSRecord
		for rows.Next() { var rec DNSRecord; _ = rows.Scan(&rec.ID, &rec.Zone, &rec.Host, &rec.RecordType, &rec.Value, &rec.TTL, &rec.Priority); records = append(records, rec) }
		content := strings.Builder{}
		_ = mainTmpl.Execute(&content, records)
		_ = baseTmplParsed.Execute(w, template.HTML(content.String()))
	}))

	http.HandleFunc("/add", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			zone, host, recType, value := r.FormValue("zone"), strings.TrimSpace(r.FormValue("host")), strings.ToUpper(r.FormValue("type")), r.FormValue("value")
			ttl, _ := strconv.Atoi(r.FormValue("ttl"))
			priority, _ := strconv.Atoi(r.FormValue("priority"))
			if zone == "" || value == "" { http.Error(w, "zone 和 value 不能为空", 400); return }
			if len(zone) > 0 && zone[len(zone)-1] != '.' { zone += "." }
			if host == "" { host = "@" }
			_, _ = db.Exec("INSERT INTO dns_records (zone, host, record_type, value, ttl, priority) VALUES (?, ?, ?, ?, ?, ?)", zone, host, recType, value, ttl, priority)
		}
		http.Redirect(w, r, "/dashboard", 302)
	}))

	http.HandleFunc("/delete", authMiddleware(func(w http.ResponseWriter, r *http.Request) { if r.Method == "POST" { _, _ = db.Exec("DELETE FROM dns_records WHERE id = ?", r.FormValue("id")) }; http.Redirect(w, r, "/dashboard", 302) }))

	http.HandleFunc("/admin/api-keys", authMiddleware(func(w http.ResponseWriter, r *http.Request) { if r.Method == "GET" { handleGetAPIKeys(w, r); return }; if r.Method == "POST" { var req struct { ID int `json:"id"`; Name string `json:"name"` }; if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.ID > 0 { handleDeleteAPIKey(w, r); return }; if req.Name != "" { handleCreateAPIKey(w, r); return }; http.Error(w, "请求体无效", 400) } }))

	go func() { println("🔐 安全 Web 管理界面已启动 http://0.0.0.0:8080"); _ = http.ListenAndServe(":8080", nil) }()
}
