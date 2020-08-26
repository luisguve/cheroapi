// Package general provides the data type Server, which implements the 
// interface CrudGeneralServer for querying data of every available section.

package general

import (
	"log"
	"fmt"

	"github.com/luisguve/cheroapi/internal/app/general"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	pbUsers "github.com/luisguve/cheroproto-go/userapi"
)

type Section struct {
	Client pbApi.CrudCheropatillaClient
	Id     string
	Name   string
}

type server struct {
	sections map[string]Section
	users    pbUsers.CrudUsersClient
}

func New(sections []Section, usersClient pbUsers.CrudUsersClient) general.Server {
	if len(sections) == 0 {
		log.Fatal("There must be at least one section.")
	}
	if usersClient == nil {
		log.Fatal("Got a nil users client.")
	}
	srv := &server{
		sections: make(map[string]Section),
		users:    usersClient,
	}
	for _, s := range sections {
		if err := s.preventDefault(); err != nil {
			log.Fatal(err)
		}
		srv.sections[s.Id] = s
	}
	return srv
}

func (s Section) preventDefault() error {
	if s.Client == nil {
		return fmt.Errorf("Got a nil section client.")
	}
	if s.Name == "" {
		return fmt.Errorf("Got an empty section name.")
	}
	if s.Id == "" {
		return fmt.Errorf("Got an empty section id.")
	}
	return nil
}
