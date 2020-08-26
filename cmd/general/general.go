package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/BurntSushi/toml"
	app "github.com/luisguve/cheroapi/internal/app/general"
	server "github.com/luisguve/cheroapi/internal/pkg/server/general"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	pbUsers "github.com/luisguve/cheroproto-go/userapi"
	"google.golang.org/grpc"
)

type grpcConfig struct {
	BindAddress string `toml:"bind_address"`
}

type sectionConfig struct {
	BindAddress string `toml:"bind_address"`
	Id          string `toml:"id"`
	Name        string `toml:"name"`
}

type cheroapiConfig struct {
	SrvConf      grpcConfig      `toml:"general_grpc_config"`
	UsersSrvConf grpcConfig      `toml:"users_grpc_config"`
	Sections     []sectionConfig `toml:"sections"`
}

func (c cheroapiConfig) preventDefault() error {
	if c.SrvConf.BindAddress == "" {
		return fmt.Errorf("Missing general service bind address.")
	}
	if c.UsersSrvConf.BindAddress == "" {
		return fmt.Errorf("Missing users service bind address.")
	}
	if len(c.Sections) == 0 {
		return fmt.Errorf("Missing sections config.")
	}
	for _, s := range c.Sections {
		if s.BindAddress == "" {
			return fmt.Errorf("Missing bind address in one or more sections.")
		}
		if s.Id == "" {
			return fmt.Errorf("Missing id in one or more sections.")
		}
		if s.Name == "" {
			return fmt.Errorf("Missing name in one or more sections.")
		}
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

	// Establish connection with section grpc services.
	var sections []server.Section

	for _, s := range config.Sections {
		conn, err = grpc.Dial(s.BindAddress, grpc.WithInsecure())
		if err != nil {
			log.Fatal("Could not setup dial:", err)
		}
		defer conn.Close()

		sectionClient := pbApi.NewCrudCheropatillaClient(conn)
		section := server.Section{
			Client: sectionClient,
			Id:     s.Id,
			Name:   s.Name,
		}
		sections = append(sections, section)
	}

	srv := server.New(sections, usersClient)
	// Start App.
	a := app.New(srv)
	log.Fatal(a.Run(config.SrvConf.BindAddress))
}
