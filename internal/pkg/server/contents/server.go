// Package server provides the data type Server, which implements the
// interface CrudCheropatillaServer.

package contents

import (
	dbmodel "github.com/luisguve/cheroapi/internal/app/cheroapi"
)

func New(dbh dbmodel.Handler) *Server {
	return &Server{
		dbHandler: dbh,
	}
}

type Server struct {
	dbHandler dbmodel.Handler
}

func (s *Server) QA() (string, error) {
	return s.dbHandler.QA()
}
