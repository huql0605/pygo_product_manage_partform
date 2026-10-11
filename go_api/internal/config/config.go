package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Python   PythonConfig   `mapstructure:"python"`
}

type ServerConfig struct {
	Port int `mapstructure:"port"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

type PythonConfig struct {
	BaseURL string        `mapstructure:"base_url"`
	Timeout time.Duration `mapstructure:"timeout"`
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	applyEnvOverrides(&cfg)
	return &cfg, nil
}

func (d DatabaseConfig) DSN() string {
	parts := []string{
		fmt.Sprintf("host=%s", d.Host),
		fmt.Sprintf("user=%s", d.User),
		fmt.Sprintf("dbname=%s", d.DBName),
		fmt.Sprintf("port=%d", d.Port),
		fmt.Sprintf("sslmode=%s", d.SSLMode),
	}
	// 空密码就不要写进 DSN，避免 "password= dbname=..." 这种歧义
	if d.Password != "" {
		parts = append(parts, fmt.Sprintf("password=%s", d.Password))
	}
	return strings.Join(parts, " ")
}

func applyEnvOverrides(c *Config) {
	if v := os.Getenv("DB_HOST"); v != "" {
		c.Database.Host = v
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.Database.Port = p
		}
	}
	if v := os.Getenv("DB_USER"); v != "" {
		c.Database.User = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		c.Database.Password = v
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		c.Database.DBName = v
	}
	if v := os.Getenv("PYTHON_BASE_URL"); v != "" {
		c.Python.BaseURL = v
	}
}
