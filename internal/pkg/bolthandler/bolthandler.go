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
	pbApi "github.com/luisguve/cheroapi/internal/protogen/cheropatillapb"
	pbDataFormat "github.com/luisguve/cheroapi/internal/protogen/dataformat"
	pbMetadata "github.com/luisguve/cheroapi/internal/protogen/metadata"
	pbContext "github.com/luisguve/cheroapi/internal/protogen/context"
)

// names of buckets
const (
	activeContentsB = "ActiveContents"
	archivedContentsB = "ArchivedContents"
	usersB = "Everyone"
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
	ErrNoSavedThreads = "This user has not saved any thread yet"
)

var sectionIds = []string{
	"mylife", "food", "tech", "art", "music", "diy", "questions", "literature",
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
	for _, sectionId := range sectionIds {
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
		sectionsDBs[sectionId] = section{db, dbPath}
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
		log.Printf("Could not create bucket: %v\n", err)
		return err
	}); err != nil {
		return nil, err
	}

	return &handler{
		users: usersDB,
		sections: sectionsDBs,
	}
}
