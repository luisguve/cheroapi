package userapi

import (
	"fmt"
	"log"
	"net"

	pbApi "github.com/luisguve/cheroproto-go/userapi"
	"google.golang.org/grpc"
)

func New(s pbApi.CrudUsersServer) *App {
	return &App{
		srv: s,
	}
}

type App struct {
	srv pbApi.CrudUsersServer
}

func (a *App) Run(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("Failed to listen: %v\n", err)
	}
	s := grpc.NewServer()

	pbApi.RegisterCrudUsersServer(s, a.srv)

	log.Println("Running")
	return s.Serve(lis)
}
