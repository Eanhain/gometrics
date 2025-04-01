package flags

import (
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"
)

var ErrNotCorrect = errors.New("неправильно введен host:port")

var err error

type addr struct {
	host string
	port int
}

type Address struct {
	ReportInterval int
	PollInterval   int
	addr           addr
}

func (a *addr) String() string {
	return fmt.Sprintf("%s:%v", a.host, a.port)
}

func (a *addr) Set(flagValue string) error {
	args := strings.Split(flagValue, ":")
	a.host = args[0]
	if len(args) == 0 || len(args) > 2 {
		return ErrNotCorrect
	}
	a.port, err = strconv.Atoi(args[1])
	if err != nil {
		return ErrNotCorrect
	}
	return nil
}

func (o *Address) GetAddr() *addr {
	return &o.addr
}

func (o *Address) GetPort() string {
	return fmt.Sprintf(":%v", o.addr.port)
}

func (o *Address) GetHost() string {
	return o.addr.host

}

func InitialFlags() Address {
	newInstance := Address{2, 10, addr{"localhost", 8080}}
	return newInstance
}

func (o *Address) ParseFlags(server bool) {

	if !server {
		flag.IntVar(&o.ReportInterval, "r", 10, "Send to server interval")
		flag.IntVar(&o.PollInterval, "p", 2, "Refresh metrics interval")
	}
	flag.Var(&o.addr, "a", "Host and port for connect/create")
	flag.Parse()
}
