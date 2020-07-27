package main

import (
	"log"

	app "github.com/luisguve/cheroapi/internal/app/cheroapi"
	"github.com/luisguve/cheroapi/internal/pkg/bolt"
	"github.com/luisguve/cheroapi/internal/pkg/server"
)

func main() {
	h, err := bolt.New("C:/cheroapi_files/db")
	if err != nil {
		log.Fatalf("Could not setup database: %v\n", err)
	}
	srv := server.New(h)
	// Start App.
	a := app.New(srv)
	log.Fatal(a.Run())
}
