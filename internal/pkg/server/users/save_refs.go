package users

import (
	"context"
	"errors"

	dbmodel "github.com/luisguve/cheroapi/internal/app/userapi"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
	pbApi "github.com/luisguve/cheroproto-go/userapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Append thread to list of recent activity of the given user.
func (s *Server) CreateThread(ctx context.Context, req *pbApi.CreateThreadRequest) (*pbApi.CreateThreadResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	err := s.dbHandler.UpdateUser(req.UserId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
		if pbUser.RecentActivity == nil {
			pbUser.RecentActivity = new(pbDataFormat.Activity)
		}
		pbUser.RecentActivity.ThreadsCreated = append(pbUser.RecentActivity.ThreadsCreated, req.Ctx)
		// Update last time created field.
		pbUser.LastTimeCreated = req.PublishDate
		return pbUser
	})
	if err != nil {
		if errors.Is(err, dbmodel.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.CreateThreadResponse{}, nil
}

// Append comment to list of recent activity of user.
func (s *Server) Comment(ctx context.Context, req *pbApi.CommentRequest) (*pbApi.CommentResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	err := s.dbHandler.UpdateUser(req.UserId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
		if pbUser.RecentActivity == nil {
			pbUser.RecentActivity = new(pbDataFormat.Activity)
		}
		pbUser.RecentActivity.Comments = append(pbUser.RecentActivity.Comments, req.Ctx)
		return pbUser
	})
	if err != nil {
		if errors.Is(err, dbmodel.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.CommentResponse{}, nil
}

// Append subcomment to list of recent activity of user.
func (s *Server) Subcomment(ctx context.Context, req *pbApi.SubcommentRequest) (*pbApi.SubcommentResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	err := s.dbHandler.UpdateUser(req.UserId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
		if pbUser.RecentActivity == nil {
			pbUser.RecentActivity = new(pbDataFormat.Activity)
		}
		pbUser.RecentActivity.Subcomments = append(pbUser.RecentActivity.Subcomments, req.Ctx)
		return pbUser
	})
	if err != nil {
		if errors.Is(err, dbmodel.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.SubcommentResponse{}, nil
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
	err := s.dbHandler.UpdateUser(userId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
		var saved bool
		for _, t := range pbUser.SavedThreads {
			if (t.SectionCtx.Id == thread.SectionCtx.Id) && (t.Id == thread.Id) {
				saved = true
				break
			}
		}
		if saved {
			return nil
		}
		pbUser.SavedThreads = append(pbUser.SavedThreads, thread)
		return pbUser
	})
	if err != nil {
		if errors.Is(err, dbmodel.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.SaveThreadResponse{}, nil
}
