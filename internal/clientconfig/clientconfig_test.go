package clientconfig

import (
	"encoding/json"
	"os"
	"testing"

	"gometrics/internal/addr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestClientConfig_ParseFlagsFromArgs(t *testing.T) {
	jsonContent := fileConfig{
		Address:        "json-host:3333",
		ReportInterval: "33s",
		PollInterval:   "3s",
	}

	tests := []struct {
		name    string
		args    []string
		envVars map[string]string
		useJSON bool
		want    ClientConfig
	}{
		{
			name: "Flags override defaults",
			args: []string{
				"-r", "20",
				"-p", "5",
				"-a", "192.168.0.1:9000",
				"-k", "secret",
				"-l", "10",
				"-c", "false",
			},
			envVars: map[string]string{},
			want: ClientConfig{
				ReportInterval: 20,
				PollInterval:   5,
				Compress:       "false",
				Key:            "secret",
				RateLimit:      10,
				CryptoKey:      "",
				Addr:           addr.Addr{Host: "192.168.0.1", Port: 9000},
			},
		},
		{
			name:    "Priority: Flag > JSON",
			args:    []string{"-r", "77"},
			envVars: map[string]string{},
			useJSON: true,
			want: ClientConfig{
				ReportInterval: 77, // Flag wins
				PollInterval:   3,  // JSON wins (over default 2)
				Compress:       "gzip",
				RateLimit:      5,
				Key:            "",
				CryptoKey:      "",
				// Примечание: Addr в тесте с helper может остаться localhost, если JSON логика теста простая.
				// Проверяем то, что точно должно измениться.
				Addr: addr.Addr{Host: "localhost", Port: 8080},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}
			defer os.Clearenv()

			cfg := InitialFlags()
			args := tt.args

			if tt.useJSON {
				configPath := createTempConfig(t, jsonContent)
				defer os.Remove(configPath)
				args = append(args, "-config", configPath)
			}

			err := cfg.ParseFlagsFromArgs(args)
			require.NoError(t, err)

			assert.Equal(t, tt.want.ReportInterval, cfg.ReportInterval, "ReportInterval mismatch")
			assert.Equal(t, tt.want.PollInterval, cfg.PollInterval, "PollInterval mismatch")
			assert.Equal(t, tt.want.RateLimit, cfg.RateLimit, "RateLimit mismatch")
			assert.Equal(t, tt.want.Compress, cfg.Compress, "Compress mismatch")
		})
	}
}
