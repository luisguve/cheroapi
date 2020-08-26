package general

import (
	"fmt"
	"log"
	"net"

	pbGeneral "github.com/luisguve/cheroproto-go/cheroapi"
	"google.golang.org/grpc"
)

type Server interface {
	pbGeneral.CrudGeneralServer
}

type App struct {
	srv Server
}

func New(s Server) *App {
	return &App{
		srv: s,
	}
}

func (a *App) Run(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("Failed to listen: %v\n", err)
	}
	s := grpc.NewServer()

	pbGeneral.RegisterCrudGeneralServer(s, a.srv)

	log.Println("Running")
	return s.Serve(lis)
}
