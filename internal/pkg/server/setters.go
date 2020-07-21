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
	pbUser, err := s.dbHandler.User(req.UserId)
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
		pbUser.BasicUserData.Username = req.Username
	}
	if req.Description != "" {
		pbUser.BasicUserData.About = req.Description
	}
	if req.PicUrl != "" {
		pbUser.BasicUserData.PicUrl = req.PicUrl
	}
	if req.Alias != "" {
		pbUser.BasicUserData.Alias = req.Alias
	}
	err = s.dbHandler.UpdateUser(pbUser, req.UserId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.UpdateBasicUserDataResponse{}, nil
}

// Mark unread notifications as read
func (s *Server) MarkAllAsRead(ctx context.Context, req *pbApi.ReadNotifsRequest) (*pbApi.ReadNotifsResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	pbUser, err := s.dbHandler.User(req.UserId)
	if err != nil {
		if errors.Is(err, dbmodel.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	// Append read notifs to list of unread notifs to keep unread as the
	// first notifs, then set the resulting list of notifs to read notifs
	// and nil to unread notifs.
	pbUser.UnreadNotifs = append(pbUser.UnreadNotifs, pbUser.ReadNotifs...)
	pbUser.ReadNotifs = pbUser.UnreadNotifs
	pbUser.UnreadNotifs = nil

	err = s.dbHandler.UpdateUser(pbUser, req.UserId)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.ReadNotifsResponse{}, nil
}

// Clear all the notifications
func (s *Server) ClearNotifs(ctx context.Context, req *pbApi.ClearNotifsRequest) (*pbApi.ClearNotifsResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	pbUser, err := s.dbHandler.User(req.UserId)
	if err != nil {
		if errors.Is(err, dbmodel.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	pbUser.ReadNotifs = nil
	pbUser.UnreadNotifs = nil
	err = s.dbHandler.UpdateUser(pbUser, req.UserId)
	if err != nil {
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
		pbFollower  *pbDataFormat.User
		pbFollowing *pbDataFormat.User
		done        = make(chan error)
		quit        = make(chan error)
	)
	// Get both users concurrently.
	go func() {
		var err error
		pbFollower, err = s.dbHandler.User(followerId)
		select {
		case done <- err:
		case <-quit:
		}
	}()
	go func() {
		followingIdB, err := s.dbHandler.FindUserIdByUsername(req.UserToFollow)
		if err == nil {
			followingId = string(followingIdB)
			pbFollowing, err = s.dbHandler.User(followingId)
		}
		select {
		case done <- err:
		case <-quit:
		}
	}()
	var err error
	// Check for errors. If there was an error, it will close the channel quit,
	// terminating any go-routine hung on the statement "case done<- err:", and
	// return the error to the client.
	for i := 0; i < 2; i++ {
		err = <-done
		if err != nil {
			close(quit)
			if errors.Is(err, dbmodel.ErrUserNotFound) {
				return nil, status.Error(codes.NotFound, err.Error())
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	// A user cannot follow itself. Check whether that's the case.
	if followingId == followerId {
		return nil, status.Error(codes.InvalidArgument, "A user cannot follow itself")
	}
	// Save both users concurrently.
	var numGR int
	// Check whether the user is already following the other user.
	following, _ := inSlice(pbFollower.FollowingIds, followingId)
	if !following {
		// Update users' data.
		pbFollower.FollowingIds = append(pbFollower.FollowingIds, followingId)
		numGR++
		go func() {
			err := s.dbHandler.UpdateUser(pbFollower, followerId)
			select {
			case done <- err:
			case <-quit:
			}
		}()
	}
	// Check whether the user is already a follower of the other user.
	follower, _ := inSlice(pbFollowing.FollowersIds, followerId)
	if !follower {
		numGR++
		// Update users' data.
		pbFollowing.FollowersIds = append(pbFollowing.FollowersIds, followerId)
		go func() {
			err := s.dbHandler.UpdateUser(pbFollowing, followingId)
			select {
			case done <- err:
			case <-quit:
			}
		}()
	}
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
		// user to stop following
		ufId string
		pbUf *pbDataFormat.User
		// user unfollowing id
		fId  = req.UserId
		pbF  *pbDataFormat.User
		done = make(chan error)
		quit = make(chan error)
	)
	// Get both users concurrently.
	go func() {
		var err error
		pbF, err = s.dbHandler.User(fId)
		select {
		case done <- err:
		case <-quit:
		}
	}()
	go func() {
		ufIdB, err := s.dbHandler.FindUserIdByUsername(req.UserToUnfollow)
		if err == nil {
			ufId = string(ufIdB)
			pbUf, err = s.dbHandler.User(ufId)
		}
		select {
		case done <- err:
		case <-quit:
		}
	}()
	var err error
	// Check for errors. If there was an error, it will close the channel quit,
	// terminating any go-routine hung on the statement "case done<- err:", and
	// return the error to the client.
	for i := 0; i < 2; i++ {
		err = <-done
		if err != nil {
			close(quit)
			if errors.Is(err, dbmodel.ErrUserNotFound) {
				return nil, status.Error(codes.NotFound, err.Error())
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	// A user cannot unfollow itself. Check whether that's the case.
	if ufId == fId {
		return nil, status.Error(codes.InvalidArgument, "A user cannot unfollow itself")
	}

	// Update users data and save both users concurrently.
	var numGR int
	following, idx := inSlice(pbF.FollowingIds, ufId)
	if following {
		numGR++
		last := len(pbF.FollowingIds) - 1
		pbF.FollowingIds[idx] = pbF.FollowingIds[last]
		pbF.FollowingIds = pbF.FollowingIds[:last]
		go func() {
			err := s.dbHandler.UpdateUser(pbF, fId)
			select {
			case done <- err:
			case <-quit:
			}
		}()
	}
	
	follower, idx := inSlice(pbUf.FollowersIds, fId)
	if follower {
		numGR++
		last := len(pbUf.FollowersIds) - 1
		pbUf.FollowersIds[idx] = pbUf.FollowersIds[last]
		pbUf.FollowersIds = pbUf.FollowersIds[:last]
		go func() {
			err := s.dbHandler.UpdateUser(pbUf, ufId)
			select {
			case done <- err:
			case <-quit:
			}
		}()
	}
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
	)
	pbUser, err := s.dbHandler.User(userId)
	if err != nil {
		if errors.Is(err, dbmodel.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	var saved bool
	for _, t := range pbUser.SavedThreads {
		if (t.SectionCtx.Id == thread.SectionCtx.Id) && (t.Id == thread.Id) {
			saved = true
			break
		}
	}
	if !saved {
		_, err := s.dbHandler.GetThreadContent(thread)
		if err != nil {
			if (errors.Is(err, dbmodel.ErrSectionNotFound)) ||
				(errors.Is(err, dbmodel.ErrThreadNotFound)) {
				return nil, status.Error(codes.NotFound, err.Error())
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
		pbUser.SavedThreads = append(pbUser.SavedThreads, thread)
		err = s.dbHandler.UpdateUser(pbUser, userId)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if err = s.dbHandler.AppendUserWhoSaved(thread, userId); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
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
	)
	pbUser, err := s.dbHandler.User(userId)
	if err != nil {
		if errors.Is(err, dbmodel.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	// Find and remove reference to the thread.
	for idx, t := range pbUser.SavedThreads {
		if (t.SectionCtx.Id == sectionId) && (t.Id == id) {
			last := len(pbUser.SavedThreads) - 1
			pbUser.SavedThreads[idx] = pbUser.SavedThreads[last]
			pbUser.SavedThreads = pbUser.SavedThreads[:last]
			err = s.dbHandler.UpdateUser(pbUser, userId)
			if err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			if err = s.dbHandler.RemoveUserWhoSaved(req.Thread, userId); err != nil {
				return nil, status.Error(codes.Internal, err.Error())
			}
			break
		}
	}
	return &pbApi.UndoSaveThreadResponse{}, nil
}
