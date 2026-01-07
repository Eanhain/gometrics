// Package addr provides functionality for parsing and handling network addresses
// in "host:port" format. It implements interfaces for standard library integration
// such as flag.Value and encoding.TextUnmarshaler.
package addr

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrNotCorrect is returned when parsing fails due to an invalid address format.
// The expected format is "host:port".
var ErrNotCorrect = errors.New("wrong host:port")

// Addr represents a network endpoint address consisting of a host and a port.
type Addr struct {
	// Host is the hostname or IP address.
	Host string
	// Port is the network port number.
	Port int
}

// UnmarshalText decodes a text representation of an address into the Addr struct.
// It trims surrounding quotes and parses the string using the Set method.
// This method implements the encoding.TextUnmarshaler interface.
func (a *Addr) UnmarshalText(text []byte) error {
	address := string(text)
	address = strings.TrimSuffix(strings.TrimPrefix(address, "\""), "\"")
	return a.Set(address)
}

// String returns the string representation of the address in "host:port" format.
// This method implements the fmt.Stringer interface.
func (a *Addr) String() string {
	return fmt.Sprintf("%s:%d", a.Host, a.Port)
}

// Set parses the provided string flagValue in "host:port" format and updates the Addr fields.
// It returns ErrNotCorrect if the format is invalid or the port is not a number.
// This method implements the flag.Value interface.
func (a *Addr) Set(flagValue string) error {
	args := strings.Split(flagValue, ":")
	if len(args) < 2 {
		return ErrNotCorrect
	}
	port, err := strconv.Atoi(args[1])
	if err != nil {
		return ErrNotCorrect
	}
	a.Host = args[0]
	a.Port = port
	return nil
}

// GetHost returns the stored hostname component of the address.
func (a *Addr) GetHost() string {
	return a.Host
}

// GetPort returns the stored port component of the address.
func (a *Addr) GetPort() int {
	return a.Port
}

// GetAddr returns the full address string in "host:port" format.
// It is an alias for the String method.
func (a *Addr) GetAddr() string {
	return a.String()
}
