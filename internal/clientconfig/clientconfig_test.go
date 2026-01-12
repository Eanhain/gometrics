package clientconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"gometrics/internal/addr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTempConfig создает временный JSON-файл для тестов.
func createTempConfig(t *testing.T, content fileConfig) string {
	t.Helper()
	file, err := os.CreateTemp("", "client_config_*.json")
	require.NoError(t, err)
	defer file.Close()

	encoder := json.NewEncoder(file)
	err = encoder.Encode(content)
	require.NoError(t, err)

	return file.Name()
}

// TestClientConfig_ParseFlagsFromArgs проверяет парсинг флагов, переменных окружения и JSON-конфига.
func TestClientConfig_ParseFlagsFromArgs(t *testing.T) {
	// Базовый JSON контент для тестов
	jsonContent := fileConfig{
		Address:        "json-host:3333",
		ReportInterval: "33s",
		PollInterval:   "3s",
		CryptoKey:      "/tmp/json.key",
	}

	tests := []struct {
		name    string
		args    []string          // Аргументы командной строки
		envVars map[string]string // Переменные окружения
		useJSON bool              // Использовать ли JSON конфиг
		want    ClientConfig      // Ожидаемый результат
	}{
		{
			name:    "Default values",
			args:    []string{},
			envVars: map[string]string{},
			want: ClientConfig{
				ReportInterval: 10,
				PollInterval:   2,
				Compress:       "gzip",
				RateLimit:      5,
				Key:            "",
				CryptoKey:      "",
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
				"-c", "false", // Проверка флага компрессии
			},
			envVars: map[string]string{},
			want: ClientConfig{
				ReportInterval: 20,
				PollInterval:   5,
				Compress:       "false", // Флаг сработал
				Key:            "secret",
				RateLimit:      10,
				CryptoKey:      "",
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
				"COMPRESS":        "best",
			},
			want: ClientConfig{
				ReportInterval: 30,
				PollInterval:   2,
				Compress:       "best",
				RateLimit:      5,
				Key:            "env-secret",
				CryptoKey:      "",
				Addr:           addr.Addr{Host: "env-host", Port: 7070},
			},
		},
		{
			name: "Env KEY priority over Flag k",
			args: []string{"-k", "flag-secret"},
			envVars: map[string]string{
				"KEY": "env-priority-secret",
			},
			want: ClientConfig{
				ReportInterval: 10,
				PollInterval:   2,
				Compress:       "gzip",
				RateLimit:      5,
				Key:            "env-priority-secret",
				CryptoKey:      "",
				Addr:           addr.Addr{Host: "localhost", Port: 8080},
			},
		},
		{
			name:    "JSON Config via -config flag",
			args:    []string{}, // Путь добавится в тесте
			envVars: map[string]string{},
			useJSON: true,
			want: ClientConfig{
				ReportInterval: 33, // Из JSON
				PollInterval:   3,  // Из JSON
				Compress:       "gzip",
				RateLimit:      5,
				CryptoKey:      "/tmp/json.key",                          // Из JSON
				Addr:           addr.Addr{Host: "json-host", Port: 3333}, // Из JSON
			},
		},
		{
			name: "Priority: Env > JSON",
			args: []string{},
			envVars: map[string]string{
				"REPORT_INTERVAL": "99", // Env
			},
			useJSON: true, // JSON (33s)
			want: ClientConfig{
				ReportInterval: 99, // Env победил
				PollInterval:   3,  // Из JSON
				Compress:       "gzip",
				Addr:           addr.Addr{Host: "json-host", Port: 3333}, // Из JSON
				CryptoKey:      "/tmp/json.key",
				RateLimit:      5,
			},
		},
		{
			name:    "Priority: Flag > JSON",
			args:    []string{"-r", "77"}, // Flag
			envVars: map[string]string{},
			useJSON: true, // JSON (33s)
			want: ClientConfig{
				ReportInterval: 77, // Flag победил
				PollInterval:   3,  // Из JSON
				Addr:           addr.Addr{Host: "json-host", Port: 3333},
				CryptoKey:      "/tmp/json.key",
				Compress:       "gzip",
				RateLimit:      5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. Очистка ENV
			os.Clearenv()
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}
			defer os.Clearenv()

			// 2. Подготовка конфига
			cfg := InitialFlags()
			args := tt.args

			if tt.useJSON {
				configPath := createTempConfig(t, jsonContent)
				defer os.Remove(configPath)
				// Используем -config, так как -c занят под компрессию
				args = append(args, "-config", configPath)
			}

			// 3. Парсинг (используем helper, который создает локальный FlagSet)
			err := cfg.ParseFlagsFromArgs(args)
			require.NoError(t, err)

			// 4. Проверки
			assert.Equal(t, tt.want.ReportInterval, cfg.ReportInterval, "ReportInterval mismatch")
			assert.Equal(t, tt.want.PollInterval, cfg.PollInterval, "PollInterval mismatch")
			assert.Equal(t, tt.want.RateLimit, cfg.RateLimit, "RateLimit mismatch")
			assert.Equal(t, tt.want.Compress, cfg.Compress, "Compress mismatch")
			assert.Equal(t, tt.want.Key, cfg.Key, "Key mismatch")

			// Проверка CryptoKey (если задан)
			if tt.want.CryptoKey != "" {
				assert.Equal(t, tt.want.CryptoKey, cfg.CryptoKey, "CryptoKey mismatch")
			}

			// Проверка Addr
			assert.Equal(t, tt.want.Addr.Host, cfg.Addr.Host, "Addr.Host mismatch")
			assert.Equal(t, tt.want.Addr.Port, cfg.Addr.Port, "Addr.Port mismatch")
		})
	}
}

// TestClientConfig_Getters проверяет геттеры.
func TestClientConfig_Getters(t *testing.T) {
	cfg := ClientConfig{}
	_ = cfg.Addr.Set("10.0.0.1:5432")

	assert.Equal(t, "10.0.0.1", cfg.GetHost())
	assert.Equal(t, ":5432", cfg.GetPort())
}

// ExampleClientConfig_GetPort
func ExampleClientConfig_GetPort() {
	cfg := InitialFlags()
	_ = cfg.Addr.Set("example.com:80")

	fmt.Println(cfg.GetPort())

	// Output:
	// :80
}

// ExampleClientConfig_GetHost
func ExampleClientConfig_GetHost() {
	cfg := InitialFlags()
	_ = cfg.Addr.Set("db.internal:5432")

	fmt.Println(cfg.GetHost())

	// Output:
	// db.internal
}
