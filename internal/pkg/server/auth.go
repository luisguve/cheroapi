package server

import(
	"context"

	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
)

// Validate user credentials to login
func (s *Server) Login(ctx context.Context,
	req *pbApi.LoginRequest) (*pbApi.LoginResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}

	userId, ok := s.dbHandler.CheckUser(req.Username, req.Password)
	if !ok {
		return nil, status.Error(codes.PermissionDenied, "Invalid username or password")
	}
	return &pbApi.LoginResponse{
		UserId: userId,
	}, nil
}

// Register new user
func (s *Server) RegisterUser(ctx context.Context,
	req *pbApi.RegisterUserRequest) (*pbApi.RegisterUserResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}

	email := req.Email
	name := req.Name
	patillavatar := req.PicUrl
	username := req.Username
	alias := req.Alias
	about := req.About
	password := req.Password

	userId, st := s.dbHandler.RegisterUser(email, name, patillavatar, username,
		alias, about, password)
	if st != nil {
		return nil, st.Err()
	}
	return &pbApi.RegisterUserResponse{
		UserId: userId,
	}, nil
}
