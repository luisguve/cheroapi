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

// Remove reference to thread from list of activity of user.
func (s *Server) DeleteThread(ctx context.Context, req *pbApi.DeleteThreadRequest) (*pbApi.DeleteThreadResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	var (
		userId    = req.UserId
		thread    = req.Ctx
		id        = thread.Id
		sectionId = thread.SectionCtx.Id
	)
	err := s.dbHandler.UpdateUser(userId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
		if (pbUser.RecentActivity == nil) && (pbUser.OldActivity == nil) {
			log.Printf("Remove thread %v: user %s has neither recent nor old activity\n", thread, userId)
			return nil
		}
		var found bool
		if pbUser.RecentActivity != nil {
			// Find and remove thread from list of recent activity of user.
			for i, t := range pbUser.RecentActivity.ThreadsCreated {
				if (t.SectionCtx.Id == sectionId) && (t.Id == id) {
					found = true
					last := len(pbUser.RecentActivity.ThreadsCreated) - 1
					pbUser.RecentActivity.ThreadsCreated[i] = pbUser.RecentActivity.ThreadsCreated[last]
					pbUser.RecentActivity.ThreadsCreated = pbUser.RecentActivity.ThreadsCreated[:last]
					break
				}
			}
		}
		if !found {
			if pbUser.OldActivity != nil {
				// Find and remove thread from list of old activity of user.
				for i, t := range pbUser.OldActivity.ThreadsCreated {
					if (t.SectionCtx.Id == sectionId) && (t.Id == id) {
						found = true
						last := len(pbUser.RecentActivity.ThreadsCreated) - 1
						pbUser.RecentActivity.ThreadsCreated[i] = pbUser.RecentActivity.ThreadsCreated[last]
						pbUser.RecentActivity.ThreadsCreated = pbUser.RecentActivity.ThreadsCreated[:last]
						break
					}
				}
			}
		}
		if !found {
			log.Printf("Remove content: could not find thread %v in neither recent nor old activity of user %s\n", thread, userId)
			return nil
		}
		return pbUser
	})
	if err != nil {
		if errors.Is(err, dbmodel.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.DeleteThreadResponse{}, nil
}

// Remove reference to comment from list of activity of user.
func (s *Server) DeleteComment(ctx context.Context, req *pbApi.DeleteCommentRequest) (*pbApi.DeleteCommentResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	var (
		userId    = req.UserId
		comment   = req.Ctx
		id        = comment.Id
		threadId  = comment.ThreadCtx.Id
		sectionId = comment.ThreadCtx.SectionCtx.Id
	)
	err := s.dbHandler.UpdateUser(userId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
		if (pbUser.RecentActivity == nil) && (pbUser.OldActivity == nil) {
			log.Printf("Remove comment %v: user %s has neither recent nor old activity\n", comment, userId)
			return nil
		}
		var found bool
		if pbUser.RecentActivity != nil {
			// Find and remove comment from list of recent activity of user.
			for i, c := range pbUser.RecentActivity.Comments {
				if (c.ThreadCtx.SectionCtx.Id == sectionId) &&
					(c.ThreadCtx.Id == threadId) &&
					(c.Id == id) {
					found = true
					last := len(pbUser.RecentActivity.Comments) - 1
					pbUser.RecentActivity.Comments[i] = pbUser.RecentActivity.Comments[last]
					pbUser.RecentActivity.Comments = pbUser.RecentActivity.Comments[:last]
					break
				}
			}
		}
		if !found {
			if pbUser.OldActivity != nil {
				// Find and remove comment from list of old activity of user.
				for i, c := range pbUser.OldActivity.Comments {
					if (c.ThreadCtx.SectionCtx.Id == sectionId) &&
						(c.ThreadCtx.Id == threadId) &&
						(c.Id == id) {
						found = true
						last := len(pbUser.RecentActivity.Comments) - 1
						pbUser.RecentActivity.Comments[i] = pbUser.RecentActivity.Comments[last]
						pbUser.RecentActivity.Comments = pbUser.RecentActivity.Comments[:last]
						break
					}
				}
			}
		}
		if !found {
			log.Printf("Remove content: could not find comment %v in neither recent nor old activity of user %s\n", comment, userId)
			return nil
		}
		return pbUser
	})
	if err != nil {
		if errors.Is(err, dbmodel.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.DeleteCommentResponse{}, nil
}

// Remove reference to subcomment from list of activity of user.
func (s *Server) DeleteSubcomment(ctx context.Context, req *pbApi.DeleteSubcommentRequest) (*pbApi.DeleteSubcommentResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	var (
		userId     = req.UserId
		subcomment = req.Ctx
		id         = subcomment.Id
		commentId  = subcomment.CommentCtx.Id
		threadId   = subcomment.CommentCtx.ThreadCtx.Id
		sectionId  = subcomment.CommentCtx.ThreadCtx.SectionCtx.Id
	)
	err := s.dbHandler.UpdateUser(userId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
		if (pbUser.RecentActivity == nil) && (pbUser.OldActivity == nil) {
			log.Printf("Remove subcomment %v: user %s has neither recent nor old activity\n", subcomment, userId)
			return nil
		}
		var found bool
		if pbUser.RecentActivity != nil {
			// Find and remove subcomment from list of recent activity of user.
			for i, s := range pbUser.RecentActivity.Subcomments {
				if (s.CommentCtx.ThreadCtx.SectionCtx.Id == sectionId) &&
					(s.CommentCtx.ThreadCtx.Id == threadId) &&
					(s.CommentCtx.Id == commentId) &&
					(s.Id == id) {
					found = true
					last := len(pbUser.RecentActivity.Subcomments) - 1
					pbUser.RecentActivity.Subcomments[i] = pbUser.RecentActivity.Subcomments[last]
					pbUser.RecentActivity.Subcomments = pbUser.RecentActivity.Subcomments[:last]
					break
				}
			}
		}
		if !found {
			if pbUser.OldActivity != nil {
				// Find and remove subcomment from list of old activity of user.
				for i, s := range pbUser.OldActivity.Subcomments {
					if (s.CommentCtx.ThreadCtx.SectionCtx.Id == sectionId) &&
						(s.CommentCtx.ThreadCtx.Id == threadId) &&
						(s.CommentCtx.Id == commentId) &&
						(s.Id == id) {
						found = true
						last := len(pbUser.RecentActivity.Subcomments) - 1
						pbUser.RecentActivity.Subcomments[i] = pbUser.RecentActivity.Subcomments[last]
						pbUser.RecentActivity.Subcomments = pbUser.RecentActivity.Subcomments[:last]
						break
					}
				}
			}
		}
		if !found {
			log.Printf("Remove content: could not find subcomment %v in neither recent nor old activity of user %s\n", subcomment, userId)
			return nil
		}
		return pbUser
	})
	if err != nil {
		if errors.Is(err, dbmodel.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.DeleteSubcommentResponse{}, nil
}

// Remove reference to thread from list of saved threads of user.
func (s *Server) RemoveSaved(ctx context.Context, req *pbApi.RemoveSavedRequest) (*pbApi.RemoveSavedResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	var (
		id        = req.Ctx.Id
		sectionId = req.Ctx.SectionCtx.Id
	)
	err := s.dbHandler.UpdateUser(req.UserId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
		// Find and remove saved thread.
		var found bool
		for idx, t := range pbUser.SavedThreads {
			if (t.SectionCtx.Id == sectionId) && (t.Id == id) {
				found = true
				last := len(pbUser.SavedThreads) - 1
				pbUser.SavedThreads[idx] = pbUser.SavedThreads[last]
				pbUser.SavedThreads = pbUser.SavedThreads[:last]
				break
			}
		}
		if !found {
			return nil
		}
		return pbUser
	})
	if err != nil {
		if errors.Is(err, dbmodel.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.RemoveSavedResponse{}, nil
}
