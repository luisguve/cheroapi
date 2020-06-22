package bolthandler

import(
	"log"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"github.com/satori/go.uuid"
	"golang.org/x/crypto/bcrypt"
	bolt "go.etcd.io/bbolt"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
)

// CheckUser returns the user id associated with the given username, and true
// if the given password and the stored password are equal, or an empty string
// and false if either such username doesn't exist or the password check failed.
func (h *handler) CheckUser(username, password string) (string, bool) {
	userId, err := h.findUserIdByUsername(username)
	if err != nil {
		log.Println(err)
		return "", false
	}

	userData := new(pbDataFormat.User)
	err = h.users.View(func(tx *bolt.Tx) error {
		usersBucket := tx.Bucket([]byte(usersB))
		if usersBucket == nil {
			return fmt.Errorf("Bucket %s of users not found\n", usersB)
		}
		userDataBytes := usersBucket.Get(userId)
		if userDataBytes == nil {
			log.Printf("Could not find user data (id %s)\n", string(userId))
			return fmt.Errorf("User %s not found\n", string(userId))
		}
		err := proto.Unmarshal(userDataBytes, userData)
		if err != nil {
			log.Println("Could not unmarshal user")
			return err
		}
		return nil
	})
	if err != nil {
		log.Println(err)
		return "", false
	}
	hashedPw := userData.PrivateData.Password
	// check whether the provided password and the stored password are equal
	err = bcrypt.CompareHashAndPassword(hashedPw, []byte(password))
	if err != nil {
		log.Println(err)
		return "", false
	}
	// checked
	return string(userId), true
}

// RegisterUser creates a new user with the provided data and returns the user id
// of the just created and saved user and a nil *status.Status, or an empty string
// and a given *status.Status indicating what went wrong: email or username already
// in use, a database failure, an uuid or password hashing issue or a proto marshal
// error.
func (h *handler) RegisterUser(email, name, patillavatar, username, alias, about,
	password string) (string, *status.Status) {
	// check whether the username has been already taken
	_, err := findUserIdByUsername(username)
	// there must be an error, which should be ErrUsernameNotFound, otherwise the
	// query could not be completed or the username has already been taken.
	if err != nil {
		if !errors.Is(err, ErrUsernameNotFound) {
			return "", status.New(codes.Internal, "Failed to query database")
		}
	} else {
		return "", status.New(codes.AlreadyExists, "Username already taken")
	}

	// check whether the email has been already taken
	_, err := findUserIdByEmail(email)
	// there must be an error, which should be ErrEmailNotFound, otherwise the
	// query could not be completed or the email has already been taken.
	if err != nil {
		if !errors.Is(err, ErrEmailNotFound) {
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
			return fmt.Errorf("Bucket %s of users not found\n", usersB)
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
			return ErrBucketNotFound
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
			return ErrBucketNotFound
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

// CheckUserCanPost returns a bool indicating whether the given user can create a
// thread and an error which may indicate the user does not exist.
// 
// A user can create a thread if the field Seconds of its field LastTimeCreated is
// less than trhe field Seconds of the field LastCleanUp of the QA field in h.
func (h *handler) CheckUserCanPost(userId string) (bool, error) {
	userData := new(pbDataFormat.User)
	err := h.users.View(func(tx *bolt.Tx) error {
		usersBucket := tx.Bucket([]byte(usersB))
		if usersBucket == nil {
			return fmt.Errorf("Bucket %s of users not found\n", usersB)
		}
		userDataBytes := usersBucket.Get(userId)
		if userDataBytes == nil {
			log.Printf("Could not find user data (id %s)\n", string(userId))
			return ErrUserNotFound
		}
		err := proto.Unmarshal(userDataBytes, userData)
		if err != nil {
			log.Println("Could not unmarshal user")
			return err
		}
		return nil
	})
	if err != nil {
		return false, err
	}
	canPost := userData.LastTimeCreated.Seconds < h.QA.LastCleanUp.Seconds
	return canPost, nil
}

// User gets the user bytes from the users bucket in the database of users, then
// unmarshals it into a *pbDataFormat.User and returns it.
func (h *handler) User(userId string) (*pbDataFormat.User, error) {
	pbUser := new(pbDataFormat.User)
	err := h.users.View(func(tx *bolt.Tx) error {
		usersBucket := tx.Bucket(usersB)
		if usersBucket == nil {
			return fmt.Errorf("Bucket %s of users not found\n", usersB)
		}
		userBytes := usersBucket.Get([]byte(userId))
		if userBytes == nil {
			log.Printf("Could not find user data (id %s)\n", string(userId))
			return ErrUserNotFound
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
