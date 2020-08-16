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

// Get a user's basic data to be displayed in the header navigation section
func (s *Server) GetUserHeaderData(ctx context.Context, req *pbApi.GetBasicUserDataRequest) (*pbApi.UserHeaderData, error) {
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
	return &pbApi.UserHeaderData{
		Alias:           pbUser.BasicUserData.Alias,
		Username:        pbUser.BasicUserData.Username,
		UnreadNotifs:    pbUser.UnreadNotifs,
		ReadNotifs:      pbUser.ReadNotifs,
		LastTimeCreated: pbUser.LastTimeCreated,
	}, nil
}

// Get a user's basic data to be dislayed in page
func (s *Server) GetBasicUserData(ctx context.Context, req *pbApi.GetBasicUserDataRequest) (*pbDataFormat.BasicUserData, error) {
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
	return pbUser.BasicUserData, nil
}

// Get the list of users followed by a given user
func (s *Server) GetUserFollowingIds(ctx context.Context, req *pbApi.GetBasicUserDataRequest) (*pbApi.UserList, error) {
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
	return &pbApi.UserList{
		Ids: pbUser.FollowingIds,
	}, nil
}

// Get either following or followers users' basic data
func (s *Server) ViewUsers(ctx context.Context, req *pbApi.ViewUsersRequest) (*pbApi.ViewUsersResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	// number of users to get
	const Q = 10
	var (
		userId      = req.UserId
		followCtx   = req.Context
		offset      = int(req.Offset)
		pbUsersData = make([]*pbDataFormat.BasicUserData, Q)
		count       = 0
	)
	pbUser, err := s.dbHandler.User(userId)
	if err != nil {
		if errors.Is(err, dbmodel.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	var (
		done = make(chan error)
		quit = make(chan error)
	)
	// Get data of users concurrently.
	switch followCtx {
	case "following":
		if offset >= len(pbUser.FollowingIds) {
			return nil, status.Error(codes.OutOfRange, "Offset out of range")
		}
		userIds := pbUser.FollowingIds[offset:]
		for i := 0; (i < len(userIds)) && (count < Q); i++ {
			count++
			userId := userIds[i]
			go func(userId string, idx int) {
				var (
					pbUser *pbDataFormat.User
					err    error
				)
				pbUser, err = s.dbHandler.User(userId)
				if err == nil {
					pbUsersData[idx] = pbUser.BasicUserData
				}
				select {
				case done <- err:
				case <-quit:
				}
			}(userId, i)
		}
	case "followers":
		if offset >= len(pbUser.FollowersIds) {
			return nil, status.Error(codes.OutOfRange, "Offset out of range")
		}
		userIds := pbUser.FollowersIds[offset:]
		for i := 0; (i < len(userIds)) && (count < Q); i++ {
			count++
			userId := userIds[i]
			go func(userId string, idx int) {
				var (
					pbUser *pbDataFormat.User
					err    error
				)
				pbUser, err = s.dbHandler.User(userId)
				if err == nil {
					pbUsersData[idx] = pbUser.BasicUserData
				}
				select {
				case done <- err:
				case <-quit:
				}
			}(userId, i)
		}
	default:
		return nil, status.Error(codes.InvalidArgument, "Context should be either following or followers")
	}
	// Check for errors. It terminates every go-routine hung on the statement
	// "case done<- err" by closing the channel quit and returns the first err
	// read.
	for i := 0; i < count; i++ {
		err = <-done
		if err != nil {
			close(quit)
			return nil, status.Error(codes.Internal, err.Error())
		}
	}
	if count < Q {
		// It got data of less than Q users; re-slice pbUsersData to get rid
		// of the last Q - count (empty) data of users.
		pbUsersData = pbUsersData[:count]
	}
	return &pbApi.ViewUsersResponse{
		BasicUserData: pbUsersData,
	}, nil
}

// Get username basic data, following, followers and threads created
func (s *Server) ViewUserByUsername(ctx context.Context, req *pbApi.ViewUserByUsernameRequest) (*pbApi.ViewUserResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	userIdB, err := s.dbHandler.FindUserIdByUsername(req.Username)
	if err != nil {
		if errors.Is(err, dbmodel.ErrUsernameNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	userId := string(userIdB)
	pbUser, err := s.dbHandler.User(userId)
	if err != nil {
		if errors.Is(err, dbmodel.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.ViewUserResponse{
		Alias:           pbUser.BasicUserData.Alias,
		Username:        pbUser.BasicUserData.Username,
		PicUrl:          pbUser.BasicUserData.PicUrl,
		About:           pbUser.BasicUserData.About,
		UserId:          userId,
		LastTimeCreated: pbUser.LastTimeCreated,
		FollowersIds:    pbUser.FollowersIds,
		FollowingIds:    pbUser.FollowingIds,
	}, nil
}

// Get dashboard data for a given user
func (s *Server) GetDashboardData(ctx context.Context, req *pbApi.GetDashboardDataRequest) (*pbApi.DashboardData, error) {
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
	return &pbApi.DashboardData{
		UserHeaderData: &pbApi.UserHeaderData{
			Alias:           pbUser.BasicUserData.Alias,
			Username:        pbUser.BasicUserData.Username,
			UnreadNotifs:    pbUser.UnreadNotifs,
			ReadNotifs:      pbUser.ReadNotifs,
			LastTimeCreated: pbUser.LastTimeCreated,
		},
		FollowersIds: pbUser.FollowersIds,
		FollowingIds: pbUser.FollowingIds,
		SavedThreads: uint32(len(pbUser.SavedThreads)),
		UserId:       req.UserId,
	}, nil
}

// Get the recent activity of a user.
func (s *Server) RecentActivity(ctx context.Context, req *pbApi.RecentActivityRequest) (*pbDataFormat.Activity, error) {
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
	return pbUser.RecentActivity, nil
}

// Get the list of saved threads of a user.
func (s *Server) SavedThreads(ctx context.Context, req *pbApi.SavedThreadsRequest) (*pbApi.SavedThreadsResponse, error) {
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
	return &pbApi.SavedThreadsResponse{
		References: pbUser.SavedThreads,
	}, nil
}
