package server

import(
	"log"
	"errors"

	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
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
		return status.Error(codes.Internal, "No database connection")
	}
	var (
		submitter = req.UserId
		err error
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
		if errors.Is(err, ErrSectionNotFound) ||
			errors.Is(err, ErrThreadNotFound) ||
			errors.Is(err, ErrCommentNotFound) ||
			errors.Is(err, ErrSubcommentNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if errors.Is(err, ErrUserNotAllowed) {
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.DeleteContentResponse{}, nil
}

// Post a thread to create
func (s *Server) CreateThread(ctx context.Context, req *pbApi.CreateThreadRequest) (*pbApi.CreateThreadResponse, error) {
	if s.dbHandler == nil {
		return status.Error(codes.Internal, "No database connection")
	}
	var (
		submitter = req.UserId
		section = req.SectionCtx
		content = req.Content
	)
	canPost, err := s.dbHandler.CheckUserCanPost(submitter)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if !canPost {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	
	permalink, err := s.dbHandler.CreateThread(content, section, submitter)
	if err != nil {
		if errors.Is(err, ErrSectionNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.CreateThreadResponse{
		Permalink: permalink,
	}, nil
}

// Update a user's basic data
func (s *Server) UpdateBasicUserData(ctx context.Context, 
	req *pbApi.UpdateBasicUserDataRequest) (*pbApi.UpdateBasicUserDataResponse, error) {
	
}

// Mark unread notifications as read
func (s *Server) MarkAllAsRead(ctx context.Context,
	req *pbApi.ReadNotifsRequest) (*pbApi.ReadNotifsResponse, error) {
	
}

// Clear all the notifications
func (s *Server) ClearNotifs(ctx context.Context,
	req *pbApi.ClearNotifsRequest) (*pbApi.ClearNotifsResponse, error) {
	
}

// Follow a user
func (s *Server) FollowUser(ctx context.Context,
	req *pbApi.FollowUserRequest) (*pbApi.FollowUserResponse, error) {
	
}

// Unfollow a user
func (s *Server) UnfollowUser(ctx context.Context,
	req *pbApi.UnfollowUserRequest) (*pbApi.UnfollowUserResponse, error) {
	
}

// Request to save thread
func (s *Server) SaveThread(ctx context.Context,
	req *pbApi.SaveThreadRequest) (*pbApi.SaveThreadResponse, error) {
	
}

// Request to remove thread from user's list of saved threads
func (s *Server) UnsaveThread(ctx context.Context,
	req *pbApi.UnsaveThreadRequest) (*pbApi.UnsaveThreadResponse, error) {
	
}
