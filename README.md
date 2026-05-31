# 数星DNS / DGDNS

一套基于 Go + MySQL 的轻量 DNS 管理系统，支持 Web 后台、API 管理和常见 DNS 记录解析。

## 项目简介

本项目提供：

- DNS 权威解析服务
- Web 后台管理界面
- API 密钥管理与接口调用
- MySQL 数据库存储
- 主从同步相关配置

## 功能特性

- 支持常见 DNS 记录：A、AAAA、CNAME、MX、NS、TXT、SOA、SRV、PTR
- 支持 Web 页面增删改查 DNS 记录
- 支持 API 方式管理记录
- 支持管理员登录与初始化
- 支持 TTL、Priority 等字段
- 支持 MySQL 数据持久化

## 项目结构

```text
dgdns/
├── api.go
├── config.go
├── config.yaml
├── db.go
├── dns_server.go
├── init.sql
├── install.sh
├── main.go
├── README.md
└── web.go
```

## 环境要求

- Go 1.21+
- MySQL 8.0+
- 常见的 Linux 和 Unix 发行版
- root 权限（用于安装脚本部署）

## 快速开始

### 1. 配置数据库

先修改 `config.yaml`：

```yaml
role: master

database:
  host: 127.0.0.1
  port: 3306
  user: root
  password: your_db_password
  dbname: dns_db
```

### 2. 初始化数据库

执行 `init.sql` 创建表结构。

### 3. 一键部署

```bash
sudo bash install.sh
```

## 使用指南

### 1. 首次访问与管理员配置

部署完成后，在浏览器访问：`http://你的服务器IP:8080`

系统会自动检测数据库，首次访问将强制跳转到“创建管理员账号”页面。设置好账号密码后，即可登录 Web 后台管理 DNS 记录。

### 2. API 密钥管理

登录 Web 后台后，在页面底部找到“API 密钥管理”模块。
点击“生成新密钥”，请务必在弹窗中复制并保存好明文密钥（出于安全考虑，系统只展示一次明文，数据库中仅存储哈希值）。
可随时在后台删除失效或泄露的密钥。

### 3. API 接口调用示例

所有 API 请求都需要在 HTTP Header 中携带 `X-API-Key`。

获取所有 DNS 记录：

```bash
curl -X GET http://你的服务器IP:8081/api/records \
  -H "X-API-Key: 你的API密钥"
```

常见记录格式示例：

- `A`：`value=1.2.3.4`
- `AAAA`：`value=2001:db8::1`
- `CNAME`：`value=target.example.com.`
- `MX`：`value=mail.example.com.`，`priority=10`
- `NS`：`value=ns1.example.com.`
- `TXT`：`value=verification-token`
- `SOA`：`value=ns1.example.com. admin.example.com. 2026053101 3600 600 604800 300`
- `SRV`：`value=10 20 443 service.example.com.`
- `PTR`：`value=host.example.com.`

每种记录的 API 添加示例：

```bash
# A 记录
curl -X POST http://你的服务器IP:8081/api/records/add \
  -H "X-API-Key: 你的API密钥" \
  -H "Content-Type: application/json" \
  -d '{"zone":"example.com","host":"www","record_type":"A","value":"1.2.3.4","ttl":300,"priority":0}'

# AAAA 记录
curl -X POST http://你的服务器IP:8081/api/records/add \
  -H "X-API-Key: 你的API密钥" \
  -H "Content-Type: application/json" \
  -d '{"zone":"example.com","host":"ipv6","record_type":"AAAA","value":"2001:db8::1","ttl":300,"priority":0}'

# CNAME 记录
curl -X POST http://你的服务器IP:8081/api/records/add \
  -H "X-API-Key: 你的API密钥" \
  -H "Content-Type: application/json" \
  -d '{"zone":"example.com","host":"blog","record_type":"CNAME","value":"target.example.com.","ttl":300,"priority":0}'

# MX 记录
curl -X POST http://你的服务器IP:8081/api/records/add \
  -H "X-API-Key: 你的API密钥" \
  -H "Content-Type: application/json" \
  -d '{"zone":"example.com","host":"@","record_type":"MX","value":"mail.example.com.","ttl":300,"priority":10}'

# NS 记录
curl -X POST http://你的服务器IP:8081/api/records/add \
  -H "X-API-Key: 你的API密钥" \
  -H "Content-Type: application/json" \
  -d '{"zone":"example.com","host":"@","record_type":"NS","value":"ns1.example.com.","ttl":300,"priority":0}'

# TXT 记录
curl -X POST http://你的服务器IP:8081/api/records/add \
  -H "X-API-Key: 你的API密钥" \
  -H "Content-Type: application/json" \
  -d '{"zone":"example.com","host":"verify","record_type":"TXT","value":"verification-token","ttl":300,"priority":0}'

# SOA 记录
curl -X POST http://你的服务器IP:8081/api/records/add \
  -H "X-API-Key: 你的API密钥" \
  -H "Content-Type: application/json" \
  -d '{"zone":"example.com","host":"@","record_type":"SOA","value":"ns1.example.com. admin.example.com. 2026053101 3600 600 604800 300","ttl":300,"priority":0}'

# SRV 记录
curl -X POST http://你的服务器IP:8081/api/records/add \
  -H "X-API-Key: 你的API密钥" \
  -H "Content-Type: application/json" \
  -d '{"zone":"example.com","host":"_sip._tcp","record_type":"SRV","value":"10 20 443 service.example.com.","ttl":300,"priority":0}'

# PTR 记录
curl -X POST http://你的服务器IP:8081/api/records/add \
  -H "X-API-Key: 你的API密钥" \
  -H "Content-Type: application/json" \
  -d '{"zone":"in-addr.arpa","host":"4.3.2.1","record_type":"PTR","value":"host.example.com.","ttl":300,"priority":0}'
```

删除一条 DNS 记录：

```bash
curl -X POST http://你的服务器IP:8081/api/records/delete \
  -H "X-API-Key: 你的API密钥" \
  -H "Content-Type: application/json" \
  -d '{"id": 1}'
```

## 常用运维命令

```bash
# 查看服务运行状态
systemctl status dgdns

# 查看实时运行日志
journalctl -u dgdns -f

# 重启服务
systemctl restart dgdns

# 停止服务
systemctl stop dgdns
```

## 注意事项

- 端口占用：请确保服务器的 `53`（DNS）、`8080`（Web）、`8081`（API）端口未被占用，并在防火墙中放行。
- 主从同步：使用主从同步功能时，请确保主库 MySQL 已开启 Binlog，且从库能够正常访问主库的 `3306` 端口。
- 安全性：生产环境中，建议将 `config.yaml` 中的数据库密码和 API 密钥修改为高强度随机字符串。

## 版权声明

Digitalgleam 版权所有，盗版必究。
### 如您遇到疑问 cue 我
QQ 3799599152
