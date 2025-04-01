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

func (o *addr) Set(flagValue string) error {
	args := strings.Split(flagValue, ":")
	o.host = args[0]
	o.port, err = strconv.Atoi(args[1])
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

func InitialFlags() *Address {
	newInstance := Address{2, 10, addr{"localhost", 8080}}
	return &newInstance
}

func (a *Address) ParseFlags(server bool) {

	if server {
		flag.Var(&a.addr, "a", "Host and port for connect/create")
	}
	flag.IntVar(&a.ReportInterval, "r", 10, "Send to server interval")
	flag.IntVar(&a.PollInterval, "p", 2, "Refresh metrics interval")
	flag.Parse()
}
