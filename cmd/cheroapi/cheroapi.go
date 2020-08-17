package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/joho/godotenv"
	app "github.com/luisguve/cheroapi/internal/app/cheroapi"
	db "github.com/luisguve/cheroapi/internal/pkg/bolt/contents"
	server "github.com/luisguve/cheroapi/internal/pkg/server/contents"
	pbApi "github.com/luisguve/cheroproto-go/userapi"
	"google.golang.org/grpc"
)

type grpcConfig struct {
	BindAddress string `toml:"bind_address"`
}

type cheroapiConfig struct {
	SectionsPath string     `toml:"sections"`
	DBdir        string     `toml:"db_dir"`
	SrvConf      grpcConfig `toml:"grpc_config"`
	UsersSrvConf grpcConfig `toml:"users_grpc_config"`
	LogDir       string     `toml:"log_dir"`
	DoQA         bool       `toml:"schedule_qa"`
}

func siteConfig(file string, vars ...string) (map[string]string, error) {
	config, err := godotenv.Read(file)
	if err != nil {
		return nil, err
	}
	var result = make(map[string]string)
	if len(vars) > 0 {
		for _, key := range vars {
			val, ok := config[key]
			if !ok {
				errMsg := fmt.Sprintf("Missing %s in %s.", key, file)
				return nil, errors.New(errMsg)
			}
			result[key] = val
		}
	} else {
		result = config
	}
	return result, nil
}

func main() {
	gopath, ok := os.LookupEnv("GOPATH")
	if !ok || gopath == "" {
		log.Fatal("GOPATH must be set.")
	}

	configDir := filepath.Join(gopath, "src", "github.com", "luisguve",
		"cheroapi", "cheroapi.toml")

	config := cheroapiConfig{}
	if _, err := toml.DecodeFile(configDir, &config); err != nil {
		log.Fatal(err)
	}

	// Get section names mapped to their ids.
	sections, err := siteConfig(config.SectionsPath)
	if err != nil {
		log.Fatal(err)
	}

	// Establish connection with users gRPC service.
	conn, err := grpc.Dial(config.UsersSrvConf.BindAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatal("Could not setup dial:", err)
	}
	defer conn.Close()

	// Create users gRPC crud client.
	usersClient := pbApi.NewCrudUsersClient(conn)

	h, err := db.New(config.DBdir, sections, usersClient)
	if err != nil {
		log.Fatal("Could not setup database:", err)
	}
	srv := server.New(h)
	// Start App.
	a := app.New(srv, config.LogDir)
	log.Fatal(a.Run(config.SrvConf.BindAddress, config.DoQA))
}
