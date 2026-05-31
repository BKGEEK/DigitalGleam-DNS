# 数星DNS 管理系统

这是一个基于 Go 语言开发的轻量级、高可用专属 DNS 系统。它提供完整的 Web 管理界面、安全的 RESTful API 接口，并支持 MySQL 主从同步与 NOTIFY 主动推送机制，适用于企业内网 DNS 解析、域名验证与自动化运维场景。

## 核心功能

- 安全可靠：Web 界面强制管理员登录（bcrypt 加密），API 接口采用 X-API-Key 密钥认证（SHA-256 哈希存储）。
- 主从同步：支持 MySQL 主从复制，主库变更后可通过 NOTIFY 机制毫秒级推送到从库。
- Web 管理：基于 Bootstrap 的现代化管理后台，支持 DNS 记录增删改查及 API 密钥生命周期管理。
- RESTful API：提供完整的 API 接口，方便对接 CI/CD 流程、自动化脚本或第三方平台（如 Let's Encrypt DNS-01 验证）。
- 高性能：基于 `miekg/dns` 库开发，响应 UDP 53 端口请求，轻量高效。

## 项目目录结构

```text
mydns/
├── go.mod         # Go 依赖配置文件
├── main.go        # 程序入口
├── config.go      # 配置文件读取逻辑
├── db.go          # 数据库连接与主从同步配置
├── dns_server.go  # DNS 核心解析服务
├── api.go         # 带密钥认证的 RESTful API
├── web.go         # 带登录认证的 Web 管理界面
├── config.yaml    # 运行配置文件
├── init.sql       # 数据库初始化脚本
└── install.sh     # 一键自动化部署脚本
```

## 快速部署指南

### 1. 环境要求

- 操作系统：Linux（推荐 Ubuntu 20.04+ / Debian 11+ / CentOS 7+）
- 数据库：MySQL 8.0+（主从同步功能依赖较新版本特性）
- 运行环境：Go 1.21+

### 2. 一键安装

在项目根目录下直接执行自动化部署脚本：

```bash
chmod +x install.sh
sudo bash install.sh
```

脚本会自动安装依赖、编译程序、配置 Systemd 服务并启动 DNS 与 Web 服务。

### 3. 配置文件说明（`config.yaml`）

启动前，请根据实际环境修改 `config.yaml`：

```yaml
role: master # 当前节点角色：master（主）或 slave（从）

database:
  host: 127.0.0.1
  port: 3306
  user: root
  password: your_db_password # 修改为你的数据库密码
  dbname: dns_db

replication:
  # 主库配置：允许哪些从库 IP 拉取数据并接收 NOTIFY 通知
  allowed_slaves:
    - 192.168.1.101
  notify_slaves:
    - 192.168.1.101
  # 从库配置：如果当前角色是 slave，请填写主库 IP 和账号信息
  master_host: 192.168.1.100
  master_port: 3306
  master_user: repl_user
  master_password: repl_password

dns:
  port: 53   # DNS 解析监听端口
  ttl: 300   # 默认 TTL 时间
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

添加一条 DNS 记录：

```bash
curl -X POST http://你的服务器IP:8081/api/records/add \
  -H "X-API-Key: 你的API密钥" \
  -H "Content-Type: application/json" \
  -d '{
    "zone": "example.com",
    "host": "www",
    "record_type": "TXT",
    "value": "dns-verification-token-12345"
  }'
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
systemctl status mydns

# 查看实时运行日志
journalctl -u mydns -f

# 重启服务
systemctl restart mydns

# 停止服务
systemctl stop mydns
```

## 注意事项

- 端口占用：请确保服务器的 `53`（DNS）、`8080`（Web）、`8081`（API）端口未被占用，并在防火墙中放行。
- 主从同步：使用主从同步功能时，请确保主库 MySQL 已开启 Binlog，且从库能够正常访问主库的 `3306` 端口。
- 安全性：生产环境中，建议将 `config.yaml` 中的数据库密码和 API 密钥修改为高强度随机字符串。

## 版权声明

Digitalgleam 版权所有，盗版必究。
如您遇到疑问cue我[QQ](https://wpa.qq.com/msgrd?v=3&uin=3799599152&site=qq&menu=yes) 。
