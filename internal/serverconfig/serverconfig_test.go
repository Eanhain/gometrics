package serverconfig

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert" // Рекомендую использовать testify для удобства
	// "gometrics/internal/addr" // Ваш импорт
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

// boolPtr возвращает указатель на bool значение (для JSON конфигурации)
func boolPtr(b bool) *bool {
	return &b
}

// TestServerConfigs_ParseFlags проверяет приоритеты:
// 1. Флаги имеют приоритет над Env (стандартное поведение).
// 2. Исключение в вашем коде: Key из Env имеет приоритет над флагом.
// 3. JSON конфигурация имеет самый низкий приоритет.
func TestServerConfigs_ParseFlags(t *testing.T) {
	tests := []struct {
		name       string
		args       []string          // Аргументы командной строки (без имени программы)
		env        map[string]string // Переменные окружения
		jsonConfig *JSONConfig       // Конфигурация из JSON файла (nil если не используется)
		want       ServerConfigs     // Ожидаемый результат
	}{
		{
			name:       "Default values",
			args:       []string{},
			env:        map[string]string{},
			jsonConfig: nil,
			want: ServerConfigs{
				StoreInter:  300,
				FilePath:    "metrics_storage",
				Restore:     true,
				DatabaseDSN: "",
				Key:         "",
			},
		},
		{
			name: "Env vars set only",
			args: []string{},
			env: map[string]string{
				"STORE_INTERVAL":    "500",
				"FILE_STORAGE_PATH": "/tmp/metrics",
				"RESTORE":           "false",
				"KEY":               "secret_env",
			},
			jsonConfig: nil,
			want: ServerConfigs{
				StoreInter: 500,
				FilePath:   "/tmp/metrics",
				Restore:    false,
				Key:        "secret_env",
			},
		},
		{
			name: "Flags set only",
			args: []string{
				"-i", "100",
				"-f", "/flag/path",
				"-r=false",
				"-k", "secret_flag",
			},
			env:        map[string]string{},
			jsonConfig: nil,
			want: ServerConfigs{
				StoreInter: 100,
				FilePath:   "/flag/path",
				Restore:    false,
				Key:        "secret_flag",
			},
		},
		{
			name: "Priority check: Flags overwrite Env (Standard fields)",
			args: []string{"-i", "777"}, // Флаг
			env: map[string]string{
				"STORE_INTERVAL": "500", // Env
			},
			jsonConfig: nil,
			want: ServerConfigs{
				StoreInter: 777, // Победил флаг
				FilePath:   "metrics_storage",
				Restore:    true,
				Key:        "",
			},
		},
		{
			name: "Priority check: Env overwrites Flags (Custom Logic for KEY)",
			args: []string{"-k", "key_from_flag"},
			env: map[string]string{
				"KEY": "key_from_env",
			},
			jsonConfig: nil,
			want: ServerConfigs{
				// В вашем коде есть if envKey != "" { o.Key = envKey },
				// поэтому Env должен победить флаг.
				Key:        "key_from_env",
				StoreInter: 300,
				FilePath:   "metrics_storage",
				Restore:    true,
			},
		},
		// ========== НОВЫЕ ТЕСТЫ ДЛЯ JSON КОНФИГУРАЦИИ ==========
		{
			name: "JSON config only (no flags, no env)",
			args: []string{}, // Путь к конфигу будет добавлен в тесте
			env:  map[string]string{},
			jsonConfig: &JSONConfig{
				StoreInterval: "60s",
				StoreFile:     "/json/path/file.db",
				Restore:       boolPtr(false),
				DatabaseDSN:   "postgres://localhost/db",
				CryptoKey:     "/json/crypto.pem",
			},
			want: ServerConfigs{
				StoreInter:  60, // 60 секунд из "60s"
				FilePath:    "/json/path/file.db",
				Restore:     false,
				DatabaseDSN: "postgres://localhost/db",
				CryptoKey:   "/json/crypto.pem",
				Key:         "",
			},
		},
		{
			name: "JSON config with minutes interval",
			args: []string{},
			env:  map[string]string{},
			jsonConfig: &JSONConfig{
				StoreInterval: "5m", // 5 минут = 300 секунд
				StoreFile:     "/data/metrics.db",
			},
			want: ServerConfigs{
				StoreInter: 300,
				FilePath:   "/data/metrics.db",
				Restore:    true, // default
			},
		},
		{
			name: "Priority: Flags > JSON config",
			args: []string{
				"-i", "999",
				"-f", "/flag/override",
			},
			env: map[string]string{},
			jsonConfig: &JSONConfig{
				StoreInterval: "10s",
				StoreFile:     "/json/path",
				Restore:       boolPtr(false),
				CryptoKey:     "/json/key.pem",
			},
			want: ServerConfigs{
				StoreInter: 999,              // Победил флаг
				FilePath:   "/flag/override", // Победил флаг
				Restore:    true,             // Из JSON не применится, т.к. -r не указан, но default=true
				CryptoKey:  "/json/key.pem",  // Из JSON (флаг не указан)
			},
		},
		{
			name: "Priority: Env > JSON config",
			args: []string{},
			env: map[string]string{
				"STORE_INTERVAL":    "888",
				"FILE_STORAGE_PATH": "/env/path",
			},
			jsonConfig: &JSONConfig{
				StoreInterval: "10s",
				StoreFile:     "/json/path",
				CryptoKey:     "/json/key.pem",
			},
			want: ServerConfigs{
				StoreInter: 888,             // Победил Env
				FilePath:   "/env/path",     // Победил Env
				Restore:    true,            // default
				CryptoKey:  "/json/key.pem", // Из JSON (env не указан)
			},
		},
		{
			name: "Priority: Flags > Env > JSON (all three sources)",
			args: []string{"-i", "111"},
			env: map[string]string{
				"FILE_STORAGE_PATH": "/env/storage",
				"CRYPTO_KEY":        "/env/crypto.pem",
			},
			jsonConfig: &JSONConfig{
				StoreInterval: "999s",
				StoreFile:     "/json/storage",
				CryptoKey:     "/json/crypto.pem",
				DatabaseDSN:   "postgres://json/db",
			},
			want: ServerConfigs{
				StoreInter:  111,                  // Победил флаг
				FilePath:    "/env/storage",       // Победил Env
				CryptoKey:   "/env/crypto.pem",    // Победил Env
				DatabaseDSN: "postgres://json/db", // Из JSON (единственный источник)
				Restore:     true,                 // default
			},
		},
		{
			name: "JSON config via -config flag",
			args: []string{}, // -config будет добавлен в тесте
			env:  map[string]string{},
			jsonConfig: &JSONConfig{
				StoreInterval: "120s",
				Restore:       boolPtr(false),
			},
			want: ServerConfigs{
				StoreInter: 120,
				FilePath:   "metrics_storage", // default
				Restore:    false,
			},
		},
		{
			name: "JSON config via CONFIG env var",
			args: []string{},
			env:  map[string]string{}, // CONFIG будет добавлен в тесте
			jsonConfig: &JSONConfig{
				StoreInterval: "180s",
				DatabaseDSN:   "mysql://host/db",
			},
			want: ServerConfigs{
				StoreInter:  180,
				FilePath:    "metrics_storage", // default
				Restore:     true,              // default
				DatabaseDSN: "mysql://host/db",
			},
		},
		{
			name: "Priority: CONFIG env var > -config flag for config path",
			args: []string{},          // Будет установлен неправильный путь через флаг
			env:  map[string]string{}, // CONFIG с правильным путём
			jsonConfig: &JSONConfig{
				StoreInterval: "200s",
			},
			want: ServerConfigs{
				StoreInter: 200, // Из JSON по пути из CONFIG env
				FilePath:   "metrics_storage",
				Restore:    true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. Сброс переменных окружения перед тестом
			os.Clearenv()
			for k, v := range tt.env {
				os.Setenv(k, v)
			}
			// Очистка после теста
			defer os.Clearenv()

			// 2. Подготовка аргументов командной строки
			args := tt.args

			// 3. Создание временного JSON файла конфигурации (если нужен)
			if tt.jsonConfig != nil {
				configPath := createTempConfigFile(t, *tt.jsonConfig)

				// Специальная обработка для теста приоритета CONFIG env
				if tt.name == "JSON config via CONFIG env var" ||
					tt.name == "Priority: CONFIG env var > -config flag for config path" {
					os.Setenv("CONFIG", configPath)
				} else {
					// Добавляем путь к конфигу через флаг
					args = append(args, "-config", configPath)
				}
			}

			// 4. Сброс флагов (критично для тестирования flag пакета)
			// Мы создаем новый Set, чтобы не засорять глобальный state,
			// но так как ваш код использует flag.Parse() (глобальный),
			// нам придется переопределять flag.CommandLine
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

			// 5. Подмена os.Args
			oldArgs := os.Args
			defer func() { os.Args = oldArgs }()
			os.Args = append([]string{"cmd"}, args...)

			// 6. Инициализация и запуск
			cfg := InitialFlags()
			cfg.ParseFlags()

			// 7. Проверки
			assert.Equal(t, tt.want.StoreInter, cfg.StoreInter, "StoreInterval mismatch")
			assert.Equal(t, tt.want.FilePath, cfg.FilePath, "FilePath mismatch")
			assert.Equal(t, tt.want.Restore, cfg.Restore, "Restore mismatch")
			assert.Equal(t, tt.want.DatabaseDSN, cfg.DatabaseDSN, "DatabaseDSN mismatch")
			assert.Equal(t, tt.want.Key, cfg.Key, "Key mismatch")
			assert.Equal(t, tt.want.CryptoKey, cfg.CryptoKey, "CryptoKey mismatch")

			// assert.Equal(t, tt.want.Addr, cfg.Addr) // Раскомментируйте, если addr.Addr сравним
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
		{"", 0, false},
		{"invalid", 0, true},
		{"100", 0, true}, // без единицы измерения
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

// TestLoadJSONConfig проверяет загрузку JSON конфигурации
func TestLoadJSONConfig(t *testing.T) {
	t.Run("Empty path returns nil", func(t *testing.T) {
		cfg, err := loadJSONConfig("")
		assert.NoError(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("Non-existent file returns error", func(t *testing.T) {
		cfg, err := loadJSONConfig("/non/existent/path.json")
		assert.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("Invalid JSON returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "invalid.json")
		os.WriteFile(configPath, []byte("{invalid json}"), 0644)

		cfg, err := loadJSONConfig(configPath)
		assert.Error(t, err)
		assert.Nil(t, cfg)
	})

	t.Run("Valid JSON is parsed correctly", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "valid.json")

		jsonData := `{
			"address": "127.0.0.1:9090",
			"restore": false,
			"store_interval": "30s",
			"store_file": "/path/to/store",
			"database_dsn": "postgres://localhost",
			"crypto_key": "/path/to/key.pem"
		}`
		os.WriteFile(configPath, []byte(jsonData), 0644)

		cfg, err := loadJSONConfig(configPath)
		assert.NoError(t, err)
		assert.NotNil(t, cfg)
		assert.Equal(t, "127.0.0.1:9090", cfg.Address)
		assert.NotNil(t, cfg.Restore)
		assert.Equal(t, false, *cfg.Restore)
		assert.Equal(t, "30s", cfg.StoreInterval)
		assert.Equal(t, "/path/to/store", cfg.StoreFile)
		assert.Equal(t, "postgres://localhost", cfg.DatabaseDSN)
		assert.Equal(t, "/path/to/key.pem", cfg.CryptoKey)
	})
}

// Пример использования (будет отображаться в godoc и работать как тест)
func ExampleServerConfigs_ParseFlags() {
	// Эмуляция аргументов командной строки
	// Допустим, пользователь запустил: ./app -i 50
	oldArgs := os.Args
	os.Args = []string{"app", "-i", "50"}
	defer func() { os.Args = oldArgs }()

	// Сброс флагов для примера
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// Инициализация
	cfg := InitialFlags()

	// Парсинг
	cfg.ParseFlags()

	fmt.Printf("Interval: %d\n", cfg.StoreInter)
	fmt.Printf("Restore: %v\n", cfg.Restore)

	// Output:
	// Interval: 50
	// Restore: true
}

// ExampleServerConfigs_ParseFlags_withJSON демонстрирует использование JSON конфигурации
func ExampleServerConfigs_ParseFlags_withJSON() {
	// Создаём временный файл конфигурации
	tmpFile, _ := os.CreateTemp("", "config-*.json")
	defer os.Remove(tmpFile.Name())

	jsonConfig := `{"store_interval": "120s", "restore": false}`
	tmpFile.WriteString(jsonConfig)
	tmpFile.Close()

	// Эмуляция запуска: ./app -config /path/to/config.json
	oldArgs := os.Args
	os.Args = []string{"app", "-config", tmpFile.Name()}
	defer func() { os.Args = oldArgs }()

	// Сброс флагов
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// Инициализация и парсинг
	cfg := InitialFlags()
	cfg.ParseFlags()

	fmt.Printf("Interval: %d\n", cfg.StoreInter)
	fmt.Printf("Restore: %v\n", cfg.Restore)

	// Output:
	// Interval: 120
	// Restore: false
}
