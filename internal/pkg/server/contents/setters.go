package contents

import (
	"context"
	"errors"

	dbmodel "github.com/luisguve/cheroapi/internal/app/cheroapi"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Update a thread, comment or subcomment
/* func (s *Server) UpdateContent(req *pbApi.UpdateContentRequest, stream pbApi.CrudCheropatilla_UpdateContentServer) error {
	if s.dbHandler == nil {
		return status.Error(codes.Internal, "No database connection")
	}
	var (
		submitter = req.UserId
		notifyUsers []*pbApi.NotifyUsers
		err error
		sendErr error
	)
} */

// Delete a thread, comment or subcomment
func (s *Server) DeleteContent(ctx context.Context, req *pbApi.DeleteContentRequest) (*pbApi.DeleteContentResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	var (
		submitter = req.UserId
		err       error
	)

	switch ctx := req.ContentContext.(type) {
	case *pbApi.DeleteContentRequest_ThreadCtx:
		err = s.dbHandler.DeleteThread(ctx.ThreadCtx, submitter)
	case *pbApi.DeleteContentRequest_CommentCtx:
		err = s.dbHandler.DeleteComment(ctx.CommentCtx, submitter)
	case *pbApi.DeleteContentRequest_SubcommentCtx:
		err = s.dbHandler.DeleteSubcomment(ctx.SubcommentCtx, submitter)
	}
	if err != nil {
		if errors.Is(err, dbmodel.ErrSectionNotFound) ||
			errors.Is(err, dbmodel.ErrThreadNotFound) ||
			errors.Is(err, dbmodel.ErrCommentNotFound) ||
			errors.Is(err, dbmodel.ErrSubcommentNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if errors.Is(err, dbmodel.ErrUserNotAllowed) {
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.DeleteContentResponse{}, nil
}

// Post a thread to create
func (s *Server) CreateThread(ctx context.Context, req *pbApi.CreateThreadRequest) (*pbApi.CreateThreadResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	var (
		submitter = req.UserId
		content   = req.Content
	)
	permalink, err := s.dbHandler.CreateThread(content, submitter)
	if err != nil {
		if errors.Is(err, dbmodel.ErrSectionNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if errors.Is(err, dbmodel.ErrUserNotAllowed) {
			return nil, status.Error(codes.FailedPrecondition, "User already posted today")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.CreateThreadResponse{
		Permalink: permalink,
	}, nil
}

// Request to save thread
func (s *Server) SaveThread(ctx context.Context, req *pbApi.SaveThreadRequest) (*pbApi.SaveThreadResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	var (
		userId = req.UserId
		thread = req.Thread
	)
	err := s.dbHandler.AppendUserWhoSaved(thread, userId)
	if err != nil {
		if errors.Is(err, dbmodel.ErrSectionNotFound) ||
			errors.Is(err, dbmodel.ErrThreadNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.SaveThreadResponse{}, nil
}

// Request to remove thread from user's list of saved threads
func (s *Server) UndoSaveThread(ctx context.Context, req *pbApi.UndoSaveThreadRequest) (*pbApi.UndoSaveThreadResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	var (
		userId = req.UserId
		thread = req.Thread
	)
	err := s.dbHandler.RemoveUserWhoSaved(thread, userId)
	if err != nil {
		if errors.Is(err, dbmodel.ErrSectionNotFound) ||
			errors.Is(err, dbmodel.ErrThreadNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.UndoSaveThreadResponse{}, nil
}
