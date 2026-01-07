// Package clientconfig manages configuration parameters for the metrics client agent.
// It supports configuration via command-line flags and environment variables,
// prioritizing environment variables over default values, and flags over environment variables
// (with specific logic for the 'Key' field).
package clientconfig

import (
	"flag"
	"fmt"

	"gometrics/internal/addr"

	"github.com/caarlos0/env/v6"
)

// ClientConfig holds all configuration settings for the client.
// Fields are tagged for parsing from environment variables using github.com/caarlos0/env.
type ClientConfig struct {
	// ReportInterval is the frequency (in seconds) of sending metrics to the server.
	ReportInterval int `env:"REPORT_INTERVAL" envDefault:"10"`

	// PollInterval is the frequency (in seconds) of updating metrics from runtime app.
	PollInterval int `env:"POLL_INTERVAL" envDefault:"2"`

	// Addr represents the target server address (host:port).
	Addr addr.Addr `env:"ADDRESS" envDefault:"localhost:8080"`

	// Compress defines the compression algorithm (e.g., "gzip").
	Compress string `env:"compress" envDefault:"gzip"`

	// Key is the secret key for signing metrics data (SHA256).
	Key string `env:"KEY" envDefault:""`

	// RateLimit controls the number of concurrent workers for sending metrics.
	RateLimit int `env:"RATE_LIMIT" envDefault:"5"`
}

// GetPort returns the port string formatted with a colon (e.g., ":8080").
func (o *ClientConfig) GetPort() string {
	return fmt.Sprintf(":%d", o.Addr.GetPort())
}

// GetHost returns the hostname part of the address.
func (o *ClientConfig) GetHost() string {
	return o.Addr.GetHost()
}

// InitialFlags creates a new ClientConfig with zero values.
// Note: Default values are actually populated during env.Parse or flag definition.
func InitialFlags() ClientConfig {
	return ClientConfig{
		Addr: addr.Addr{},
	}
}

// ParseFlags reads configuration from environment variables and command-line flags.
//
// Priority logic:
// 1. Default values (defined in struct tags and flag defaults).
// 2. Environment variables (read via env.Parse).
// 3. Command-line flags (override env vars for most fields).
//
// Special case for 'Key': The environment variable 'KEY' takes precedence over the flag '-k'
// if both are present and the env var is not empty (as per original logic).
func (o *ClientConfig) ParseFlags() {
	// 1. Parse Environment variables first to populate defaults or env-set values
	if err := env.Parse(o); err != nil {
		// In a real app, you might want to log this properly or return error
		fmt.Println("ENV var not found or parse error:", err)
	}

	// Capture the key from ENV if it exists, to restore it later if needed
	envKey := o.Key

	// 2. Define Flags.
	// We use current values of 'o' (set by env or defaults) as default values for flags.
	// This allows flags to override env vars generally.
	// Since flag.Parse uses the global flag set, this should ideally be called once.
	if flag.Lookup("r") == nil { // Prevent re-definition in tests if running multiple times in same process
		flag.IntVar(&o.ReportInterval, "r", o.ReportInterval, "Send to server interval")
		flag.IntVar(&o.PollInterval, "p", o.PollInterval, "Refresh metrics interval")
		flag.IntVar(&o.RateLimit, "l", o.RateLimit, "sender counter")
		flag.Var(&o.Addr, "a", "Host and port for connect/create")
		flag.StringVar(&o.Compress, "c", o.Compress, "Send metrics with compression")
		flag.StringVar(&o.Key, "k", o.Key, "Cipher key")
	}

	// 3. Parse Flags
	flag.Parse()

	// 4. Special Logic: Restore Key from Env if it was present.
	// The original code implies that ENV 'KEY' > FLAG 'k'.
	if envKey != "" {
		o.Key = envKey
	}
}

// ParseFlagsFromArgs is a helper for testing that allows passing custom arguments.
// It mimics ParseFlags but works with a custom FlagSet.
func (o *ClientConfig) ParseFlagsFromArgs(args []string) error {
	// Reset to defaults or env
	if err := env.Parse(o); err != nil {
		return err
	}
	envKey := o.Key

	fs := flag.NewFlagSet("test-client", flag.ContinueOnError)
	fs.IntVar(&o.ReportInterval, "r", o.ReportInterval, "Send to server interval")
	fs.IntVar(&o.PollInterval, "p", o.PollInterval, "Refresh metrics interval")
	fs.IntVar(&o.RateLimit, "l", o.RateLimit, "sender counter")
	fs.Var(&o.Addr, "a", "Host and port for connect/create")
	fs.StringVar(&o.Compress, "c", o.Compress, "Send metrics with compression")
	fs.StringVar(&o.Key, "k", o.Key, "Cipher key")

	// Map internal Addr implementation to flag.Value interface explicitly if needed,
	// but since addr.Addr implements it, it works directly.

	if err := fs.Parse(args); err != nil {
		return err
	}

	if envKey != "" {
		o.Key = envKey
	}
	return nil
}
