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
	return &Config{
		DSN:      getEnv("APP_DSN", "host=localhost user=postgres password=postgres dbname=pickups sslmode=disable"),
		HTTPPort: getEnv("APP_PORT", "9000"),
		Username: getEnv("APP_USER", "admin"),
		Password: getEnv("APP_PASS", "secret"),
	}
}

func getEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}

func (c *Config) Addr() string {
	return fmt.Sprintf(":%s", c.HTTPPort)
}
