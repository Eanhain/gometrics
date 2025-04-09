package addr

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var ErrNotCorrect = errors.New("wrong host:port")

type Interface interface {
	UnmarshalText([]byte) error
	String() string
	Set(string) error
	GetHost() string
	GetPort() int
}

type Addr struct {
	Host string
	Port int
}

func (a *Addr) UnmarshalText(text []byte) error {
	address := string(text)
	address = strings.TrimSuffix(strings.TrimPrefix(address, "\""), "\"")
	return a.Set(address)
}

func (a *Addr) String() string {
	return fmt.Sprintf("%s:%d", a.Host, a.Port)
}

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

func (a *Addr) GetHost() string {
	return a.Host
}

func (a *Addr) GetPort() int {
	return a.Port
}
