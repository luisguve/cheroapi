// package bolthandler provides a Handler for performing CRUD operations
// on a bolt database.

package bolthandler

import(
	"log"
	"fmt"
	"sync"
	"time"
	"errors"
	"math/rand"

	bolt "go.etcd.io/bbolt"
	"github.com/luisguve/cheroapi/internal/pkg/dbmodel"
	"google.golang.org/protobuf/proto"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
	pbMetadata "github.com/luisguve/cheroproto-go/metadata"
	pbContext "github.com/luisguve/cheroproto-go/context"
)

// names of buckets
const (
	activeContentsB = "ActiveContents"
	archivedContentsB = "ArchivedContents"
	usersB = "Everyone"
	usernamesB = "UsernameMappings"
	emailsB = "EmailMappings"
	commentsB = "Comments"
	subcommentsB = "Subcomments"
)

const (
	ErrSectionNotFound = "Section not found"
	ErrBucketNotFound = "Bucket not found"
	ErrUserNotFound = "User not found"
	ErrNoComments = "No comments available"
	ErrThreadNotFound = "Thread not found"
	ErrCommentNotFound = "Comment not found"
	ErrSubcommentNotFound = "Subcomment not found"
	ErrNoSavedThreads = "This user has not saved any thread yet"
	ErrUsernameNotFound = "Username not found"
	ErrEmailNotFound = "Email not found"
	ErrSubcommentsBucketNotFound = "Subcomments bucket not found"
	ErrNotUpvoted = "This user has not upvoted this content"
	ErrUserUnableToPost = "This user cannot post another thread today"
	ErrUsernameAlreadyExists = "Username already exists"
	ErrOffsetOutOfRange = "Offset out of range"
)

var sectionIds = map[string]string{
	"My Life": "mylife",
	"Food": "food",
	"Technology": "tech",
	"Art": "art",
	"Music": "music",
	"Do it yourself": "diy",
	"Questions": "questions",
	"Literature": "literature",
}

type handler struct {
	// database for user management.
	users *bolt.DB
	// section ids (lowercased, space-trimmed name) mapped to section.
	sections map[string]section
}

type section struct {
	// every section has its own database, which holds two buckets: one for
	// read-write (activeContentsB) and one for read only (archivedContentsB).
	contents *bolt.DB
	// relative path to the database, including extension
	path string
	// section name
	name string
}

// New returns a dbmodel.Handler with a few just open bolt databases; one for
// all the users and one for each section.
//
// The section databases hold a couple of buckets: one for active contents with 
// read-write access and other for archived content with read-only access. If
// they already exists, they aren't created again.
func New() (dbmodel.Handler, error) {
	sectionsDBs := make(map[string]section)

	// open or create section databases
	for sectionName, sectionId := range sectionIds {
		dbPath := sectionId + "/contents.db"
		db, err := bolt.Open(dbPath, 0600, nil)
		if err != nil {
			return nil, err
		}
		// create bucket for active contents and for archived contents
		err = db.Update(func(tx *bolt.Tx) error {
			// active
			_, err = tx.CreateBucketIfNotExists([]byte(activeContentsB))
			if err != nil {
				log.Printf("Could not create bucket %s: %v\n", activeContentsB, err)
				return fmt.Errorf("Could not create bucket: %v", err)
			}
			// archived
			_, err = tx.CreateBucketIfNotExists([]byte(archivedContentsB))
			if err != nil {
				log.Printf("Could not create bucket %s: %v\n", archivedContentsB, err)
				return fmt.Errorf("Could not create bucket: %v", err)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		// create bucket for archived contents
		sectionsDBs[sectionId] = section{
			contents: db,
			path: dbPath,
			name: sectionName,
		}
	}

	// open or create users database
	usersPath := "users/users.db"
	usersDB, err := bolt.Open(usersPath, 0600, nil)
	if err != nil {
		return nil, err
	}

	// create bucket for users
	if err = usersDB.Update(func(tx *bolt.Tx) error {
		_, err = tx.CreateBucketIfNotExists([]byte(usersB))
		log.Printf("Could not create bucket %s: %v\n", usersB, err)
		return err
	}); err != nil {
		return nil, err
	}

	// create bucket for usernames to user ids mapping
	if err = usersDB.Update(func(tx *bolt.Tx) error {
		_, err = tx.CreateBucketIfNotExists([]byte(usernamesB))
		log.Printf("Could not create bucket %s: %v\n", usernamesB, err)
		return err
	}); err != nil {
		return nil, err
	}

	// create bucket for emails to user ids mapping
	if err = usersDB.Update(func(tx *bolt.Tx) error {
		_, err = tx.CreateBucketIfNotExists([]byte(emailsB))
		log.Printf("Could not create bucket %s: %v\n", emailsB, err)
		return err
	}); err != nil {
		return nil, err
	}

	return &handler{
		users: usersDB,
		sections: sectionsDBs,
	}
}
