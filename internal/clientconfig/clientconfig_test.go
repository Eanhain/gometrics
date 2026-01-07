package clientconfig

import (
	"fmt"
	"os"
	"testing"

	"gometrics/internal/addr"
)

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
		name    string
		args    []string          // Аргументы командной строки (флаги)
		envVars map[string]string // Переменные окружения
		want    ClientConfig      // Ожидаемая итоговая конфигурация
	}{
		{
			name:    "Default values",
			args:    []string{},
			envVars: map[string]string{},
			want: ClientConfig{
				ReportInterval: 10,                                       // envDefault:"10"
				PollInterval:   2,                                        // envDefault:"2"
				Compress:       "gzip",                                   // envDefault:"gzip"
				RateLimit:      5,                                        // envDefault:"5"
				Key:            "",                                       // envDefault:""
				Addr:           addr.Addr{Host: "localhost", Port: 8080}, // envDefault
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
			envVars: map[string]string{},
			want: ClientConfig{
				ReportInterval: 20,
				PollInterval:   5,
				Compress:       "gzip", // Флаг не передавали, остался дефолт
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
			want: ClientConfig{
				ReportInterval: 30,
				PollInterval:   2,      // Default
				Compress:       "gzip", // Default
				RateLimit:      5,      // Default
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
			want: ClientConfig{
				ReportInterval: 50, // Flag
				PollInterval:   8,  // Env
				Compress:       "gzip",
				RateLimit:      5,
				Key:            "",
				Addr:           addr.Addr{Host: "localhost", Port: 8080},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. Настройка окружения
			cleanup := setEnv(tt.envVars)
			defer cleanup()

			// 2. Инициализация и парсинг
			cfg := InitialFlags()
			err := cfg.ParseFlagsFromArgs(tt.args)
			if err != nil {
				t.Fatalf("ParseFlagsFromArgs failed: %v", err)
			}

			// 3. Полевая проверка результатов
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

			// Сравнение структуры Addr
			if cfg.Addr.Host != tt.want.Addr.Host || cfg.Addr.Port != tt.want.Addr.Port {
				t.Errorf("Addr = %v, want %v", cfg.Addr, tt.want.Addr)
			}
		})
	}
}

// TestClientConfig_Getters проверяет методы-геттеры для хоста и порта.
func TestClientConfig_Getters(t *testing.T) {
	cfg := ClientConfig{}
	// Эмулируем установку адреса, как это делает парсер
	_ = cfg.Addr.Set("10.0.0.1:5432")

	if got := cfg.GetHost(); got != "10.0.0.1" {
		t.Errorf("GetHost() = %v, want %v", got, "10.0.0.1")
	}

	if got := cfg.GetPort(); got != ":5432" {
		t.Errorf("GetPort() = %v, want %v", got, ":5432")
	}
}

// ExampleClientConfig_GetPort демонстрирует использование метода GetPort.
// Этот пример попадет в сгенерированную документацию godoc.
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
