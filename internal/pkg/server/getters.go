package server

import(
	
)

// Get a user's basic data to be displayed in the header navigation section
func (s *Server) GetUserHeaderData(ctx context.Context,
	req *pbApi.GetBasicUserDataRequest) (*pbApi.UserHeaderData, error) {
	
}

// Get a user's basic data to be dislayed in page
func (s *Server) GetBasicUserData(ctx context.Context,
	req *pbApi.GetBasicUserDataRequest) (*pbApi.BasicUserData, error) {
	
}

// Get the list of users followed by a given user
func (s *Server) GetUserFollowingIds(ctx context.Context,
	req *pbApi.GetBasicUserDataRequest) (*pbApi.UserList, error) {
	
}

// Get a single thread
func (s *Server) GetThread(ctx context.Context, 
	req *pbApi.GetThreadRequest) (*pbApi.ContentData, error) {
	
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
