// Package server/users provides the data type Server, which implements the
// interface CrudUsersServer.

package users

import (
	dbmodel "github.com/luisguve/cheroapi/internal/app/userapi"
)

func New(dbh dbmodel.Handler) *Server {
	return &Server{
		dbHandler: dbh,
	}
}

type Server struct {
	dbHandler dbmodel.Handler
}
