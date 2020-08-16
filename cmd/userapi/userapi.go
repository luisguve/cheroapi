package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/joho/godotenv"
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

func main() {
	gopath, ok := os.LookupEnv("GOPATH")
	if !ok || gopath == "" {
		log.Fatal("GOPATH must be set.")
	}

	configDir := filepath.Join(gopath, "src", "github.com", "luisguve",
		"cheroapi", "cheroapi.toml")

	cheroConfig := new(cheroapiConfig)
	if _, err := toml.DecodeFile(configDir, cheroConfig); err != nil {
		log.Fatal(err)
	}

	h, err := bolt.New(cheroConfig.DBdir)
	if err != nil {
		log.Fatalf("Could not setup database: %v\n", err)
	}
	srv := server.New(h)
	// Start App.
	a := app.New(srv)
	log.Fatal(a.Run(cheroConfig.SrvConf.BindAddress))
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
