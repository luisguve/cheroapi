package server

import(
	
)

// Update a thread, comment or subcomment
func (s *Server) UpdateContent(req *pbApi.UpdateContentRequest,
	stream pbApi.CrudCheropatilla_UpdateContentServer) error {
	
}

// Delete a thread, comment or subcomment
func (s *Server) DeleteContent(ctx context.Context,
	req *pbApi.DeleteContentRequest) (*pbApi.DeleteContentResponse, error) {
	
}

// Post a thread to create
func (s *Server) CreateThread(ctx context.Context, 
	req *pbApi.CreateThreadRequest) (*pbApi.CreateThreadResponse, error) {
	
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
