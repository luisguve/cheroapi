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
func (s *Server) UpdateBasicUserData(ctx context.Context, req *pbApi.UpdateBasicUserDataRequest) (*pbApi.UpdateBasicUserDataResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	pbUser, err := s.dbHandler.User(req.UserId)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	if req.Username != "" {
		err = s.dbHandler.MapUsername(req.Username, req.UserId)
		if err != nil {
			if errors.Is(err, ErrUsernameAlreadyExists) {
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
		if errors.Is(err, ErrUserNotFound) {
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
		if errors.Is(err, ErrUserNotFound) {
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
		followerId = req.UserId
		pbFollower *pbDataFormat.User
		pbFollowing *pbDataFormat.User
		done = make(chan error)
		quit = make(chan error)
	)
	// Get both users concurrently.
	go func() {
		var err error
		pbFollower, err = s.dbHandler.User(followerId)
		select {
		case done<- err:
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
		case done<- err:
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
			if errors.Is(err, ErrUserNotFound) {
				return nil, status.Error(codes.NotFound, err.Error())
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	// Update users data.
	pbFollower.FollowingIds = append(pbFollower.FollowingIds, followingId)
	pbFollowing.FollowerIds = append(pbFollowing.FollowerIds, followerId)
	// Save both users concurrently.
	go func() {
		err := s.dbHandler.UpdateUser(pbFollower, followerId)
		select {
		case done<- err:
		case <-quit:
		}
	}()
	go func() {
		err := s.dbHandler.UpdateUser(pbFollowing, followingId)
		select {
		case done<- err:
		case <-quit:
		}
	}()
	// Check for errors. If there was an error, it will close the channel quit,
	// terminating any go-routine hung on the statement "case done<- err:", and
	// return the error to the client.
	for i := 0; i < 2; i++ {
		err = <-done
		if err != nil {
			close(quit)
			if errors.Is(err, ErrUserNotFound) {
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
		fId = req.UserId
		pbF *pbDataFormat.User
		done = make(chan error)
		quit = make(chan error)
	)
	// Get both users concurrently.
	go func() {
		var err error
		pbF, err = s.dbHandler.User(fId)
		select {
		case done<- err:
		case <-quit:
		}
	}()
	go func() {
		ufIdB, err := s.dbHandler.FindUserIdByUsername(req.UserToUnFollow)
		if err == nil {
			ufId = string(ufIdB)
			pbUf, err = s.dbHandler.User(ufId)
		}
		select {
		case done<- err:
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
			if errors.Is(err, ErrUserNotFound) {
				return nil, status.Error(codes.NotFound, err.Error())
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	// Update users data.
	following, idx := inSlice(pbF.FollowingIds, ufId)
	if following {
		last := len(pbF.FollowingIds) - 1
		pbF.FollowingIds[idx] = pbF.FollowingIds[last]
		pbF.FollowingIds = pbF.FollowingIds[:last]
	}

	follower, idx := inSlice(pbUf.FollowerIds, fId)
	if follower {
		last := len(pbUf.FollowerIds) - 1
		pbUf.FollowerIds[idx] = pbUf.FollowerIds[last]
		pbUf.FollowerIds = pbUf.FollowerIds[:last]
	}
	// Save both users concurrently.
	go func() {
		err := s.dbHandler.UpdateUser(pbF, fId)
		select {
		case done<- err:
		case <-quit:
		}
	}()
	go func() {
		err := s.dbHandler.UpdateUser(pbUf, ufId)
		select {
		case done<- err:
		case <-quit:
		}
	}()
	// Check for errors. If there was an error, it will close the channel quit,
	// terminating any go-routine hung on the statement "case done<- err:", and
	// return the error to the client.
	for i := 0; i < 2; i++ {
		err = <-done
		if err != nil {
			close(quit)
			if errors.Is(err, ErrUserNotFound) {
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
		if errors.Is(err, ErrUserNotFound) {
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
		_, err = s.dbHandler.GetThreadContent(thread)
		if err != nil {
			if (errors.Is(err, ErrSectionNotFound)) ||
				(errors.Is(err, ErrThreadNotFound)) {
				return nil, status.Error(codes.NotFound, err.Error())
			}
			return nil, status.Error(codes.Internal, err.Error())
		}
		pbUser.SavedThreads = append(pbUser.SavedThreads, thread)
		err = s.dbHandler.UpdateUser(pbUser, userId)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	return &pbApi.SaveThreadResponse{}, nil
}

// Request to remove thread from user's list of saved threads
func (s *Server) UnsaveThread(ctx context.Context,
	req *pbApi.UnsaveThreadRequest) (*pbApi.UnsaveThreadResponse, error) {
	
}
