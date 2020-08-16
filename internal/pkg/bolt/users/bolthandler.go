// Package bolt/users provides a Handler for performing user-related CRUD
// operations on a bolt database.

package users

import (
	"log"
	"os"
	"path/filepath"

	dbmodel "github.com/luisguve/cheroapi/internal/app/userapi"
	bolt "go.etcd.io/bbolt"
)

// Names of buckets.
const (
	usersB            = "Everyone"
	usernameIdsB      = "UsernameIdMappings"
	// Store usernames in lowercase as the keys and the real usernames as
	// the values.
	lowercasedUsernamesB = "LowercasedUsernames"
	idUsernamesB         = "IdUsernameMappings"
	emailIdsB            = "EmailIdMappings"
	// Store emails in lowercase as the keys and the real emails as the
	// values.
	lowercasedEmailsB = "LowercasedEmails"
	idEmailsB         = "IdEmailMappings"
)

type handler struct {
	// database for user-related management.
	users *bolt.DB
}

// Close the database of users, return any occurred error.
func (h *handler) Close() error {
	return h.users.Close()
}

// New returns a dbmodel.Handler with a just open bolt database under a "users"
// folder in the directory specified by path for all the users. If the "users"
// folder does not exist, it is created.
func New(path string) (dbmodel.Handler, error) {

	// Open or create users database.
	usersPath := filepath.Join(path, "users")
	if _, err := os.Stat(usersPath); os.IsNotExist(err) {
		os.MkdirAll(usersPath, os.ModeDir)
	}
	usersFile := filepath.Join(usersPath, "users.db")
	usersDB, err := bolt.Open(usersFile, 0600, nil)
	if err != nil {
		return nil, err
	}

	// Setup buckets.
	err = usersDB.Update(func(tx *bolt.Tx) error {
		// Create bucket for users.
		_, err = tx.CreateBucketIfNotExists([]byte(usersB))
		if err != nil {
			log.Printf("Could not create bucket %s: %v\n", usersB, err)
			return err
		}

		// Create bucket for usernames to user ids mapping.
		_, err = tx.CreateBucketIfNotExists([]byte(usernameIdsB))
		if err != nil {
			log.Printf("Could not create bucket %s: %v\n", usernameIdsB, err)
			return err
		}
		// Create bucket for lowercased usernames to real usernames mappings.
		_, err = tx.CreateBucketIfNotExists([]byte(lowercasedUsernamesB))
		if err != nil {
			log.Printf("Could not create bucket %s: %v\n", lowercasedUsernamesB, err)
			return err
		}
		// Create bucket for emails to user ids mapping.
		_, err = tx.CreateBucketIfNotExists([]byte(emailIdsB))
		if err != nil {
			log.Printf("Could not create bucket %s: %v\n", emailIdsB, err)
			return err
		}
		// Create bucket for ids to emails mapping.
		_, err = tx.CreateBucketIfNotExists([]byte(idEmailsB))
		if err != nil {
			log.Printf("Could not create bucket %s: %v\n", idEmailsB, err)
			return err
		}
		// Create bucket for lowercased emails to real emails mapping.
		_, err = tx.CreateBucketIfNotExists([]byte(lowercasedEmailsB))
		if err != nil {
			log.Printf("Could not create bucket %s: %v\n", lowercasedEmailsB, err)
			return err
		}
		// Create bucket for ids to usernames mapping.
		_, err = tx.CreateBucketIfNotExists([]byte(idUsernamesB))
		if err != nil {
			log.Printf("Could not create bucket %s: %v\n", idUsernamesB, err)
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &handler{
		users: usersDB,
	}, nil
}
