#!/usr/bin/env bash
set -euo pipefail

# 专属 DNS 系统一键部署脚本

if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
  echo "❌ 请使用 root 权限运行此脚本（例如: sudo bash install.sh）"
  exit 1
fi

if [[ ! -f "go.mod" || ! -f "init.sql" || ! -f "main.go" || ! -f "config.yaml" ]]; then
  echo "❌ 错误：当前目录缺少必要的源文件。"
  echo "请在包含 config.yaml 的源码根目录下运行此脚本。"
  exit 1
fi

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
INSTALL_DIR="/opt/dgdns"
OS_NAME=$(uname -s)
OS_ID=""
OS_LIKE=""
MYSQL_SERVICE="mysql"
MYSQL_BIN="mysql"
GO_BIN="go"

if ! command -v "$GO_BIN" >/dev/null 2>&1; then
  echo "❌ 未检测到 Go，请先安装 Go 1.21+。"
  exit 1
fi

if [[ "$OS_NAME" == "Linux" && -f /etc/os-release ]]; then
  # shellcheck disable=SC1091
  . /etc/os-release
  OS_ID=${ID:-}
  OS_LIKE=${ID_LIKE:-}
elif [[ "$OS_NAME" != "FreeBSD" ]]; then
  echo "❌ 当前脚本仅支持 Debian/Ubuntu、RHEL/CentOS 及 FreeBSD。"
  exit 1
fi

install_mysql() {
  if command -v "$MYSQL_BIN" >/dev/null 2>&1; then
    echo "✅ 已检测到 MySQL 客户端。"
    return
  fi

  case "$OS_NAME:$OS_ID:$OS_LIKE" in
    Linux:*debian*|Linux:debian:*|Linux:ubuntu:*)
      echo "📦 正在安装 MySQL（Debian/Ubuntu）..."
      apt update
      DEBIAN_FRONTEND=noninteractive apt install -y mysql-server
      MYSQL_SERVICE="mysql"
      ;;
    Linux:*rhel*|Linux:*fedora*|Linux:rocky:*|Linux:almalinux:*|Linux:centos:*|Linux:*suse*)
      echo "📦 正在安装 MySQL（RHEL/CentOS 系）..."
      if command -v dnf >/dev/null 2>&1; then
        dnf install -y mysql-server
      else
        yum install -y mysql-server
      fi
      MYSQL_SERVICE="mysqld"
      ;;
    FreeBSD:*)
      echo "📦 正在安装 MySQL（FreeBSD）..."
      pkg install -y mysql84-server || pkg install -y mysql80-server || pkg install -y mysql-server
      MYSQL_SERVICE="mysql-server"
      ;;
    *)
      echo "❌ 无法识别系统发行版，请手动安装 MySQL 后重试。"
      exit 1
      ;;
  esac
}

start_mysql() {
  case "$OS_NAME" in
    Linux)
      systemctl enable "$MYSQL_SERVICE"
      systemctl start "$MYSQL_SERVICE"
      ;;
    FreeBSD)
      sysrc mysql_enable="YES" >/dev/null
      service "$MYSQL_SERVICE" start
      ;;
  esac
}

update_config_password() {
  local new_password="$1"
  local safe_password
  safe_password=${new_password//\/\\}
  safe_password=${safe_password//&/\\&}
  safe_password=${safe_password//\//\\/}
  if [[ "$OS_NAME" == "FreeBSD" ]]; then
    sed -i '' "s/^  password:.*/  password: ${safe_password}/" "$SCRIPT_DIR/config.yaml"
  else
    sed -i "s/^  password:.*/  password: ${safe_password}/" "$SCRIPT_DIR/config.yaml"
  fi
}

install_mysql
start_mysql

DB_PASSWORD=$(awk -F': *' '/^[[:space:]]*password:[[:space:]]*/ {print $2; exit}' "$SCRIPT_DIR/config.yaml" | tr -d '[:space:]')
if [[ -z "${DB_PASSWORD}" || "${DB_PASSWORD}" == "your_db_password" ]]; then
  echo "⚠️ 检测到 config.yaml 中数据库密码为空或默认值。"
  while true; do
    read -rsp "请输入新的 MySQL root 密码（至少 8 位）: " NEW_PASSWORD
    echo
    read -rsp "请再次输入确认: " CONFIRM_PASSWORD
    echo

    if [[ "$NEW_PASSWORD" == "$CONFIRM_PASSWORD" && ${#NEW_PASSWORD} -ge 8 ]]; then
      DB_PASSWORD="$NEW_PASSWORD"
      update_config_password "$DB_PASSWORD"
      echo "✅ 密码已更新到 config.yaml"
      break
    fi

    if [[ ${#NEW_PASSWORD} -lt 8 ]]; then
      echo "❌ 密码长度不能少于 8 位。"
    else
      echo "❌ 两次输入不一致。"
    fi
  done
fi

if "$MYSQL_BIN" -u root -p"$DB_PASSWORD" -e "SELECT 1" >/dev/null 2>&1; then
  "$MYSQL_BIN" -u root -p"$DB_PASSWORD" < "$SCRIPT_DIR/init.sql"
else
  if "$MYSQL_BIN" -u root -e "SELECT 1" >/dev/null 2>&1; then
    "$MYSQL_BIN" -u root -e "ALTER USER 'root'@'localhost' IDENTIFIED BY '${DB_PASSWORD}'; FLUSH PRIVILEGES;"
    "$MYSQL_BIN" -u root -p"$DB_PASSWORD" < "$SCRIPT_DIR/init.sql"
  else
    echo "❌ 无法连接到 MySQL，请确认 root 密码是否正确。"
    exit 1
  fi
fi

echo "🔨 构建 Go 程序..."
mkdir -p "$INSTALL_DIR"
cp "$SCRIPT_DIR/config.yaml" "$INSTALL_DIR/config.yaml"
cp "$SCRIPT_DIR/init.sql" "$INSTALL_DIR/init.sql"
(cd "$SCRIPT_DIR" && "$GO_BIN" mod tidy && "$GO_BIN" build -o "$INSTALL_DIR/dgdns")

if [[ "$OS_NAME" == "FreeBSD" ]]; then
  echo "⚙️ 配置 FreeBSD rc.d 服务..."
  cat > /usr/local/etc/rc.d/dgdns <<EOF
#!/bin/sh
# PROVIDE: dgdns
# REQUIRE: NETWORKING mysql
# KEYWORD: shutdown

. /etc/rc.subr

name="dgdns"
rcvar="dgdns_enable"
command="${INSTALL_DIR}/dgdns"
pidfile="/var/run/${name}.pid"
command_args=""

load_rc_config $name
: \\${dgdns_enable:=NO}

start_precmd="dgdns_prestart"
dgdns_prestart()
{
    install -d -o root -g wheel -m 0755 /var/run
}

run_rc_command "$1"
EOF
  chmod +x /usr/local/etc/rc.d/dgdns
  sysrc dgdns_enable="YES" >/dev/null
  service dgdns restart || service dgdns start
else
  echo "⚙️ 配置 systemd 服务..."
  cat > /etc/systemd/system/dgdns.service <<EOF
[Unit]
Description=My Custom DNS Server
After=network.target $MYSQL_SERVICE.service

[Service]
Type=simple
User=root
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/dgdns
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable dgdns
  systemctl restart dgdns
fi

echo "✅ 部署完成"
echo "Web 管理界面: http://你的服务器IP:8080"
echo "API 服务: http://你的服务器IP:8081"



