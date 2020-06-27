package bolt

import(
	"log"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"github.com/satori/go.uuid"
	"golang.org/x/crypto/bcrypt"
	bolt "go.etcd.io/bbolt"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
	"github.com/luisguve/cheroapi/internal/pkg/dbmodel"
)

// FindUserIdByUsername looks for a user id with the given username as the key
// in the bucket usernamesB from the users database of h, and returns it and a
// nil error if it could be found, or a nil []byte and an ErrUsernameNotFound
// if the username could not be found or an ErrBucketNotFound if the query could
// not be completed.
func (h *handler) FindUserIdByUsername(username string) ([]byte, error) {
	var (
		userId []byte
		err error
	)
	err = h.users.View(func(tx *bolt.Tx) error {
		usernamesBucket := tx.Bucket([]byte(usernamesB))
		if usernamesBucket == nil {
			log.Printf("Bucket %s of users not found\n", usernamesB)
			return dbmodel.ErrBucketNotFound
		}
		userIdBytes := usernamesBucket.Get([]byte(username))
		if userIdBytes == nil {
			return dbmodel.ErrUsernameNotFound
		}
		copy(userId, userIdBytes)
		return nil
	})
	return userId, err
}

// FindUserIdByEmail looks for a user id with the given email as the key in the
// bucket usernamesB from the users database of h, and returns it and a nil
// error if it could be found, or a nil []byte and an ErrEmailNotFound if the
// username could not be found or an ErrBucketNotFound if the query could not
// not be completed.
func (h *handler) FindUserIdByEmail(email string) ([]byte, error) {
	var (
		userId []byte
		err error
	)
	err = h.users.View(func(tx *bolt.Tx) error {
		emailsBucket := tx.Bucket([]byte(emailsB))
		if emailsBucket == nil {
			log.Printf("Bucket %s of users not found\n", emailsB)
			return dbmodel.ErrBucketNotFound
		}
		userIdBytes := emailsBucket.Get([]byte(email))
		if userIdBytes == nil {
			return dbmodel.ErrEmailNotFound
		}
		copy(userId, userIdBytes)
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
	// check whether the username has been already taken
	_, err := FindUserIdByUsername(username)
	// there must be an error, which should be ErrUsernameNotFound, otherwise the
	// query could not be completed or the username has already been taken.
	if err != nil {
		if !errors.Is(err, dbmodel.ErrUsernameNotFound) {
			return "", status.New(codes.Internal, "Failed to query database")
		}
	} else {
		return "", status.New(codes.AlreadyExists, "Username already taken")
	}

	// check whether the email has been already taken
	_, err := FindUserIdByEmail(email)
	// there must be an error, which should be ErrEmailNotFound, otherwise the
	// query could not be completed or the email has already been taken.
	if err != nil {
		if !errors.Is(err, dbmodel.ErrEmailNotFound) {
			return "", status.New(codes.Internal, "Failed to query database")
		}
	} else {
		return "", status.New(codes.AlreadyExists, "Email already taken")
	}

	// generate universally unique identifier for the user id.
	userIdBytes, err := uuid.NewV4()
	if err != nil {
		log.Printf("Could not get new uuid V4: %v\n", err)
		return "", status.New(codes.Internal, "Could not generate user id")
	}
	userId := userIdBytes.String()

	// hash password 10 times (Default cost)
	hashedPw, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Could not hash password \"%s\": %v\n", password, err)
		return "", status.New(codes.Internal, "Could not generate password hash")
	}

	// format user in protobuf message
	pbUser := &pbDataFormat.User{
		BasicUserData: &pbDataFormat.BasicUserData{
			Alias: alias,
			Username: username,
			PicUrl: patillavatar,
			About: about,
		},
		PrivateData: &pbDataFormat.PrivateData{
			Email: email,
			Password: hashedPw,
		},
	}
	// encode user into bytes
	pbUserBytes, err := proto.Marshal(pbUser)
	if err != nil {
		log.Printf("Could not marshal user: %v\n", err)
		return "", status.New(codes.Internal, "Could not marshal user")
	}

	// save user data
	err = h.users.Update(func(tx *bolt.Tx) error {
		// save user into users database
		usersBucket := tx.Bucket([]byte(usersB))
		if usersBucket == nil {
			log.Printf("Bucket %s of users not found\n", usersB)
			return dbmodel.ErrBucketNotFound
		}
		err := usersBucket.Put([]byte(userId), pbUserBytes)
		if err != nil {
			log.Printf("Could not put user: %v\n", err)
			return err
		}

		// associate username to user id
		usernamesBucket := tx.Bucket([]byte(usernamesB))
		if usernamesBucket == nil {
			log.Printf("Bucket %s of users not found\n", usernamesB)
			return dbmodel.ErrBucketNotFound
		}
		err = usernamesBucket.Put([]byte(username), []byte(userId))
		if err != nil {
			log.Printf("Could not put username: %v\n", err)
			return err
		}

		// associate email to user id
		emailsBucket := tx.Bucket([]byte(emailsB))
		if emailsBucket == nil {
			log.Printf("Bucket %s of users not found\n", emailsB)
			return dbmodel.ErrBucketNotFound
		}
		err = emailsBucket.Put([]byte(email), []byte(userId))
		if err != nil {
			log.Printf("Could not put user email: %v\n", err)
			return err
		}
		return nil
	})
	if err != nil {
		log.Println(err)
		return "", status.New(codes.Internal, "Failed to query database")
	}
	return userId, nil
}

// User gets the user bytes from the users bucket in the database of users, then
// unmarshals it into a *pbDataFormat.User and returns it.
func (h *handler) User(userId string) (*pbDataFormat.User, error) {
	pbUser := new(pbDataFormat.User)
	err := h.users.View(func(tx *bolt.Tx) error {
		usersBucket := tx.Bucket(usersB)
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

// MapUsername associates username to user id, returns ErrUsernameAlreadyExists
// if the username is not available.
func (h *handler) MapUsername(username, userId string) error {
	return h.users.Update(func(tx *bolt.Tx) error {
		// associate username to user id
		usernamesBucket := tx.Bucket([]byte(usernamesB))
		if usernamesBucket == nil {
			log.Printf("Bucket %s of users not found\n", usernamesB)
			return dbmodel.ErrBucketNotFound
		}
		userIdBytes := usernamesBucket.Get([]byte(username))
		if userIdBytes != nil {
			return dbmodel.ErrUsernameAlreadyExists
		}
		return usernamesBucket.Put([]byte(username), []byte(userId))
	})
}

// UpdateUser marshals the given pbUser and puts it into the database with
// userId as the key.
func (h *handler) UpdateUser(pbUser *pbDataFormat.User, userId string) error {
	pbUserBytes, err := proto.Marshal(pbUser)
	if err != nil {
		log.Printf("Could not marshal user: %v\n", err)
		return err
	}
	return h.users.Update(func(tx *bolt.Tx) error {
		// save user into users database
		usersBucket := tx.Bucket([]byte(usersB))
		if usersBucket == nil {
			log.Printf("Bucket %s of users not found\n", usersB)
			return dbmodel.ErrBucketNotFound
		}
		return usersBucket.Put([]byte(userId), pbUserBytes)
	}
}
