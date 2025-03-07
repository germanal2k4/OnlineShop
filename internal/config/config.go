package config

import (
	"fmt"
	"os"
)

type Config struct {
	DSN      string
	HTTPPort string
	Username string
	Password string
}

func LoadConfig() *Config {
	dsn := os.Getenv("APP_DSN")
	if dsn == "" {
		dsn = "host=localhost user=postgres password=postgres dbname=pickups sslmode=disable"
	}
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "9000"
	}
	user := os.Getenv("APP_USER")
	if user == "" {
		user = "admin"
	}
	pass := os.Getenv("APP_PASS")
	if pass == "" {
		pass = "secret"
	}
	return &Config{
		DSN:      dsn,
		HTTPPort: port,
		Username: user,
		Password: pass,
	}
}

func (c *Config) Addr() string {
	return fmt.Sprintf(":%s", c.HTTPPort)
}
