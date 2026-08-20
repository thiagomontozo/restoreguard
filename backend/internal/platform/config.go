package platform

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment, HTTPAddr, DatabaseURL, MigrationPath, SessionSecret, MasterKey         string
	ObjectStorageType, LocalStoragePath, S3Endpoint, S3Bucket, S3AccessKey, S3SecretKey string
	AllowedPostgresImage, CORSOrigin                                                    string
	S3UseTLS, CookieSecure                                                              bool
	MaxConcurrentDrills                                                                 int
	MaxArtifactBytes                                                                    int64
	DefaultDrillTimeout                                                                 time.Duration
	BootstrapEmail, BootstrapPassword                                                   string
}

func LoadConfig() (Config, error) {
	cfg := Config{
		Environment: env("RESTOREGUARD_ENV", "development"), HTTPAddr: env("RESTOREGUARD_HTTP_ADDR", ":8080"),
		DatabaseURL: os.Getenv("RESTOREGUARD_DATABASE_URL"), MigrationPath: env("RESTOREGUARD_MIGRATION_PATH", "../migrations"),
		SessionSecret: os.Getenv("RESTOREGUARD_SESSION_SECRET"), MasterKey: os.Getenv("RESTOREGUARD_MASTER_KEY"),
		ObjectStorageType: env("RESTOREGUARD_OBJECT_STORAGE_TYPE", "local"), LocalStoragePath: env("RESTOREGUARD_LOCAL_STORAGE_PATH", "../data/artifacts"),
		S3Endpoint: os.Getenv("RESTOREGUARD_S3_ENDPOINT"), S3Bucket: env("RESTOREGUARD_S3_BUCKET", "restoreguard"), S3AccessKey: os.Getenv("RESTOREGUARD_S3_ACCESS_KEY"), S3SecretKey: os.Getenv("RESTOREGUARD_S3_SECRET_KEY"),
		S3UseTLS: boolEnv("RESTOREGUARD_S3_USE_TLS", false), CookieSecure: boolEnv("RESTOREGUARD_COOKIE_SECURE", false),
		MaxConcurrentDrills: intEnv("RESTOREGUARD_MAX_CONCURRENT_DRILLS", 2), MaxArtifactBytes: int64Env("RESTOREGUARD_MAX_ARTIFACT_BYTES", 1<<30),
		AllowedPostgresImage: env("RESTOREGUARD_ALLOWED_POSTGRES_IMAGE", "postgres:17.6-alpine"), CORSOrigin: env("RESTOREGUARD_CORS_ORIGIN", "http://localhost:5173"),
		BootstrapEmail: strings.ToLower(strings.TrimSpace(os.Getenv("RESTOREGUARD_BOOTSTRAP_ADMIN_EMAIL"))), BootstrapPassword: os.Getenv("RESTOREGUARD_BOOTSTRAP_ADMIN_PASSWORD"),
	}
	duration, err := time.ParseDuration(env("RESTOREGUARD_DEFAULT_DRILL_TIMEOUT", "30m"))
	if err != nil {
		return cfg, err
	}
	cfg.DefaultDrillTimeout = duration
	if cfg.DatabaseURL == "" {
		return cfg, errors.New("RESTOREGUARD_DATABASE_URL is required")
	}
	if len(cfg.SessionSecret) < 32 {
		return cfg, errors.New("RESTOREGUARD_SESSION_SECRET must contain at least 32 characters")
	}
	if cfg.MaxConcurrentDrills < 1 || cfg.MaxConcurrentDrills > 20 {
		return cfg, errors.New("MAX_CONCURRENT_DRILLS must be between 1 and 20")
	}
	return cfg, nil
}
func env(k, d string) string {
	if value := os.Getenv(k); value != "" {
		return value
	}
	return d
}
func boolEnv(k string, d bool) bool {
	value := os.Getenv(k)
	if value == "" {
		return d
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return d
	}
	return parsed
}
func intEnv(k string, d int) int {
	value, err := strconv.Atoi(os.Getenv(k))
	if err != nil {
		return d
	}
	return value
}
func int64Env(k string, d int64) int64 {
	value, err := strconv.ParseInt(os.Getenv(k), 10, 64)
	if err != nil {
		return d
	}
	return value
}
