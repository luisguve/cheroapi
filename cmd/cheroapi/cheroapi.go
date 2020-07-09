package main

import (
	"log"

	"github.com/luisguve/cheroapi/internal/pkg/server"
	"github.com/luisguve/cheroapi/internal/pkg/bolt"
	app "github.com/luisguve/cheroapi/internal/app/cheroapi"
)

func main() {
	h, err := bolt.New("db")
	if err != nil {
		log.Fatalf("Could not setup database: %v\n", err)
	}
	srv := server.New(h)
	// Start App.
	a := app.New(srv)
	log.Fatal(a.Run())
}