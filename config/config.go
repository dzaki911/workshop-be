package config

import (
	"bufio"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppEnv        string
	AppPort       string
	DevAdminEmail string // admin uji untuk sesi 2, digantikan auth di sesi 3
	DB            DB
	JWT           JWT
}

type DB struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

type JWT struct {
	Secret       string
	ExpiresHours int
}

// URL mengembalikan connection string PostgreSQL (password di-escape).
func (d DB) URL() string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(d.User, d.Password),
		Host:     d.Host + ":" + d.Port,
		Path:     d.Name,
		RawQuery: "sslmode=" + d.SSLMode,
	}
	return u.String()
}

// Load membaca file .env (jika ada, tidak menimpa env yang sudah di-set) lalu env var.
func Load() Config {
	loadDotEnv(".env")
	return Config{
		AppEnv:        getEnv("APP_ENV", "development"),
		AppPort:       getEnv("APP_PORT", "8080"),
		DevAdminEmail: getEnv("DEV_ADMIN_EMAIL", "admin@raport.local"),
		DB: DB{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", ""),
			Name:     getEnv("DB_NAME", "raport_evaluasi"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		JWT: JWT{
			Secret:       getEnv("JWT_SECRET", ""),
			ExpiresHours: getEnvInt("JWT_EXPIRES_HOURS", 24),
		},
	}
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if _, set := os.LookupEnv(k); !set {
			os.Setenv(k, v)
		}
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
