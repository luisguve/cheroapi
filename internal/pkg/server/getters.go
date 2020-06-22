package server

import(
	
)

// Get a user's basic data to be displayed in the header navigation section
func (s *Server) GetUserHeaderData(ctx context.Context, req *pbApi.GetBasicUserDataRequest) (*pbApi.UserHeaderData, error) {
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
		if errors.Is(err, ErrUserNotFound) {
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
		if errors.Is(err, ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.UserList{
		Ids: pbUser.FollowingIds,
	}, nil
}

// Get a single thread
func (s *Server) GetThread(ctx context.Context, req *pbApi.GetThreadRequest) (*pbApi.ContentData, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	contentRule, err := s.dbHandler.GetThread(req.Thread)
	if err != nil {
		if errors.Is(err, ErrSectionNotFound) || errors.Is(err, ErrThreadNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return contentRule.Data, nil
}

// Get a comment's comments
func (s *Server) GetSubcomments(req *pbApi.GetSubcommentsRequest,
	stream pbApi.CrudCheropatilla_GetSubcommentsServer) error {
	
}

// Get either following or followers users' basic data
func (s *Server) ViewUsers(ctx context.Context,
	req *pbApi.ViewUsersRequest) (*pbApi.ViewUsersResponse, error) {
	
}

// Get username basic data, following, followers and threads created
func (s *Server) ViewUserByUsername(ctx context.Context,
	req *pbApi.ViewUserByUsernameRequest) (*pbApi.ViewUserResponse, error) {
	
}

// Get dashboard data for a given user
func (s *Server) GetDashboardData(ctx context.Context,
	req *pbApi.GetDashboardDataRequest) (*pbApi.DashboardData, error) {
	
}
