package addr

import (
	"fmt"
	"testing"
)

// TestAddr_Set uses table-driven tests to verify the parsing logic of Set.
func TestAddr_Set(t *testing.T) {
	// Определение структуры тестового случая
	tests := []struct {
		name      string // Имя теста для вывода в логах
		input     string // Входная строка адреса
		wantHost  string // Ожидаемый хост
		wantPort  int    // Ожидаемый порт
		wantErr   bool   // Ожидается ли ошибка
		targetErr error  // Ожидаемая конкретная ошибка (опционально)
	}{
		{
			name:     "Valid full address",
			input:    "localhost:8080",
			wantHost: "localhost",
			wantPort: 8080,
			wantErr:  false,
		},
		{
			name:     "Valid IP address",
			input:    "127.0.0.1:9090",
			wantHost: "127.0.0.1",
			wantPort: 9090,
			wantErr:  false,
		},
		{
			name:     "Empty host (valid in Go net)",
			input:    ":80",
			wantHost: "",
			wantPort: 80,
			wantErr:  false,
		},
		{
			name:      "Missing port",
			input:     "localhost",
			wantErr:   true,
			targetErr: ErrNotCorrect,
		},
		{
			name:      "Invalid port (not a number)",
			input:     "localhost:abc",
			wantErr:   true,
			targetErr: ErrNotCorrect,
		},
		{
			name:      "Empty string",
			input:     "",
			wantErr:   true,
			targetErr: ErrNotCorrect,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var a Addr
			err := a.Set(tt.input)

			// Проверка наличия ошибки
			if (err != nil) != tt.wantErr {
				t.Errorf("Set() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Проверка конкретной ошибки, если она ожидается
			if tt.wantErr && tt.targetErr != nil && err != tt.targetErr {
				t.Errorf("Set() error = %v, want targetErr %v", err, tt.targetErr)
			}

			// Если ошибки не ожидалось, проверяем поля структуры
			if !tt.wantErr {
				if a.Host != tt.wantHost {
					t.Errorf("Set() Host = %v, want %v", a.Host, tt.wantHost)
				}
				if a.Port != tt.wantPort {
					t.Errorf("Set() Port = %v, want %v", a.Port, tt.wantPort)
				}
			}
		})
	}
}

// TestAddr_String verifies the string representation of Addr.
func TestAddr_String(t *testing.T) {
	tests := []struct {
		name string
		addr Addr
		want string
	}{
		{
			name: "Localhost",
			addr: Addr{Host: "localhost", Port: 8080},
			want: "localhost:8080",
		},
		{
			name: "Empty host",
			addr: Addr{Host: "", Port: 9000},
			want: ":9000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.addr.String(); got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestAddr_UnmarshalText verifies parsing from byte slice (e.g. JSON/YAML/Env).
func TestAddr_UnmarshalText(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		wantHost string
		wantPort int
		wantErr  bool
	}{
		{
			name:     "Simple bytes",
			input:    []byte("192.168.1.1:5432"),
			wantHost: "192.168.1.1",
			wantPort: 5432,
			wantErr:  false,
		},
		{
			name:     "Quoted bytes (JSON style)",
			input:    []byte("\"localhost:8000\""),
			wantHost: "localhost",
			wantPort: 8000,
			wantErr:  false,
		},
		{
			name:    "Invalid bytes",
			input:   []byte("invalid"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var a Addr
			err := a.UnmarshalText(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalText() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if a.Host != tt.wantHost || a.Port != tt.wantPort {
					t.Errorf("UnmarshalText() = %v:%v, want %v:%v", a.Host, a.Port, tt.wantHost, tt.wantPort)
				}
			}
		})
	}
}

// ExampleAddr_Set demonstrates how to parse an address string.
// This function will appear in godoc as an example.
func ExampleAddr_Set() {
	var addr Addr
	// Simulate parsing a flag or config string
	err := addr.Set("example.com:443")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("Host: %s\n", addr.Host)
	fmt.Printf("Port: %d\n", addr.Port)

	// Output:
	// Host: example.com
	// Port: 443
}

// ExampleAddr_String demonstrates converting Addr back to string.
func ExampleAddr_String() {
	addr := Addr{
		Host: "localhost",
		Port: 6060,
	}
	fmt.Println(addr.String())

	// Output:
	// localhost:6060
}
