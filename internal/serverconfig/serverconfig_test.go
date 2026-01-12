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

func createTempConfig(t *testing.T, content fileConfig) string {
	t.Helper()
	file, err := os.CreateTemp("", "server_config_*.json")
	require.NoError(t, err)
	defer file.Close()

	encoder := json.NewEncoder(file)
	err = encoder.Encode(content)
	require.NoError(t, err)

	return file.Name()
}

func TestServerConfigs_ParseFlags(t *testing.T) {
	jsonContent := fileConfig{
		Address:       "localhost:9090",
		Restore:       false,
		StoreInterval: "1s",
		StoreFile:     "/tmp/json_metrics",
	}

	tests := []struct {
		name    string
		args    []string
		env     map[string]string
		useJSON bool
		want    ServerConfigs
	}{
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
			name: "Priority check: Flags overwrite everything",
			args: []string{"-i", "777"},
			env: map[string]string{
				"STORE_INTERVAL": "888",
			},
			useJSON: true,
			want: ServerConfigs{
				StoreInter: 777,
				FilePath:   "/tmp/json_metrics",
				Restore:    false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. Очистка ENV
			os.Clearenv()
			for k, v := range tt.env {
				os.Setenv(k, v)
			}
			defer os.Clearenv()

			// 2. ЖЕСТКИЙ СБРОС ГЛОБАЛЬНЫХ ФЛАГОВ
			// Создаем FlagSet с именем "cmd", чтобы эмулировать реальный запуск
			flag.CommandLine = flag.NewFlagSet("cmd", flag.ContinueOnError)

			// 3. Подготовка os.Args
			// os.Args[0] должно быть именем программы
			args := append([]string{"cmd"}, tt.args...)

			if tt.useJSON {
				configPath := createTempConfig(t, jsonContent)
				defer os.Remove(configPath)
				args = append(args, "-config", configPath)
			}

			// Подменяем os.Args глобально
			oldArgs := os.Args
			os.Args = args
			defer func() { os.Args = oldArgs }()

			// 4. Парсинг
			cfg := InitialFlags()
			cfg.ParseFlags()

			// 5. Проверки
			assert.Equal(t, tt.want.StoreInter, cfg.StoreInter, "StoreInter mismatch")
			if tt.want.FilePath != "" {
				assert.Equal(t, tt.want.FilePath, cfg.FilePath, "FilePath mismatch")
			}
			assert.Equal(t, tt.want.Restore, cfg.Restore, "Restore mismatch")
		})
	}
}

func ExampleServerConfigs_ParseFlags() {
	// Сброс флагов для примера
	flag.CommandLine = flag.NewFlagSet("example", flag.ContinueOnError)

	oldArgs := os.Args
	os.Args = []string{"example", "-i", "50"}
	defer func() { os.Args = oldArgs }()

	cfg := InitialFlags()
	cfg.ParseFlags()

	fmt.Printf("Store Interval: %d\n", cfg.StoreInter)

	// Output:
	// Store Interval: 50
}
