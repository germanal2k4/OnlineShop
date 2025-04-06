package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	DSN          string
	HTTPPort     string
	Username     string
	Password     string
	FilterWord   string
	KafkaBrokers []string
	KafkaGroupID string
	KafkaTopic   string
}

func LoadConfig() *Config {
	brokersStr := getEnv("KAFKA_BROKERS", "localhost:9092")
	return &Config{
		DSN:          getEnv("APP_DSN", "host=localhost user=postgres password=postgres dbname=pickups sslmode=disable"),
		HTTPPort:     getEnv("APP_PORT", "9000"),
		Username:     getEnv("APP_USER", "admin"),
		Password:     getEnv("APP_PASS", "secret"),
		FilterWord:   getEnv("APP_FILTER", ""),
		KafkaBrokers: strings.Split(brokersStr, ","),
		KafkaGroupID: getEnv("KAFKA_GROUP_ID", "audit-group"),
		KafkaTopic:   getEnv("KAFKA_TOPIC", "audit-tasks"),
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
