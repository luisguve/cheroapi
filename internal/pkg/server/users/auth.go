package users

import (
	"context"
	"errors"

	"golang.org/x/crypto/bcrypt"

	dbmodel "github.com/luisguve/cheroapi/internal/app/userapi"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
	pbApi "github.com/luisguve/cheroproto-go/userapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
