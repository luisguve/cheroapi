package server

import (
	"context"
	"errors"

	dbmodel "github.com/luisguve/cheroapi/internal/app/cheroapi"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
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
		section   = req.SectionCtx
		content   = req.Content
	)
	permalink, err := s.dbHandler.CreateThread(content, section, submitter)
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

// Update a user's basic data
func (s *Server) UpdateBasicUserData(ctx context.Context, req *pbApi.UpdateBasicUserDataRequest) (*pbApi.UpdateBasicUserDataResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	if req.PicUrl != "" || req.Alias != "" || req.Description != "" || req.Username != "" {
		_, err := s.dbHandler.User(req.UserId)
		if err != nil {
			if errors.Is(err, dbmodel.ErrUserNotFound) {
				return nil, status.Error(codes.NotFound, err.Error())
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
		if req.Username != "" {
			err = s.dbHandler.MapUsername(req.Username, req.UserId)
			if err != nil {
				if errors.Is(err, dbmodel.ErrUsernameAlreadyExists) {
					return nil, status.Error(codes.AlreadyExists, "Username already taken")
				}
				return nil, status.Error(codes.Internal, err.Error())
			}
		}
		err = s.dbHandler.UpdateUser(req.UserId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
			if req.Description != "" {
				pbUser.BasicUserData.About = req.Description
			}
			if req.PicUrl != "" {
				pbUser.BasicUserData.PicUrl = req.PicUrl
			}
			if req.Alias != "" {
				pbUser.BasicUserData.Alias = req.Alias
			}
			if req.Username != "" {
				pbUser.BasicUserData.Username = req.Username
			}
			return pbUser
		})
		if err != nil {
			if errors.Is(err, dbmodel.ErrUserNotFound) {
				return nil, status.Error(codes.NotFound, err.Error())
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	return &pbApi.UpdateBasicUserDataResponse{}, nil
}

// Mark unread notifications as read
func (s *Server) MarkAllAsRead(ctx context.Context, req *pbApi.ReadNotifsRequest) (*pbApi.ReadNotifsResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	err := s.dbHandler.UpdateUser(req.UserId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
		// Append read notifs to list of unread notifs to keep unread as the
		// first notifs, then set the resulting list of notifs to read notifs
		// and nil to unread notifs.
		pbUser.UnreadNotifs = append(pbUser.UnreadNotifs, pbUser.ReadNotifs...)
		pbUser.ReadNotifs = pbUser.UnreadNotifs
		pbUser.UnreadNotifs = nil
		return pbUser
	})
	if err != nil {
		if errors.Is(err, dbmodel.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.ReadNotifsResponse{}, nil
}

// Clear all the notifications
func (s *Server) ClearNotifs(ctx context.Context, req *pbApi.ClearNotifsRequest) (*pbApi.ClearNotifsResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	err := s.dbHandler.UpdateUser(req.UserId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
		pbUser.ReadNotifs = nil
		pbUser.UnreadNotifs = nil
		return pbUser
	})
	if err != nil {
		if errors.Is(err, dbmodel.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.ClearNotifsResponse{}, nil
}

// Follow a user
func (s *Server) FollowUser(ctx context.Context, req *pbApi.FollowUserRequest) (*pbApi.FollowUserResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	var (
		followingId string
		followerId  = req.UserId
	)

	followingIdB, err := s.dbHandler.FindUserIdByUsername(req.UserToFollow)
	if err != nil {
		if errors.Is(err, dbmodel.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	followingId = string(followingIdB)

	// A user cannot follow itself. Check whether that's the case.
	if followingId == followerId {
		return nil, status.Error(codes.InvalidArgument, "A user cannot follow itself")
	}

	// Update both users concurrently.
	var (
		numGR int
		done = make(chan error)
		quit = make(chan struct{})
	)
	// Data of new follower.
	numGR++
	go func() {
		err := s.dbHandler.UpdateUser(followerId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
			// Check whether the user is already following the other user.
			following, _ := inSlice(pbUser.FollowingIds, followingId)
			if following {
				return nil
			}
			// Update users' data.
			pbUser.FollowingIds = append(pbUser.FollowingIds, followingId)
			return pbUser
		})
		select {
		case done<- err:
		case <-quit:
		}
	}()
	// Data of user following.
	numGR++
	go func() {
		err := s.dbHandler.UpdateUser(followingId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
			// Check whether the user is already a follower of the other user.
			follower, _ := inSlice(pbUser.FollowersIds, followerId)
			if follower {
				return nil
			}
			// Update users' data.
			pbUser.FollowersIds = append(pbUser.FollowersIds, followerId)
			return pbUser
		})
		select {
		case done<- err:
		case <-quit:
		}
	}()

	// Check for errors. If there was an error, it will close the channel quit,
	// terminating any go-routine hung on the statement "case done<- err:", and
	// return the error to the client.
	for i := 0; i < numGR; i++ {
		err = <-done
		if err != nil {
			close(quit)
			if errors.Is(err, dbmodel.ErrUserNotFound) {
				return nil, status.Error(codes.NotFound, err.Error())
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	return &pbApi.FollowUserResponse{}, nil
}

// Unfollow a user
func (s *Server) UnfollowUser(ctx context.Context, req *pbApi.UnfollowUserRequest) (*pbApi.UnfollowUserResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	var (
		followingId string
		followerId  = req.UserId
	)

	followingIdB, err := s.dbHandler.FindUserIdByUsername(req.UserToUnfollow)
	if err != nil {
		if errors.Is(err, dbmodel.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	followingId = string(followingIdB)

	// A user cannot unfollow itself. Check whether that's the case.
	if followingId == followerId {
		return nil, status.Error(codes.InvalidArgument, "A user cannot unfollow itself")
	}

	// Update both users concurrently.
	var (
		numGR int
		done = make(chan error)
		quit = make(chan struct{})
	)
	// Data of former follower.
	numGR++
	go func() {
		err := s.dbHandler.UpdateUser(followerId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
			following, idx := inSlice(pbUser.FollowingIds, followingId)
			if !following {
				return nil
			}
			last := len(pbUser.FollowingIds) - 1
			pbUser.FollowingIds[idx] = pbUser.FollowingIds[last]
			pbUser.FollowingIds = pbUser.FollowingIds[:last]
			return pbUser
		})
		select {
		case done<- err:
		case <-quit:
		}
	}()
	// Data of former following.
	numGR++
	go func() {
		err := s.dbHandler.UpdateUser(followingId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
			follower, idx := inSlice(pbUser.FollowersIds, followerId)
			if !follower {
				return nil
			}
			last := len(pbUser.FollowersIds) - 1
			pbUser.FollowersIds[idx] = pbUser.FollowersIds[last]
			pbUser.FollowersIds = pbUser.FollowersIds[:last]
			return pbUser
		})
		select {
		case done<- err:
		case <-quit:
		}
	}()
	// Check for errors. If there was an error, it will close the channel quit,
	// terminating any go-routine hung on the statement "case done<- err:", and
	// return the error to the client.
	for i := 0; i < numGR; i++ {
		err = <-done
		if err != nil {
			close(quit)
			if errors.Is(err, dbmodel.ErrUserNotFound) {
				return nil, status.Error(codes.NotFound, err.Error())
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	return &pbApi.UnfollowUserResponse{}, nil
}

// Request to save thread
func (s *Server) SaveThread(ctx context.Context, req *pbApi.SaveThreadRequest) (*pbApi.SaveThreadResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	var (
		userId = req.UserId
		thread = req.Thread
		getErr error
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
		if _, getErr = s.dbHandler.GetThreadContent(thread); getErr != nil {
			return nil
		}
		if getErr = s.dbHandler.AppendUserWhoSaved(thread, userId); getErr != nil {
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
	if getErr != nil {
		if errors.Is(getErr, dbmodel.ErrSectionNotFound) ||
			errors.Is(getErr, dbmodel.ErrThreadNotFound) {
			return nil, status.Error(codes.NotFound, getErr.Error())
		}
		return nil, status.Error(codes.Internal, getErr.Error())
	}
	return &pbApi.SaveThreadResponse{}, nil
}

// Request to remove thread from user's list of saved threads
func (s *Server) UndoSaveThread(ctx context.Context, req *pbApi.UndoSaveThreadRequest) (*pbApi.UndoSaveThreadResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	var (
		userId    = req.UserId
		id        = req.Thread.Id
		sectionId = req.Thread.SectionCtx.Id
		setErr    error
	)
	err := s.dbHandler.UpdateUser(userId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
		// Find and remove reference to the thread.
		var found bool
		for idx, t := range pbUser.SavedThreads {
			if (t.SectionCtx.Id == sectionId) && (t.Id == id) {
				setErr = s.dbHandler.RemoveUserWhoSaved(req.Thread, userId)
				if setErr != nil {
					return nil
				}
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
	if setErr != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.UndoSaveThreadResponse{}, nil
}
