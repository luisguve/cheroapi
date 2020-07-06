package server

import(
	"context"
	"errors"
	"golang.org/x/crypto/bcrypt"

	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	dbmodel "github.com/luisguve/cheroapi/internal/app/cheroapi"
)

// Validate user credentials to login
func (s *Server) Login(ctx context.Context, req *pbApi.LoginRequest) (*pbApi.LoginResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	userId, err := s.dbHandler.FindUserIdByUsername(req.Username)
	if err != nil {
		if errors.Is(err, dbmodel.ErrUsernameNotFound) {
			return nil, status.Error(codes.PermissionDenied, "Invalid username or password")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	pbUser, err := s.dbHandler.User(string(userId))
	if err != nil {
		if errors.Is(err, dbmodel.ErrUserNotFound) {
			return nil, status.Error(codes.PermissionDenied, "Invalid username or password")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	hashedPw := pbUser.PrivateData.Password
	// check whether the provided password and the stored password are equal
	err = bcrypt.CompareHashAndPassword(hashedPw, []byte(req.Password))
	if err != nil {
		return nil, status.Error(codes.PermissionDenied, "Invalid username or password")
	}

	return &pbApi.LoginResponse{
		UserId: string(userId),
	}, nil
}

// Register new user
func (s *Server) RegisterUser(ctx context.Context, req *pbApi.RegisterUserRequest) (*pbApi.RegisterUserResponse, error) {
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
