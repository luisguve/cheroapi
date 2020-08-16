package users

import (
	"context"
	"errors"
	"log"

	dbmodel "github.com/luisguve/cheroapi/internal/app/userapi"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
	pbApi "github.com/luisguve/cheroproto-go/userapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Move thread from recent activity of user to list of old activity.
func (s *Server) OldThread(ctx context.Context, req *pbApi.OldThreadRequest) (*pbApi.OldThreadResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	var (
		userId    = req.UserId
		id        = req.Ctx.Id
		sectionId = req.Ctx.SectionCtx.Id
		found     bool
	)
	err := s.dbHandler.UpdateUser(userId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
		if pbUser.RecentActivity != nil {
			// Find and copy thread from recent activity to old activity of
			// the user, then remove it from recent activity.
			for i, t := range pbUser.RecentActivity.ThreadsCreated {
				if (t.SectionCtx.Id == sectionId) && (t.Id == id) {
					found = true
					if pbUser.OldActivity == nil {
						pbUser.OldActivity = new(pbDataFormat.Activity)
					}
					// Append to old activity.
					tc := pbUser.OldActivity.ThreadsCreated
					pbUser.OldActivity.ThreadsCreated = append(tc, t)
					// Remove from recent activity.
					last := len(pbUser.RecentActivity.ThreadsCreated) - 1
					pbUser.RecentActivity.ThreadsCreated[i] = pbUser.RecentActivity.ThreadsCreated[last]
					pbUser.RecentActivity.ThreadsCreated = pbUser.RecentActivity.ThreadsCreated[:last]
					break
				}
			}
			if !found {
				log.Printf("Thread %v is not in recent activity of user %s.\n", req.Ctx, userId)
				return nil
			}
		}
		return pbUser
	})
	if err != nil {
		if errors.Is(err, dbmodel.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.OldThreadResponse{}, nil
}

// Move comment from recent activity of user to list of old activity.
func (s *Server) OldComment(ctx context.Context, req *pbApi.OldCommentRequest) (*pbApi.OldCommentResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	var (
		userId    = req.UserId
		id        = req.Ctx.Id
		threadId  = req.Ctx.ThreadCtx.Id
		sectionId = req.Ctx.ThreadCtx.SectionCtx.Id
		found     bool
	)
	err := s.dbHandler.UpdateUser(userId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
		if pbUser.RecentActivity != nil {
			// Find and copy comment from recent activity to old activity of
			// the user, then remove it from recent activity.
			for i, c := range pbUser.RecentActivity.Comments {
				if (c.ThreadCtx.SectionCtx.Id == sectionId) &&
					(c.ThreadCtx.Id == threadId) &&
					(c.Id == id) {
					found = true
					if pbUser.OldActivity == nil {
						pbUser.OldActivity = new(pbDataFormat.Activity)
					}
					// Append to old activity.
					comments := pbUser.OldActivity.Comments
					pbUser.OldActivity.Comments = append(comments, c)
					// Remove from recent activity.
					last := len(pbUser.RecentActivity.Comments) - 1
					pbUser.RecentActivity.Comments[i] = pbUser.RecentActivity.Comments[last]
					pbUser.RecentActivity.Comments = pbUser.RecentActivity.Comments[:last]
					break
				}
			}
			if !found {
				log.Printf("Comment %v is not in recent activity of user %s.\n", req.Ctx, userId)
				return nil
			}
		}
		return pbUser
	})
	if err != nil {
		if errors.Is(err, dbmodel.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.OldCommentResponse{}, nil
}

// Move subcomment from recent activity of user to list of old activity.
func (s *Server) OldSubcomment(ctx context.Context, req *pbApi.OldSubcommentRequest) (*pbApi.OldSubcommentResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	var (
		userId    = req.UserId
		id        = req.Ctx.Id
		commentId = req.Ctx.CommentCtx.Id
		threadId  = req.Ctx.CommentCtx.ThreadCtx.Id
		sectionId = req.Ctx.CommentCtx.ThreadCtx.SectionCtx.Id
		found     bool
	)
	err := s.dbHandler.UpdateUser(userId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
		if pbUser.RecentActivity != nil {
			// Find and copy thread from recent activity to old activity of
			// the user, then remove it from recent activity.
			for i, s := range pbUser.RecentActivity.Subcomments {
				if (s.CommentCtx.ThreadCtx.SectionCtx.Id == sectionId) &&
					(s.CommentCtx.ThreadCtx.Id == threadId) &&
					(s.CommentCtx.Id == commentId) &&
					(s.Id == id) {
					found = true
					if pbUser.OldActivity == nil {
						pbUser.OldActivity = new(pbDataFormat.Activity)
					}
					// Append to old activity.
					subcomments := pbUser.OldActivity.Subcomments
					pbUser.OldActivity.Subcomments = append(subcomments, s)
					// Remove from recent activity.
					last := len(pbUser.RecentActivity.Subcomments) - 1
					pbUser.RecentActivity.Subcomments[i] = pbUser.RecentActivity.Subcomments[last]
					pbUser.RecentActivity.Subcomments = pbUser.RecentActivity.Subcomments[:last]
					break
				}
			}
			if !found {
				log.Printf("Subcomment %v is not in recent activity of user %s.\n", req.Ctx, userId)
				return nil
			}
		}
		return pbUser
	})
	if err != nil {
		if errors.Is(err, dbmodel.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.OldSubcommentResponse{}, nil
}
