package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/BurntSushi/toml"
	app "github.com/luisguve/cheroapi/internal/app/cheroapi"
	db "github.com/luisguve/cheroapi/internal/pkg/bolt/contents"
	server "github.com/luisguve/cheroapi/internal/pkg/server/contents"
	pbUsers "github.com/luisguve/cheroproto-go/userapi"
	"google.golang.org/grpc"
)

type grpcConfig struct {
	BindAddress string `toml:"bind_address"`
}

type cheroapiConfig struct {
	SectionId    string     `toml:"section_id"`
	SectionName  string     `toml:"section_name"`
	DBdir        string     `toml:"db_dir"`
	SrvConf      grpcConfig `toml:"contents_grpc_config"`
	UsersSrvConf grpcConfig `toml:"users_grpc_config"`
	LogDir       string     `toml:"log_dir"`
	DoQA         bool       `toml:"schedule_qa"`
}

func (c cheroapiConfig) preventDefault() error {
	if c.SectionId == "" {
		return fmt.Errorf("Missing section id.")
	}
	if c.SectionName == "" {
		return fmt.Errorf("Missing section name.")
	}
	if c.DBdir == "" {
		return fmt.Errorf("Missing db dir.")
	}
	if c.LogDir == "" {
		return fmt.Errorf("Missing log dir.")
	}
	if c.SrvConf.BindAddress == "" {
		return fmt.Errorf("Missing contents service bind address.")
	}
	if c.UsersSrvConf.BindAddress == "" {
		return fmt.Errorf("Missing users service bind address.")
	}
	return nil
}

func main() {
	var configFile string
	flag.StringVar(&configFile, "config", "", "Absolute path of .toml config file.")

	flag.Parse()

	if configFile == "" {
		log.Fatal("Absolute path of .toml config file must be set.")
	}

	config := cheroapiConfig{}
	if _, err := toml.DecodeFile(configFile, &config); err != nil {
		log.Fatal(err)
	}

	if err := config.preventDefault(); err != nil {
		log.Fatal(err)
	}

	// Establish connection with users gRPC service.
	conn, err := grpc.Dial(config.UsersSrvConf.BindAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatal("Could not setup dial:", err)
	}
	defer conn.Close()

	// Create users gRPC crud client.
	usersClient := pbUsers.NewCrudUsersClient(conn)

	dbHandler, err := db.New(config.DBdir, config.SectionId, config.SectionName, usersClient)
	if err != nil {
		log.Fatal("Could not setup database:", err)
	}
	srv := server.New(dbHandler)
	// Start App.
	a := app.New(srv, config.LogDir)
	log.Fatal(a.Run(config.SrvConf.BindAddress, config.SectionName, config.DoQA))
}
