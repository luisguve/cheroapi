package bolt

import (
	"errors"
	"log"
	"strings"

	"github.com/golang/protobuf/proto"
	dbmodel "github.com/luisguve/cheroapi/internal/app/cheroapi"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
	uuid "github.com/satori/go.uuid"
	bolt "go.etcd.io/bbolt"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// FindUserIdByUsername looks for a user id with the given username as the key
// in the bucket usernameIdsB from the users database of h, and returns it and a
// nil error if it could be found, or a nil []byte and an ErrUsernameNotFound
// if the username could not be found or an ErrBucketNotFound if the query could
// not be completed.
func (h *handler) FindUserIdByUsername(username string) ([]byte, error) {
	var (
		userId []byte
		err    error
	)
	// lowercased username
	lcUsername := strings.ToLower(username)
	err = h.users.View(func(tx *bolt.Tx) error {
		// First find the real username, which should be the value of the
		// lowercased username as the key.
		usernamesBucket := tx.Bucket([]byte(lowercasedUsernamesB))
		if usernamesBucket == nil {
			log.Printf("Bucket %s of users not found\n", lowercasedUsernamesB)
			return dbmodel.ErrBucketNotFound
		}
		username := usernamesBucket.Get([]byte(lcUsername))
		if username == nil {
			return dbmodel.ErrUsernameNotFound
		}
		usernamesBucket = tx.Bucket([]byte(usernameIdsB))
		if usernamesBucket == nil {
			log.Printf("Bucket %s of users not found\n", usernameIdsB)
			return dbmodel.ErrBucketNotFound
		}
		userId = usernamesBucket.Get(username)
		if userId == nil {
			return dbmodel.ErrUsernameNotFound
		}
		return nil
	})
	return userId, err
}

// FindUserIdByEmail looks for a user id with the given email as the key in the
// bucket usernameIdsB from the users database of h, and returns it and a nil
// error if it could be found, or a nil []byte and an ErrEmailNotFound if the
// username could not be found or an ErrBucketNotFound if the query could not
// not be completed.
func (h *handler) FindUserIdByEmail(email string) ([]byte, error) {
	var (
		userId []byte
		err    error
	)
	lcEmail := strings.ToLower(email)
	err = h.users.View(func(tx *bolt.Tx) error {
		// First find the real email, which should be the value of the
		// lowercased email as the key.
		emailsBucket := tx.Bucket([]byte(lowercasedEmailsB))
		if emailsBucket == nil {
			log.Printf("Bucket %s of users not found\n", emailIdsB)
			return dbmodel.ErrBucketNotFound
		}
		email := emailsBucket.Get([]byte(lcEmail))
		if email == nil {
			return dbmodel.ErrEmailNotFound
		}
		emailsBucket = tx.Bucket([]byte(emailIdsB))
		if emailsBucket == nil {
			log.Printf("Bucket %s of users not found\n", emailIdsB)
			return dbmodel.ErrBucketNotFound
		}
		userId = emailsBucket.Get(email)
		if userId == nil {
			return dbmodel.ErrEmailNotFound
		}
		return nil
	})
	return userId, err
}

// RegisterUser creates a new user with the provided data and returns the user id
// of the just created and saved user and a nil *status.Status, or an empty string
// and a given *status.Status indicating what went wrong: email or username already
// in use, a database failure, an uuid or password hashing issue or a proto marshal
// error.
func (h *handler) RegisterUser(email, name, patillavatar, username, alias, about,
	password string) (string, *status.Status) {
	var userId string
	// Save user data. All the operations must be done on the same transaction,
	// otherwise there could be a race condition if several users are trying to
	// register the same username or email at the same time.
	err := h.users.Update(func(tx *bolt.Tx) error {
		// check whether the username has been already taken
		_, err := h.FindUserIdByUsername(username)
		// there must be an error, which should be ErrUsernameNotFound, otherwise the
		// query could not be completed or the username has already been taken.
		if err != nil {
			if !errors.Is(err, dbmodel.ErrUsernameNotFound) {
				return err
			}
		} else {
			return dbmodel.ErrUsernameAlreadyExists
		}

		// check whether the email has been already taken
		_, err = h.FindUserIdByEmail(email)
		// there must be an error, which should be ErrEmailNotFound, otherwise the
		// query could not be completed or the email has already been taken.
		if err != nil {
			if !errors.Is(err, dbmodel.ErrEmailNotFound) {
				return err
			}
		} else {
			return dbmodel.ErrEmailAlreadyExists
		}

		// generate universally unique identifier for the user id.
		userIdBytes, err := uuid.NewV4()
		if err != nil {
			log.Printf("Could not get new uuid V4: %v\n", err)
			return errors.New("Could not generate user id")
		}
		userId = userIdBytes.String()

		// hash password 10 times (Default cost)
		hashedPw, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("Could not hash password \"%s\": %v\n", password, err)
			return errors.New("Could not generate password hash")
		}

		// format user in protobuf message
		pbUser := &pbDataFormat.User{
			BasicUserData: &pbDataFormat.BasicUserData{
				Alias:    alias,
				Username: username,
				PicUrl:   patillavatar,
				About:    about,
				Name:     name,
			},
			PrivateData: &pbDataFormat.PrivateData{
				Email:    email,
				Password: hashedPw,
			},
		}
		// encode user into bytes
		pbUserBytes, err := proto.Marshal(pbUser)
		if err != nil {
			log.Printf("Could not marshal user: %v\n", err)
			return errors.New("Could not marshal user")
		}

		// save user into users database
		usersBucket := tx.Bucket([]byte(usersB))
		if usersBucket == nil {
			log.Printf("Bucket %s of users not found\n", usersB)
			return dbmodel.ErrBucketNotFound
		}
		err = usersBucket.Put([]byte(userId), pbUserBytes)
		if err != nil {
			log.Printf("Could not put user: %v\n", err)
			return err
		}

		// Associate username to user id.
		usernamesBucket := tx.Bucket([]byte(usernameIdsB))
		if usernamesBucket == nil {
			log.Printf("Bucket %s of users not found\n", usernameIdsB)
			return dbmodel.ErrBucketNotFound
		}
		err = usernamesBucket.Put([]byte(username), []byte(userId))
		if err != nil {
			log.Printf("Could not put username: %v\n", err)
			return err
		}
		// Associate user id to username.
		usernamesBucket = tx.Bucket([]byte(idUsernamesB))
		if usernamesBucket == nil {
			log.Printf("Bucket %s of users not found\n", idUsernamesB)
			return dbmodel.ErrBucketNotFound
		}
		err = usernamesBucket.Put([]byte(userId), []byte(username))
		if err != nil {
			log.Printf("Could not put username: %v\n", err)
			return err
		}
		// Associate lowercased username to real username.
		usernamesBucket = tx.Bucket([]byte(lowercasedUsernamesB))
		if usernamesBucket == nil {
			log.Printf("Bucket %s of users not found\n", lowercasedUsernamesB)
			return dbmodel.ErrBucketNotFound
		}
		lcUsername := strings.ToLower(username)
		err = usernamesBucket.Put([]byte(lcUsername), []byte(username))
		if err != nil {
			log.Printf("Could not put username: %v\n", err)
			return err
		}

		// Associate email to user id.
		emailsBucket := tx.Bucket([]byte(emailIdsB))
		if emailsBucket == nil {
			log.Printf("Bucket %s of users not found\n", emailIdsB)
			return dbmodel.ErrBucketNotFound
		}
		err = emailsBucket.Put([]byte(email), []byte(userId))
		if err != nil {
			log.Printf("Could not put user email: %v\n", err)
			return err
		}
		// Associate user id to email.
		emailsBucket = tx.Bucket([]byte(idEmailsB))
		if emailsBucket == nil {
			log.Printf("Bucket %s of users not found\n", idEmailsB)
			return dbmodel.ErrBucketNotFound
		}
		err = emailsBucket.Put([]byte(userId), []byte(email))
		if err != nil {
			log.Printf("Could not put user email: %v\n", err)
			return err
		}
		// Associate lowercased username to real username.
		emailsBucket = tx.Bucket([]byte(lowercasedEmailsB))
		if emailsBucket == nil {
			log.Printf("Bucket %s of users not found\n", lowercasedEmailsB)
			return dbmodel.ErrBucketNotFound
		}
		lcEmail := strings.ToLower(email)
		err = emailsBucket.Put([]byte(lcEmail), []byte(email))
		if err != nil {
			log.Printf("Could not put user email: %v\n", err)
			return err
		}
		return nil
	})
	if err == dbmodel.ErrUsernameAlreadyExists {
		return "", status.New(codes.AlreadyExists, "Username already taken")
	}
	if err == dbmodel.ErrEmailAlreadyExists {
		return "", status.New(codes.AlreadyExists, "Email already taken")
	}
	if err != nil {
		return "", status.New(codes.Internal, err.Error())
	}
	return userId, nil
}

// User gets the user bytes from the users bucket in the database of users, then
// unmarshals it into a *pbDataFormat.User and returns it.
func (h *handler) User(userId string) (*pbDataFormat.User, error) {
	pbUser := new(pbDataFormat.User)
	err := h.users.View(func(tx *bolt.Tx) error {
		usersBucket := tx.Bucket([]byte(usersB))
		if usersBucket == nil {
			log.Printf("Bucket %s of users not found\n", usersB)
			return dbmodel.ErrBucketNotFound
		}
		userBytes := usersBucket.Get([]byte(userId))
		if userBytes == nil {
			log.Printf("Could not find user data (id %s)\n", string(userId))
			return dbmodel.ErrUserNotFound
		}
		err := proto.Unmarshal(userBytes, pbUser)
		if err != nil {
			log.Println("Could not unmarshal user")
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return pbUser, nil
}

// MapUsername associates newUsername to user id, returns ErrUsernameAlreadyExists
// if the username is not available.
func (h *handler) MapUsername(newUsername, userId string) error {
	return h.users.Update(func(tx *bolt.Tx) error {
		usernamesBucket := tx.Bucket([]byte(usernameIdsB))
		if usernamesBucket == nil {
			log.Printf("Bucket %s of users not found\n", usernameIdsB)
			return dbmodel.ErrBucketNotFound
		}
		userIdBytes := usernamesBucket.Get([]byte(newUsername))
		if userIdBytes != nil {
			return dbmodel.ErrUsernameAlreadyExists
		}
		idsBucket := tx.Bucket([]byte(idUsernamesB))
		if idsBucket == nil {
			log.Printf("Bucket %s of users not found\n", idUsernamesB)
			return dbmodel.ErrBucketNotFound
		}
		oldUsername := idsBucket.Get([]byte(userId))
		if oldUsername == nil {
			return dbmodel.ErrUsernameNotFound
		}
		// Delete old username
		err := usernamesBucket.Delete(oldUsername)
		if err != nil {
			return err
		}
		// Set new username.
		err = usernamesBucket.Put([]byte(newUsername), []byte(userId))
		if err != nil {
			return err
		}
		// Reset value in ids to usernames mapping.
		err = idsBucket.Put([]byte(userId), []byte(newUsername))
		if err != nil {
			return err
		}
		// Delete lowercased version of old username.
		usernamesBucket = tx.Bucket([]byte(lowercasedUsernamesB))
		lcOldUsername := strings.ToLower(string(oldUsername))
		err = usernamesBucket.Delete([]byte(lcOldUsername))
		if err != nil {
			return err
		}
		// Set lowercased version of new username.
		lcNewUsername := strings.ToLower(newUsername)
		return usernamesBucket.Put([]byte(lcNewUsername), []byte(newUsername))
	})
}

// UpdateUser gets the user with the given user id, passes it to updateUserFn,
// which modifies it then returns it, and marshals the resulting pbUser and puts
// it into the database with userId as the key, all in the same transaction.
// If updateFn returns nil, it does not marshal the pbUser, does not update
// the database and returns a nil error.
func (h *handler) UpdateUser(userId string, updateFn dbmodel.UpdateUserFunc) error {
	return h.users.Update(func(tx *bolt.Tx) error {
		usersBucket := tx.Bucket([]byte(usersB))
		if usersBucket == nil {
			log.Printf("Bucket %s of users not found\n", usersB)
			return dbmodel.ErrBucketNotFound
		}
		userBytes := usersBucket.Get([]byte(userId))
		if userBytes == nil {
			log.Printf("Could not find user data (id %s)\n", string(userId))
			return dbmodel.ErrUserNotFound
		}
		pbUser := new(pbDataFormat.User)
		err := proto.Unmarshal(userBytes, pbUser)
		if err != nil {
			log.Printf("Could not unmarshal user: %v.\n", err)
			return err
		}
		pbUser = updateFn(pbUser)
		if pbUser == nil {
			return nil
		}
		userBytes, err = proto.Marshal(pbUser)
		if err != nil {
			log.Printf("Could not marshal user: %v\n", err)
			return err
		}
		return usersBucket.Put([]byte(userId), userBytes)
	})
}
