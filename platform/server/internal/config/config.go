package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr string

	ServiceCatalogPath string
	DatabaseURL        string

	RabbitMQURL         string
	RabbitMQExchange    string
	RabbitMQQueue       string
	RabbitMQDLQ         string
	RabbitMQRoutingKey  string
	RabbitMQConsumerTag string

	JenkinsBaseURL  string
	JenkinsUsername string
	JenkinsToken    string

	Kubeconfig string

	JenkinsPollInterval time.Duration
	JenkinsTimeout      time.Duration
	ArgoPollInterval    time.Duration
	ArgoTimeout         time.Duration
	RolloutPollInterval time.Duration
	RolloutTimeout      time.Duration
	ReleaseLockTTL      time.Duration
}

func Load() Config {
	return Config{
		HTTPAddr: getEnv("HTTP_ADDR", ":8080"),

		ServiceCatalogPath: getEnv("SERVICE_CATALOG_PATH", "configs/service-catalog.yaml"),
		DatabaseURL:        getEnv("DATABASE_URL", ""),

		RabbitMQURL:         getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		RabbitMQExchange:    getEnv("RABBITMQ_EXCHANGE", "platform.release.exchange"),
		RabbitMQQueue:       getEnv("RABBITMQ_QUEUE", "platform.release.requested.queue"),
		RabbitMQDLQ:         getEnv("RABBITMQ_DLQ", "platform.release.dlq"),
		RabbitMQRoutingKey:  getEnv("RABBITMQ_ROUTING_KEY", "release.requested"),
		RabbitMQConsumerTag: getEnv("RABBITMQ_CONSUMER_TAG", "platform-release-worker"),

		JenkinsBaseURL:  getEnv("JENKINS_BASE_URL", ""),
		JenkinsUsername: getEnv("JENKINS_USERNAME", ""),
		JenkinsToken:    getEnv("JENKINS_TOKEN", ""),

		Kubeconfig: getEnv("KUBECONFIG", ""),

		JenkinsPollInterval: getDurationEnv("JENKINS_POLL_INTERVAL", 10*time.Second),
		JenkinsTimeout:      getDurationEnv("JENKINS_TIMEOUT", 45*time.Minute),
		ArgoPollInterval:    getDurationEnv("ARGO_POLL_INTERVAL", 10*time.Second),
		ArgoTimeout:         getDurationEnv("ARGO_TIMEOUT", 10*time.Minute),
		RolloutPollInterval: getDurationEnv("ROLLOUT_POLL_INTERVAL", 5*time.Second),
		RolloutTimeout:      getDurationEnv("ROLLOUT_TIMEOUT", 10*time.Minute),
		ReleaseLockTTL:      getDurationEnv("RELEASE_LOCK_TTL", 4*time.Hour),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	if duration, err := time.ParseDuration(value); err == nil {
		return duration
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(seconds) * time.Second
	}
	return fallback
}
