package server

import (
	"context"
	"errors"
	"log"

	dbmodel "github.com/luisguve/cheroapi/internal/app/cheroapi"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Get a single thread
func (s *Server) GetThread(ctx context.Context, req *pbApi.GetThreadRequest) (*pbApi.ContentData, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	contentRule, err := s.dbHandler.GetThread(req.Thread)
	if err != nil {
		if errors.Is(err, dbmodel.ErrSectionNotFound) ||
			errors.Is(err, dbmodel.ErrThreadNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return contentRule.Data, nil
}

// Get a comment's comments
func (s *Server) GetSubcomments(req *pbApi.GetSubcommentsRequest, stream pbApi.CrudCheropatilla_GetSubcommentsServer) error {
	if s.dbHandler == nil {
		return status.Error(codes.Internal, "No database connection")
	}
	var (
		err          error
		sendErr      error
		ctx          = req.CommentCtx
		offset       = int(req.Offset)
		contentRules []*pbApi.ContentRule
	)
	contentRules, err = s.dbHandler.GetSubcomments(ctx, offset)
	if err != nil {
		if errors.Is(err, dbmodel.ErrSectionNotFound) ||
			errors.Is(err, dbmodel.ErrThreadNotFound) ||
			errors.Is(err, dbmodel.ErrCommentNotFound) {
			return status.Error(codes.NotFound, err.Error())
		}
		if errors.Is(err, dbmodel.ErrOffsetOutOfRange) {
			return status.Error(codes.OutOfRange, err.Error())
		}
		return status.Error(codes.Internal, err.Error())
	}
	for _, contentRule := range contentRules {
		if sendErr = stream.Send(contentRule); err != nil {
			log.Printf("Could not send Content Rule: %v\n", sendErr)
			return status.Error(codes.Internal, sendErr.Error())
		}
	}
	return nil
}
