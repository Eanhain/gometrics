package clientconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gometrics/internal/addr"
)

// createTempConfigFile создаёт временный JSON файл конфигурации для тестов
func createTempConfigFile(t *testing.T, cfg JSONConfig) string {
	t.Helper()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	return configPath
}

// TestClientConfig_ParseFlagsFromArgs проверяет парсинг флагов и приоритет переменных окружения.
// Использует табличные тесты для покрытия различных сценариев конфигурации.
func TestClientConfig_ParseFlagsFromArgs(t *testing.T) {
	// setEnv - вспомогательная функция для установки переменных окружения на время теста.
	// Возвращает функцию очистки, которую нужно вызвать через defer.
	setEnv := func(kv map[string]string) func() {
		original := make(map[string]string)
		for k, v := range kv {
			original[k] = os.Getenv(k)
			os.Setenv(k, v)
		}
		return func() {
			for k, v := range original {
				if v == "" {
					os.Unsetenv(k)
				} else {
					os.Setenv(k, v)
				}
			}
		}
	}

	tests := []struct {
		name       string
		args       []string          // Аргументы командной строки (флаги)
		envVars    map[string]string // Переменные окружения
		jsonConfig *JSONConfig       // Конфигурация из JSON файла (nil если не используется)
		want       ClientConfig      // Ожидаемая итоговая конфигурация
	}{
		{
			name:       "Default values",
			args:       []string{},
			envVars:    map[string]string{},
			jsonConfig: nil,
			want: ClientConfig{
				ReportInterval: 10,
				PollInterval:   2,
				Compress:       "gzip",
				RateLimit:      5,
				Key:            "",
				Addr:           addr.Addr{Host: "localhost", Port: 8080},
			},
		},
		{
			name: "Flags override defaults",
			args: []string{
				"-r", "20",
				"-p", "5",
				"-a", "192.168.0.1:9000",
				"-k", "secret",
				"-l", "10",
			},
			envVars:    map[string]string{},
			jsonConfig: nil,
			want: ClientConfig{
				ReportInterval: 20,
				PollInterval:   5,
				Compress:       "gzip",
				Key:            "secret",
				RateLimit:      10,
				Addr:           addr.Addr{Host: "192.168.0.1", Port: 9000},
			},
		},
		{
			name: "Environment variables override defaults",
			args: []string{},
			envVars: map[string]string{
				"REPORT_INTERVAL": "30",
				"ADDRESS":         "env-host:7070",
				"KEY":             "env-secret",
			},
			jsonConfig: nil,
			want: ClientConfig{
				ReportInterval: 30,
				PollInterval:   2,
				Compress:       "gzip",
				RateLimit:      5,
				Key:            "env-secret",
				Addr:           addr.Addr{Host: "env-host", Port: 7070},
			},
		},
		{
			name: "Env KEY priority over Flag k (Special Logic)",
			args: []string{"-k", "flag-secret"},
			envVars: map[string]string{
				"KEY": "env-priority-secret",
			},
			jsonConfig: nil,
			want: ClientConfig{
				ReportInterval: 10,
				PollInterval:   2,
				Compress:       "gzip",
				RateLimit:      5,
				Key:            "env-priority-secret", // ENV победил флаг
				Addr:           addr.Addr{Host: "localhost", Port: 8080},
			},
		},
		{
			name: "Mixed flags and env",
			args: []string{"-r", "50"},
			envVars: map[string]string{
				"POLL_INTERVAL": "8",
			},
			jsonConfig: nil,
			want: ClientConfig{
				ReportInterval: 50, // Flag
				PollInterval:   8,  // Env
				Compress:       "gzip",
				RateLimit:      5,
				Key:            "",
				Addr:           addr.Addr{Host: "localhost", Port: 8080},
			},
		},
		// ========== НОВЫЕ ТЕСТЫ ДЛЯ JSON КОНФИГУРАЦИИ ==========
		{
			name:    "JSON config only (no flags, no env)",
			args:    []string{},
			envVars: map[string]string{},
			jsonConfig: &JSONConfig{
				Address:        "json-host:3000",
				ReportInterval: "30s",
				PollInterval:   "5s",
				CryptoKey:      "/json/crypto.pem",
			},
			want: ClientConfig{
				ReportInterval: 30,
				PollInterval:   5,
				Compress:       "gzip",
				RateLimit:      5,
				Key:            "",
				CryptoKey:      "/json/crypto.pem",
				Addr:           addr.Addr{Host: "json-host", Port: 3000},
			},
		},
		{
			name:    "JSON config with minutes interval",
			args:    []string{},
			envVars: map[string]string{},
			jsonConfig: &JSONConfig{
				ReportInterval: "1m",
				PollInterval:   "30s",
			},
			want: ClientConfig{
				ReportInterval: 60,
				PollInterval:   30,
				Compress:       "gzip",
				RateLimit:      5,
				Addr:           addr.Addr{Host: "localhost", Port: 8080},
			},
		},
		{
			name: "Priority: Flags > JSON config",
			args: []string{
				"-r", "100",
				"-p", "10",
			},
			envVars: map[string]string{},
			jsonConfig: &JSONConfig{
				ReportInterval: "5s",
				PollInterval:   "1s",
				CryptoKey:      "/json/key.pem",
			},
			want: ClientConfig{
				ReportInterval: 100,
				PollInterval:   10,
				Compress:       "gzip",
				RateLimit:      5,
				CryptoKey:      "/json/key.pem",
				Addr:           addr.Addr{Host: "localhost", Port: 8080},
			},
		},
		{
			name: "Priority: Env > JSON config",
			args: []string{},
			envVars: map[string]string{
				"REPORT_INTERVAL": "200",
				"ADDRESS":         "env-host:5000",
			},
			jsonConfig: &JSONConfig{
				Address:        "json-host:3000",
				ReportInterval: "10s",
				PollInterval:   "3s",
				CryptoKey:      "/json/crypto.pem",
			},
			want: ClientConfig{
				ReportInterval: 200,
				PollInterval:   3,
				Compress:       "gzip",
				RateLimit:      5,
				CryptoKey:      "/json/crypto.pem",
				Addr:           addr.Addr{Host: "env-host", Port: 5000},
			},
		},
		{
			name: "Priority: Flags > Env > JSON (all three sources)",
			args: []string{"-r", "999"},
			envVars: map[string]string{
				"POLL_INTERVAL": "77",
				"CRYPTO_KEY":    "/env/crypto.pem",
			},
			jsonConfig: &JSONConfig{
				Address:        "json-host:1111",
				ReportInterval: "1s",
				PollInterval:   "1s",
				CryptoKey:      "/json/crypto.pem",
			},
			want: ClientConfig{
				ReportInterval: 999,
				PollInterval:   77,
				Compress:       "gzip",
				RateLimit:      5,
				CryptoKey:      "/env/crypto.pem",
				Addr:           addr.Addr{Host: "json-host", Port: 1111},
			},
		},
		{
			name:    "JSON config via -config flag",
			args:    []string{},
			envVars: map[string]string{},
			jsonConfig: &JSONConfig{
				ReportInterval: "45s",
				PollInterval:   "15s",
			},
			want: ClientConfig{
				ReportInterval: 45,
				PollInterval:   15,
				Compress:       "gzip",
				RateLimit:      5,
				Addr:           addr.Addr{Host: "localhost", Port: 8080},
			},
		},
		{
			name:    "JSON config via CONFIG env var",
			args:    []string{},
			envVars: map[string]string{},
			jsonConfig: &JSONConfig{
				ReportInterval: "120s",
				Address:        "config-host:9999",
			},
			want: ClientConfig{
				ReportInterval: 120,
				PollInterval:   2,
				Compress:       "gzip",
				RateLimit:      5,
				Addr:           addr.Addr{Host: "config-host", Port: 9999},
			},
		},
		{
			name:    "JSON config with complex duration",
			args:    []string{},
			envVars: map[string]string{},
			jsonConfig: &JSONConfig{
				ReportInterval: "1m30s",
				PollInterval:   "500ms",
			},
			want: ClientConfig{
				ReportInterval: 90,
				PollInterval:   0,
				Compress:       "gzip",
				RateLimit:      5,
				Addr:           addr.Addr{Host: "localhost", Port: 8080},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. Настройка окружения
			cleanup := setEnv(tt.envVars)
			defer cleanup()

			// 2. Подготовка аргументов командной строки
			args := tt.args

			// 3. Создание временного JSON файла конфигурации (если нужен)
			if tt.jsonConfig != nil {
				configPath := createTempConfigFile(t, *tt.jsonConfig)

				// Специальная обработка для теста приоритета CONFIG env
				if tt.name == "JSON config via CONFIG env var" {
					os.Setenv("CONFIG", configPath)
					defer os.Unsetenv("CONFIG")
				} else {
					args = append(args, "-config", configPath)
				}
			}

			// 4. Инициализация и парсинг
			cfg := InitialFlags()
			err := cfg.ParseFlagsFromArgs(args)
			if err != nil {
				t.Fatalf("ParseFlagsFromArgs failed: %v", err)
			}

			// 5. Полевая проверка результатов
			if cfg.ReportInterval != tt.want.ReportInterval {
				t.Errorf("ReportInterval = %d, want %d", cfg.ReportInterval, tt.want.ReportInterval)
			}
			if cfg.PollInterval != tt.want.PollInterval {
				t.Errorf("PollInterval = %d, want %d", cfg.PollInterval, tt.want.PollInterval)
			}
			if cfg.RateLimit != tt.want.RateLimit {
				t.Errorf("RateLimit = %d, want %d", cfg.RateLimit, tt.want.RateLimit)
			}
			if cfg.Compress != tt.want.Compress {
				t.Errorf("Compress = %s, want %s", cfg.Compress, tt.want.Compress)
			}
			if cfg.Key != tt.want.Key {
				t.Errorf("Key = %s, want %s", cfg.Key, tt.want.Key)
			}
			if cfg.CryptoKey != tt.want.CryptoKey {
				t.Errorf("CryptoKey = %s, want %s", cfg.CryptoKey, tt.want.CryptoKey)
			}

			// Сравнение структуры Addr
			if cfg.Addr.Host != tt.want.Addr.Host || cfg.Addr.Port != tt.want.Addr.Port {
				t.Errorf("Addr = %v, want %v", cfg.Addr, tt.want.Addr)
			}
		})
	}
}

// TestParseInterval проверяет парсинг строк интервала
func TestParseInterval(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		hasError bool
	}{
		{"1s", 1, false},
		{"60s", 60, false},
		{"5m", 300, false},
		{"1h", 3600, false},
		{"1m30s", 90, false},
		{"500ms", 0, false},
		{"", 0, false},
		{"invalid", 0, true},
		{"100", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseInterval(tt.input)
			if tt.hasError {
				if err == nil {
					t.Errorf("expected error for input %q, got nil", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for input %q: %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("parseInterval(%q) = %d, want %d", tt.input, result, tt.expected)
				}
			}
		})
	}
}

// TestLoadJSONConfig проверяет загрузку JSON конфигурации
func TestLoadJSONConfig(t *testing.T) {
	t.Run("Empty path returns nil", func(t *testing.T) {
		cfg, err := loadJSONConfig("")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if cfg != nil {
			t.Errorf("expected nil config, got %v", cfg)
		}
	})

	t.Run("Non-existent file returns error", func(t *testing.T) {
		cfg, err := loadJSONConfig("/non/existent/path.json")
		if err == nil {
			t.Error("expected error for non-existent file")
		}
		if cfg != nil {
			t.Errorf("expected nil config, got %v", cfg)
		}
	})

	t.Run("Invalid JSON returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "invalid.json")
		os.WriteFile(configPath, []byte("{invalid json}"), 0644)

		cfg, err := loadJSONConfig(configPath)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
		if cfg != nil {
			t.Errorf("expected nil config, got %v", cfg)
		}
	})

	t.Run("Valid JSON is parsed correctly", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "valid.json")

		jsonData := `{
			"address": "127.0.0.1:9090",
			"report_interval": "30s",
			"poll_interval": "5s",
			"crypto_key": "/path/to/key.pem"
		}`
		os.WriteFile(configPath, []byte(jsonData), 0644)

		cfg, err := loadJSONConfig(configPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg == nil {
			t.Fatal("expected non-nil config")
		}
		if cfg.Address != "127.0.0.1:9090" {
			t.Errorf("Address = %s, want 127.0.0.1:9090", cfg.Address)
		}
		if cfg.ReportInterval != "30s" {
			t.Errorf("ReportInterval = %s, want 30s", cfg.ReportInterval)
		}
		if cfg.PollInterval != "5s" {
			t.Errorf("PollInterval = %s, want 5s", cfg.PollInterval)
		}
		if cfg.CryptoKey != "/path/to/key.pem" {
			t.Errorf("CryptoKey = %s, want /path/to/key.pem", cfg.CryptoKey)
		}
	})
}

// TestClientConfig_Getters проверяет методы-геттеры для хоста и порта.
func TestClientConfig_Getters(t *testing.T) {
	cfg := ClientConfig{}
	_ = cfg.Addr.Set("10.0.0.1:5432")

	if got := cfg.GetHost(); got != "10.0.0.1" {
		t.Errorf("GetHost() = %v, want %v", got, "10.0.0.1")
	}

	if got := cfg.GetPort(); got != ":5432" {
		t.Errorf("GetPort() = %v, want %v", got, ":5432")
	}
}

// ExampleClientConfig_GetPort демонстрирует использование метода GetPort.
func ExampleClientConfig_GetPort() {
	cfg := InitialFlags()
	_ = cfg.Addr.Set("example.com:80")

	fmt.Println(cfg.GetPort())

	// Output:
	// :80
}

// ExampleClientConfig_GetHost демонстрирует использование метода GetHost.
func ExampleClientConfig_GetHost() {
	cfg := InitialFlags()
	_ = cfg.Addr.Set("db.internal:5432")

	fmt.Println(cfg.GetHost())

	// Output:
	// db.internal
}

// ExampleClientConfig_ParseFlagsFromArgs_withJSON демонстрирует использование JSON конфигурации
func ExampleClientConfig_ParseFlagsFromArgs_withJSON() {
	tmpFile, _ := os.CreateTemp("", "config-*.json")
	defer os.Remove(tmpFile.Name())

	jsonConfig := `{"report_interval": "60s", "poll_interval": "10s", "address": "metrics.local:8080"}`
	tmpFile.WriteString(jsonConfig)
	tmpFile.Close()

	cfg := InitialFlags()
	_ = cfg.ParseFlagsFromArgs([]string{"-config", tmpFile.Name()})

	fmt.Printf("ReportInterval: %d\n", cfg.ReportInterval)
	fmt.Printf("PollInterval: %d\n", cfg.PollInterval)
	fmt.Printf("Address: %s\n", cfg.Addr.GetAddr())

	// Output:
	// ReportInterval: 60
	// PollInterval: 10
	// Address: metrics.local:8080
}
