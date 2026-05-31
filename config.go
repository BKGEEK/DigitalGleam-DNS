package main

import (
	"fmt"
	"os"
	"path/filepath"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Role          string `yaml:"role"`
	Database      struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		DBName   string `yaml:"dbname"`
	} `yaml:"database"`
	Replication struct {
		SlaveUser      string   `yaml:"slave_user"`
		SlavePassword  string   `yaml:"slave_password"`
		AllowedSlaves  []string `yaml:"allowed_slaves"`
		NotifySlaves   []string `yaml:"notify_slaves"`
		MasterHost     string `yaml:"master_host"`
		MasterPort     int    `yaml:"master_port"`
		MasterUser     string `yaml:"master_user"`
		MasterPassword string `yaml:"master_password"`
	} `yaml:"replication"`
	DNS struct {
		Port int    `yaml:"port"`
		TTL  uint32 `yaml:"ttl"`
	} `yaml:"dns"`
}

func LoadConfig() (*Config, error) {
	exePath, err := os.Executable()
	if err != nil { return nil, fmt.Errorf("获取程序路径失败: %v", err) }
	configPath := filepath.Join(filepath.Dir(exePath), "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil { return nil, fmt.Errorf("读取配置文件失败: %v", err) }
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil { return nil, fmt.Errorf("解析配置文件失败: %v", err) }
	return &cfg, nil
}
