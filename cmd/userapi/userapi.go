package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/BurntSushi/toml"
	app "github.com/luisguve/cheroapi/internal/app/userapi"
	bolt "github.com/luisguve/cheroapi/internal/pkg/bolt/users"
	server "github.com/luisguve/cheroapi/internal/pkg/server/users"
)

type grpcConfig struct {
	BindAddress string `toml:"bind_address"`
}

type cheroapiConfig struct {
	DBdir   string     `toml:"db_dir"`
	SrvConf grpcConfig `toml:"users_grpc_config"`
}

func (c cheroapiConfig) preventDefault() error {
	if c.DBdir == "" {
		return fmt.Errorf("Missing db dir.")
	}
	if c.SrvConf.BindAddress == "" {
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

	dbHandler, err := bolt.New(config.DBdir)
	if err != nil {
		log.Fatalf("Could not setup database: %v\n", err)
	}
	srv := server.New(dbHandler)
	// Start App.
	a := app.New(srv)
	log.Fatal(a.Run(config.SrvConf.BindAddress))
}
