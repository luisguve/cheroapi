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
		done  = make(chan error)
		quit  = make(chan struct{})
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
		case done <- err:
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
		case done <- err:
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
		done  = make(chan error)
		quit  = make(chan struct{})
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
		case done <- err:
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
		case done <- err:
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

// inSlice returns whether user is in users and an integer indicating the index
// where the user id is in the slice. Returns false an 0 if the user is not in
// users slice.
func inSlice(users []string, user string) (bool, int) {
	for idx, u := range users {
		if u == user {
			return true, idx
		}
	}
	return false, 0
}
