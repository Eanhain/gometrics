package serverconfig

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert" // Рекомендую использовать testify для удобства
	// "gometrics/internal/addr" // Ваш импорт
)

// TestServerConfigs_ParseFlags проверяет приоритеты:
// 1. Флаги имеют приоритет над Env (стандартное поведение).
// 2. Исключение в вашем коде: Key из Env имеет приоритет над флагом.
func TestServerConfigs_ParseFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string          // Аргументы командной строки (без имени программы)
		env  map[string]string // Переменные окружения
		want ServerConfigs     // Ожидаемый результат
	}{
		{
			name: "Default values",
			args: []string{},
			env:  map[string]string{},
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
			env: map[string]string{},
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
			want: ServerConfigs{
				// В вашем коде есть if envKey != "" { o.Key = envKey },
				// поэтому Env должен победить флаг.
				Key:        "key_from_env",
				StoreInter: 300,
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

			// 2. Сброс флагов (критично для тестирования flag пакета)
			// Мы создаем новый Set, чтобы не засорять глобальный state,
			// но так как ваш код использует flag.Parse() (глобальный),
			// нам придется переопределять flag.CommandLine
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

			// 3. Подмена os.Args
			oldArgs := os.Args
			defer func() { os.Args = oldArgs }()
			os.Args = append([]string{"cmd"}, tt.args...)

			// 4. Инициализация и запуск
			cfg := InitialFlags()
			cfg.ParseFlags()

			// 5. Проверки
			assert.Equal(t, tt.want.StoreInter, cfg.StoreInter, "StoreInterval mismatch")
			assert.Equal(t, tt.want.FilePath, cfg.FilePath, "FilePath mismatch")
			assert.Equal(t, tt.want.Restore, cfg.Restore, "Restore mismatch")
			assert.Equal(t, tt.want.DatabaseDSN, cfg.DatabaseDSN, "DatabaseDSN mismatch")
			assert.Equal(t, tt.want.Key, cfg.Key, "Key mismatch")

			// assert.Equal(t, tt.want.Addr, cfg.Addr) // Раскомментируйте, если addr.Addr сравним
		})
	}
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
