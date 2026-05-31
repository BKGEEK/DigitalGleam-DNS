package main

import "log"

func main() {
	cfg, err := LoadConfig()
	if err != nil { log.Fatalf("❌ 加载配置失败: %v", err) }
	if err := InitDB(cfg); err != nil { log.Fatalf("❌ 数据库初始化失败: %v", err) }
	defer db.Close()

	StartWebUI()         // 启动 Web 界面 (8080端口)
	StartAPI()           // 启动 API 接口 (8081端口)
	StartDNSServer(cfg)  // 启动 DNS 服务 (53端口)
}
