package userapi

import (
	"errors"

	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
	"google.golang.org/grpc/status"
)

type UpdateUserFunc func(*pbDataFormat.User) *pbDataFormat.User

// Handler defines the set of available CRUD operations to perform on the
// database.
type Handler interface {
	// Add user credentials and data (sign in); returns the user id and a nil
	// *status.Status on successful registering or an empty string an a *status.Status
	// indicating what went wrong (email or username already in use) otherwise.
	RegisterUser(email, name, patillavatar, username, alias, about, password string) (string, *status.Status)
	// Get data of a user.
	User(userId string) (*pbDataFormat.User, error)
	// Associate username to user id.
	MapUsername(username, userId string) error
	// Update data of user.
	UpdateUser(userId string, updateFn UpdateUserFunc) error
	// Get user id with the given username.
	FindUserIdByUsername(username string) ([]byte, error)
	// Get user id with the given email.
	FindUserIdByEmail(email string) ([]byte, error)
	// Release all database resources.
	Close() error
}

// These errors are returned when data is not found.
var (
	ErrUserNotFound     = errors.New("User not found")
	ErrUsernameNotFound = errors.New("Username not found")
	ErrEmailNotFound    = errors.New("Email not found")
	ErrBucketNotFound   = errors.New("Bucket not found")
)

// These errors can be returned when submitting actions.
var (
	// A user wants to use an unavailable username.
	ErrUsernameAlreadyExists = errors.New("Username already exists")
	// A user wants to use an unavailable email.
	ErrEmailAlreadyExists = errors.New("Email already exists")
)
