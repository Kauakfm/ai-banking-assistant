package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Porta              string
	ProfileAPIURL      string
	TransactionsAPIURL string
	AgentServiceURL    string
	CacheTTL           time.Duration
	RequestTimeout     time.Duration
	MaxConcorrencia    int
	RetryMaxTentativas int
	CBLimiteFalhas     int
	LogLevel           string
}

func Carregar() *Config {
	return &Config{
		Porta:              obterEnv("PORTA", "8080"),
		ProfileAPIURL:      obterEnv("PROFILE_API_URL", "http://localhost:8081"),
		TransactionsAPIURL: obterEnv("TRANSACTIONS_API_URL", "http://localhost:8081"),
		AgentServiceURL:    obterEnv("AGENT_SERVICE_URL", "http://localhost:8081"),
		CacheTTL:           obterEnvDuracao("CACHE_TTL", 5*time.Minute),
		RequestTimeout:     obterEnvDuracao("REQUEST_TIMEOUT", 30*time.Second),
		MaxConcorrencia:    obterEnvInt("MAX_CONCORRENCIA", 100),
		RetryMaxTentativas: obterEnvInt("RETRY_MAX_TENTATIVAS", 3),
		CBLimiteFalhas:     obterEnvInt("CIRCUIT_BREAKER_LIMITE", 5),
		LogLevel:           obterEnv("LOG_LEVEL", "info"),
	}
}

func obterEnv(chave, padrao string) string {
	if valor := os.Getenv(chave); valor != "" {
		return valor
	}
	return padrao
}

func obterEnvInt(chave string, padrao int) int {
	if valor := os.Getenv(chave); valor != "" {
		if v, err := strconv.Atoi(valor); err == nil {
			return v
		}
	}
	return padrao
}

func obterEnvDuracao(chave string, padrao time.Duration) time.Duration {
	if valor := os.Getenv(chave); valor != "" {
		if d, err := time.ParseDuration(valor); err == nil {
			return d
		}
	}
	return padrao
}
