package config

import "os"

type Config struct {
	ServerPort  string
	DatabaseURL string
	RabbitMQURL string
}

func Load() *Config {
	return &Config{
		ServerPort:  getEnv("SERVER_PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://loan_user:loan_pass@localhost:5432/loan_engine?sslmode=disable"),
		RabbitMQURL: getEnv("RABBITMQ_URL", "amqp://rabbit_user:rabbit_pass@localhost:5672/"),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
