package main

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

func InitDB(cfg *Config) error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		cfg.Database.User, cfg.Database.Password, cfg.Database.Host, cfg.Database.Port, cfg.Database.DBName)
	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("打开数据库连接失败: %v", err)
	}
	if err = db.Ping(); err != nil {
		return fmt.Errorf("数据库连接测试失败: %v", err)
	}

	if cfg.Role == "master" {
		if err := setupMaster(db, cfg); err != nil {
			return fmt.Errorf("配置主服务器失败: %v", err)
		}
		fmt.Println("✅ 主服务器 MySQL 权限与 NOTIFY 配置完成")
	} else if cfg.Role == "slave" {
		if err := setupSlave(db, cfg); err != nil {
			return fmt.Errorf("配置从服务器失败: %v", err)
		}
		fmt.Println("✅ 从服务器 MySQL 同步配置完成")
	}

	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(10)
	fmt.Println("✅ MySQL 数据库连接成功")
	return nil
}

func setupMaster(db *sql.DB, cfg *Config) error {
	if len(cfg.Replication.AllowedSlaves) > 0 {
		for _, ip := range cfg.Replication.AllowedSlaves {
			user := fmt.Sprintf("'%s'@'%s'", cfg.Replication.SlaveUser, ip)
			_, err := db.Exec(fmt.Sprintf("CREATE USER IF NOT EXISTS %s IDENTIFIED BY '%s'", user, cfg.Replication.SlavePassword))
			if err != nil && !strings.Contains(err.Error(), "_duplicate") {
				return err
			}
			_, err = db.Exec(fmt.Sprintf("GRANT REPLICATION SLAVE ON *.* TO %s", user))
			if err != nil {
				return err
			}
		}
		_, _ = db.Exec("FLUSH PRIVILEGES")
	}
	if len(cfg.Replication.NotifySlaves) > 0 {
		notifyIps := "'" + strings.Join(cfg.Replication.NotifySlaves, "', '") + "'"
		_, err := db.Exec(fmt.Sprintf("CHANGE REPLICATION SOURCE TO SOURCE_NOTIFY = (%s)", notifyIps))
		if err != nil {
			return fmt.Errorf("配置 NOTIFY 失败(请确认 MySQL 版本>=8.0.22): %v", err)
		}
	}
	return nil
}

func setupSlave(db *sql.DB, cfg *Config) error {
	_, _ = db.Exec("STOP SLAVE")
	_, err := db.Exec(fmt.Sprintf(
		"CHANGE MASTER TO MASTER_HOST='%s', MASTER_PORT=%d, MASTER_USER='%s', MASTER_PASSWORD='%s', MASTER_AUTO_POSITION=1",
		cfg.Replication.MasterHost, cfg.Replication.MasterPort, cfg.Replication.MasterUser, cfg.Replication.MasterPassword,
	))
	if err != nil {
		return err
	}
	_, err = db.Exec("START SLAVE")
	return err
}
