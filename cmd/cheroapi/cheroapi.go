package main

import (
	"errors"
	"fmt"
	"log"

	"github.com/joho/godotenv"
	app "github.com/luisguve/cheroapi/internal/app/cheroapi"
	"github.com/luisguve/cheroapi/internal/pkg/bolt"
	"github.com/luisguve/cheroapi/internal/pkg/server"
)

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
	// Get section names mapped to their ids.
	sections, err := siteConfig("C:/cheroshared_files/sections.env")
	if err != nil {
		log.Fatal(err)
	}

	h, err := bolt.New("C:/cheroapi_files/db", sections)
	if err != nil {
		log.Fatalf("Could not setup database: %v\n", err)
	}
	srv := server.New(h)
	// Start App.
	a := app.New(srv)
	log.Fatal(a.Run())
}
