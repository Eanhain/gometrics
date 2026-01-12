package serverconfig

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// createTempConfigFile создаёт временный JSON файл конфигурации для тестов.
// Файл автоматически удаляется после завершения теста.
func createTempConfigFile(t *testing.T, cfg JSONConfig) string {
	t.Helper()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	data, _ := json.Marshal(cfg)
	os.WriteFile(configPath, data, 0644)
	return configPath
}

// boolPtr возвращает указатель на bool значение.
// Используется для JSON конфигурации, где nil означает "не задано".
func boolPtr(b bool) *bool { return &b }

// TestServerConfigs_ParseFlagsFromArgs проверяет парсинг конфигурации с учётом приоритетов:
//  1. Флаги командной строки (высший приоритет)
//  2. Переменные окружения
//  3. JSON файл конфигурации
//  4. Значения по умолчанию (низший приоритет)
//
// Исключение: для поля Key переменная окружения KEY имеет приоритет над флагом -k.
func TestServerConfigs_ParseFlagsFromArgs(t *testing.T) {
	tests := []struct {
		name       string            // Название тест-кейса
		args       []string          // Аргументы командной строки (флаги)
		env        map[string]string // Переменные окружения
		jsonConfig *JSONConfig       // JSON конфигурация (nil если не используется)
		want       ServerConfigs     // Ожидаемый результат
	}{
		{
			name: "Default values",
			args: []string{}, env: map[string]string{}, jsonConfig: nil,
			want: ServerConfigs{StoreInter: 300, FilePath: "metrics_storage", Restore: true},
		},
		{
			name: "Env vars set only",
			args: []string{},
			env: map[string]string{
				"STORE_INTERVAL": "500", "FILE_STORAGE_PATH": "/tmp/metrics",
				"RESTORE": "false", "KEY": "secret_env",
			},
			want: ServerConfigs{StoreInter: 500, FilePath: "/tmp/metrics", Restore: false, Key: "secret_env"},
		},
		{
			name: "Flags set only",
			args: []string{"-i", "100", "-f", "/flag/path", "-r=false", "-k", "secret_flag"},
			env:  map[string]string{},
			want: ServerConfigs{StoreInter: 100, FilePath: "/flag/path", Restore: false, Key: "secret_flag"},
		},
		{
			name: "Flags > Env (стандартные поля)",
			args: []string{"-i", "777"},
			env:  map[string]string{"STORE_INTERVAL": "500"},
			want: ServerConfigs{StoreInter: 777, FilePath: "metrics_storage", Restore: true},
		},
		{
			name: "Env KEY > Flag k (особая логика)",
			args: []string{"-k", "flag_key"},
			env:  map[string]string{"KEY": "env_key"},
			want: ServerConfigs{StoreInter: 300, FilePath: "metrics_storage", Restore: true, Key: "env_key"},
		},
		// ========== ТЕСТЫ ДЛЯ JSON КОНФИГУРАЦИИ ==========
		{
			name: "JSON config only",
			args: []string{}, env: map[string]string{},
			jsonConfig: &JSONConfig{
				StoreInterval: "60s", StoreFile: "/json/file.db",
				Restore: boolPtr(false), DatabaseDSN: "postgres://db", CryptoKey: "/json/crypto.pem",
			},
			want: ServerConfigs{
				StoreInter: 60, FilePath: "/json/file.db", Restore: false,
				DatabaseDSN: "postgres://db", CryptoKey: "/json/crypto.pem",
			},
		},
		{
			name: "Flags > JSON",
			args: []string{"-i", "999", "-f", "/flag/override", "-r=true"},
			jsonConfig: &JSONConfig{
				StoreInterval: "10s", StoreFile: "/json/path",
				Restore: boolPtr(false), CryptoKey: "/json/key.pem",
			},
			want: ServerConfigs{
				StoreInter: 999, FilePath: "/flag/override",
				Restore: true, CryptoKey: "/json/key.pem", // CryptoKey из JSON
			},
		},
		{
			name: "Env > JSON",
			env:  map[string]string{"STORE_INTERVAL": "888", "FILE_STORAGE_PATH": "/env/path"},
			jsonConfig: &JSONConfig{
				StoreInterval: "10s", StoreFile: "/json/path", CryptoKey: "/json/key.pem",
			},
			want: ServerConfigs{
				StoreInter: 888, FilePath: "/env/path",
				Restore: true, CryptoKey: "/json/key.pem",
			},
		},
		{
			name: "Flags > Env > JSON",
			args: []string{"-i", "111"},
			env:  map[string]string{"FILE_STORAGE_PATH": "/env/storage", "CRYPTO_KEY": "/env/crypto.pem"},
			jsonConfig: &JSONConfig{
				StoreInterval: "999s", StoreFile: "/json/storage",
				CryptoKey: "/json/crypto.pem", DatabaseDSN: "postgres://json/db",
			},
			want: ServerConfigs{
				StoreInter: 111, FilePath: "/env/storage",
				CryptoKey: "/env/crypto.pem", DatabaseDSN: "postgres://json/db", Restore: true,
			},
		},
		{
			name:       "JSON via CONFIG env var",
			env:        map[string]string{},
			jsonConfig: &JSONConfig{StoreInterval: "180s", DatabaseDSN: "mysql://host/db"},
			want: ServerConfigs{
				StoreInter: 180, FilePath: "metrics_storage",
				Restore: true, DatabaseDSN: "mysql://host/db",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. Очищаем и устанавливаем переменные окружения
			os.Clearenv()
			for k, v := range tt.env {
				os.Setenv(k, v)
			}
			defer os.Clearenv()

			// 2. Подготавливаем аргументы
			args := tt.args

			// 3. Создаём временный JSON файл (если нужен)
			if tt.jsonConfig != nil {
				configPath := createTempConfigFile(t, *tt.jsonConfig)
				if tt.name == "JSON via CONFIG env var" {
					os.Setenv("CONFIG", configPath)
				} else {
					args = append(args, "-config", configPath)
				}
			}

			// 4. Инициализация и парсинг
			cfg := InitialFlags()
			err := cfg.ParseFlagsFromArgs(args)
			assert.NoError(t, err)

			// 5. Проверка результатов
			assert.Equal(t, tt.want.StoreInter, cfg.StoreInter, "StoreInter")
			assert.Equal(t, tt.want.FilePath, cfg.FilePath, "FilePath")
			assert.Equal(t, tt.want.Restore, cfg.Restore, "Restore")
			assert.Equal(t, tt.want.DatabaseDSN, cfg.DatabaseDSN, "DatabaseDSN")
			assert.Equal(t, tt.want.Key, cfg.Key, "Key")
			assert.Equal(t, tt.want.CryptoKey, cfg.CryptoKey, "CryptoKey")
		})
	}
}

// TestServerConfigs_ParseFlags проверяет основной метод ParseFlags.
func TestServerConfigs_ParseFlags(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Clearenv()
	os.Args = []string{"cmd", "-i", "50"}

	cfg := InitialFlags()
	cfg.ParseFlags()

	assert.Equal(t, 50, cfg.StoreInter)
	assert.Equal(t, true, cfg.Restore)
}

// TestParseInterval проверяет парсинг строк интервала в секунды.
func TestParseInterval(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		hasError bool
	}{
		{"1s", 1, false}, {"60s", 60, false}, {"5m", 300, false},
		{"1h", 3600, false}, {"1m30s", 90, false}, {"", 0, false},
		{"invalid", 0, true}, {"100", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseInterval(tt.input)
			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestLoadJSONConfig проверяет загрузку JSON файла конфигурации.
func TestLoadJSONConfig(t *testing.T) {
	t.Run("Empty path", func(t *testing.T) {
		cfg, err := loadJSONConfig("")
		assert.NoError(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("Non-existent", func(t *testing.T) {
		_, err := loadJSONConfig("/no/file.json")
		assert.Error(t, err)
	})

	t.Run("Valid JSON", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "cfg.json")
		os.WriteFile(p, []byte(`{"store_interval":"30s","restore":false}`), 0644)
		cfg, err := loadJSONConfig(p)
		assert.NoError(t, err)
		assert.Equal(t, "30s", cfg.StoreInterval)
		assert.Equal(t, false, *cfg.Restore)
	})
}

// ExampleServerConfigs_ParseFlagsFromArgs демонстрирует использование JSON конфигурации.
func ExampleServerConfigs_ParseFlagsFromArgs() {
	f, _ := os.CreateTemp("", "*.json")
	defer os.Remove(f.Name())
	f.WriteString(`{"store_interval":"120s","restore":false}`)
	f.Close()

	cfg := InitialFlags()
	cfg.ParseFlagsFromArgs([]string{"-config", f.Name()})

	fmt.Printf("Interval: %d\n", cfg.StoreInter)
	fmt.Printf("Restore: %v\n", cfg.Restore)
	// Output:
	// Interval: 120
	// Restore: false
}
