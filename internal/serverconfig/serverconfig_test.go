package serverconfig

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTempConfig создает временный JSON-файл с конфигурацией для тестов.
// Возвращает путь к созданному файлу. Файл автоматически удаляется при завершении теста (если использовать t.Cleanup, но здесь мы удаляем вручную в defer).
func createTempConfig(t *testing.T, content fileConfig) string {
	t.Helper() // Помечает функцию как вспомогательную для корректного вывода стека ошибок

	// Создаем временный файл с префиксом "config_" и суффиксом ".json"
	file, err := os.CreateTemp("", "config_*.json")
	require.NoError(t, err, "Не удалось создать временный файл конфигурации")

	// Закрываем файл после записи
	defer file.Close()

	// Записываем структуру fileConfig в формате JSON
	encoder := json.NewEncoder(file)
	err = encoder.Encode(content)
	require.NoError(t, err, "Не удалось записать JSON в файл")

	return file.Name()
}

// TestServerConfigs_ParseFlags проверяет логику парсинга конфигурации.
// Приоритет значений (от высшего к низшему):
// 1. Флаги командной строки (Flags)
// 2. Переменные окружения (Env)
// 3. Файл конфигурации JSON (Config File)
// 4. Значения по умолчанию (Defaults)
func TestServerConfigs_ParseFlags(t *testing.T) {
	// Базовая конфигурация для JSON-файла, используемая в тестах
	jsonContent := fileConfig{
		Address:       "localhost:9090",
		Restore:       false,
		StoreInterval: "1s", // В JSON интервал задается строкой (например, "1s", "500ms")
		StoreFile:     "/tmp/json_metrics",
		DatabaseDSN:   "postgres://json:pass@localhost:5432/db",
		CryptoKey:     "/tmp/json_key.pem",
	}

	tests := []struct {
		name    string            // Название теста
		args    []string          // Аргументы командной строки (имитация os.Args)
		env     map[string]string // Переменные окружения
		useJSON bool              // Если true, будет создан и передан файл конфигурации
		want    ServerConfigs     // Ожидаемая итоговая конфигурация
	}{
		{
			name: "Default values (No flags, No Env, No Config)",
			args: []string{},
			env:  map[string]string{},
			want: ServerConfigs{
				// Значения по умолчанию из envDefault тэгов или инициализации
				StoreInter:  300, // envDefault:"300"
				FilePath:    "metrics_storage",
				Restore:     true,
				DatabaseDSN: "",
				Key:         "",
			},
		},
		{
			name: "Environment variables only (Env > Defaults)",
			args: []string{},
			env: map[string]string{
				"STORE_INTERVAL":    "500",
				"FILE_STORAGE_PATH": "/tmp/metrics",
				"RESTORE":           "false",
				"KEY":               "secret_env",
			},
			want: ServerConfigs{
				StoreInter: 500,
				FilePath:   "/tmp/metrics",
				Restore:    false,
				Key:        "secret_env",
			},
		},
		{
			name: "Flags only (Flags > Defaults)",
			args: []string{
				"-i", "100",
				"-f", "/flag/path",
				"-r=false",
				"-k", "secret_flag",
			},
			env: map[string]string{},
			want: ServerConfigs{
				StoreInter: 100,
				FilePath:   "/flag/path",
				Restore:    false,
				Key:        "secret_flag",
			},
		},
		{
			name:    "JSON Config only (Config > Defaults)",
			args:    []string{},
			env:     map[string]string{},
			useJSON: true, // Генерируем файл из jsonContent
			want: ServerConfigs{
				StoreInter:  1, // "1s" парсится в 1 секунду
				FilePath:    "/tmp/json_metrics",
				Restore:     false,
				DatabaseDSN: "postgres://json:pass@localhost:5432/db",
				CryptoKey:   "/tmp/json_key.pem",
			},
		},
		{
			name: "Priority check: Env overwrites JSON (Env > JSON)",
			args: []string{},
			env: map[string]string{
				"STORE_INTERVAL": "999", // Задано в Env
				// Остальные поля не заданы в Env, должны взяться из JSON
			},
			useJSON: true,
			want: ServerConfigs{
				StoreInter:  999,                 // Значение из Env (победило JSON "1s")
				FilePath:    "/tmp/json_metrics", // Значение из JSON (нет в Env)
				Restore:     false,               // Значение из JSON
				DatabaseDSN: "postgres://json:pass@localhost:5432/db",
			},
		},
		{
			name: "Priority check: Flags overwrite everything (Flag > Env > JSON)",
			args: []string{"-i", "777"}, // Задано флагом
			env: map[string]string{
				"STORE_INTERVAL": "888", // Задано в Env
			},
			useJSON: true, // В JSON "1s" (1)
			want: ServerConfigs{
				StoreInter: 777,                 // Флаг имеет высший приоритет
				FilePath:   "/tmp/json_metrics", // Из JSON (нет флага и Env)
				Restore:    false,               // Из JSON
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. Очистка окружения перед каждым тестом
			os.Clearenv()
			for k, v := range tt.env {
				os.Setenv(k, v)
			}
			// Восстанавливаем чистое окружение после теста
			defer os.Clearenv()

			// 2. Сброс состояния флагов
			// flag.CommandLine хранит глобальное состояние. Нам нужно его сбросить,
			// чтобы парсинг аргументов одного теста не влиял на другой.
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

			// 3. Подготовка аргументов командной строки
			// "cmd" - это имя программы (os.Args[0]), далее идут аргументы
			args := append([]string{"cmd"}, tt.args...)

			// Если тест требует JSON конфиг, создаем его и добавляем флаг -config
			if tt.useJSON {
				configPath := createTempConfig(t, jsonContent)
				// Удаляем файл после теста
				defer os.Remove(configPath)
				args = append(args, "-config", configPath)
			}

			// Подменяем os.Args
			oldArgs := os.Args
			os.Args = args
			defer func() { os.Args = oldArgs }()

			// 4. Инициализация конфигурации
			cfg := InitialFlags()

			// 5. Запуск тестируемого метода
			cfg.ParseFlags()

			// 6. Проверки (Assertions)

			// Проверка StoreInter
			assert.Equal(t, tt.want.StoreInter, cfg.StoreInter, "StoreInter (интервал сохранения) не совпадает")

			// Проверка FilePath
			if tt.want.FilePath != "" {
				assert.Equal(t, tt.want.FilePath, cfg.FilePath, "FilePath (путь к файлу) не совпадает")
			}

			// Проверка Restore (bool проверяем всегда)
			assert.Equal(t, tt.want.Restore, cfg.Restore, "Restore (флаг восстановления) не совпадает")

			// Проверка DatabaseDSN
			if tt.want.DatabaseDSN != "" {
				assert.Equal(t, tt.want.DatabaseDSN, cfg.DatabaseDSN, "DatabaseDSN (строка подключения к БД) не совпадает")
			}

			// Проверка Key
			if tt.want.Key != "" {
				assert.Equal(t, tt.want.Key, cfg.Key, "Key (ключ подписи) не совпадает")
			}

			// Дополнительная проверка адреса для JSON теста
			// Порт 9090 задан в jsonContent
			if tt.useJSON && len(tt.args) == 0 && len(tt.env) == 0 {
				assert.Equal(t, ":9090", cfg.GetPort(), "Порт сервера из JSON конфига не совпадает")
			}
		})
	}
}

// ExampleServerConfigs_ParseFlags демонстрирует пример работы парсера конфигурации.
// Этот код будет включен в документацию GoDoc и проверяется как тест.
func ExampleServerConfigs_ParseFlags() {
	// 1. Эмуляция запуска с флагом: ./app -i 50
	oldArgs := os.Args
	os.Args = []string{"app", "-i", "50"}
	defer func() { os.Args = oldArgs }()

	// Сброс флагов для чистоты примера
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// 2. Инициализация и парсинг
	cfg := InitialFlags()
	cfg.ParseFlags()

	// 3. Вывод результата
	fmt.Printf("Store Interval: %d\n", cfg.StoreInter)

	// Output:
	// Store Interval: 50
}
