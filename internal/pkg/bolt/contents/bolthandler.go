// Package contents provides a Handler for performing CRUD operations on a
// section's bolt database.

package contents

import (
	"log"
	"os"
	"path/filepath"
	"time"

	dbmodel "github.com/luisguve/cheroapi/internal/app/cheroapi"
	pbApi "github.com/luisguve/cheroproto-go/userapi"
	bolt "go.etcd.io/bbolt"
)

// names of buckets
const (
	activeContentsB   = "ActiveContents"
	archivedContentsB = "ArchivedContents"
	commentsB         = "Comments"
	subcommentsB      = "Subcomments"
	deletedThreadsB   = "DeletedThreads"
	deletedCommentsB  = "DeletedComments"
)

type handler struct {
	section section // Contents section
	lastQA  int64 // Last time a clean up was done.
	users   pbApi.CrudUsersClient // Connection to remote users service.
}

type section struct {
	// Every section has its own database, which holds two buckets: one for
	// read-write (activeContentsB) and one for read only (archivedContentsB).
	contents *bolt.DB
	path     string // Absolute path to the database, including extension
	name     string // Section name
	id       string // Section ID
}

// Close the section database and returns an error, if any.
func (h *handler) Close() error {
	return h.section.contents.Close()
}

// New returns a dbmodel.Handler with a few just open bolt databases under the
// directory specified by path for the given sections.
//
// The section databases hold a couple of buckets: one for active contents with
// read-write access and other for archived content with read-only access. If
// they already exists, they aren't created again.
//
// The top-level bucket of active contents holds key/value pairs representing
// thread ids and thread contents, respectively, a comments bucket and a bucket
// for deleted threads, which hold the thread-id/thread-content pairs of deleted
// threads.
//
// The bucket of comments has a bucket for each thread, where the keys are the
// same as the key of the thread the comments belong to. Each of these buckets
// have key/value pairs representing comment ids and comment contents,
// respectively, a bucket for subcomments and a bucket for deleted comments,
// which hold the comment-id/comment-content pairs of deleted comments.
//
// Finally, the subcomments bucket has a bucket for each comment, where the keys
// are the same as the key of the comment the subcomments belong to. Each of
// these buckets have key/value pairs representing subcomment ids and subcomment
// contents, respectively.
//
// Both comments and subcomments have numeric, sequential ids.
//
// The bucket of archived contents has almost the same structure. The only
// difference is that it doesn't have buckets for deleted contents.
//
// New only creates the bucket of active contents and the bucket of archived
// contents, along with their top-level bucket for comments. In the bucket of
// active contents, it also creates a bucket for deleted threads.
func New(path string, sectionId, sectionName string, usersClient pbApi.CrudUsersClient) (dbmodel.Handler, error) {

	// open or create section database
	dbPath := filepath.Join(path, sectionId)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		os.MkdirAll(dbPath, os.ModeDir)
	}
	dbFile := filepath.Join(dbPath, "contents.db")
	db, err := bolt.Open(dbFile, 0600, nil)
	if err != nil {
		return nil, err
	}
	// create bucket for active contents and for archived contents
	err = db.Update(func(tx *bolt.Tx) error {
		// active
		b, err := tx.CreateBucketIfNotExists([]byte(activeContentsB))
		if err != nil {
			log.Printf("Could not create bucket %s: %v\n", activeContentsB, err)
			return err
		}
		_, err = b.CreateBucketIfNotExists([]byte(commentsB))
		if err != nil {
			log.Printf("Could not create bucket %s: %v\n", commentsB, err)
			return err
		}
		_, err = b.CreateBucketIfNotExists([]byte(deletedThreadsB))
		if err != nil {
			log.Printf("Could not create bucket %s: %v\n", deletedThreadsB, err)
			return err
		}
		// archived
		b, err = tx.CreateBucketIfNotExists([]byte(archivedContentsB))
		if err != nil {
			log.Printf("Could not create bucket %s: %v\n", archivedContentsB, err)
			return err
		}
		_, err = b.CreateBucketIfNotExists([]byte(commentsB))
		if err != nil {
			log.Printf("Could not create bucket %s: %v\n", commentsB, err)
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	now := time.Now()

	return &handler{
		users:    usersClient,
		section:  section{
			contents: db,
			path:     dbFile,
			name:     sectionName,
			id:       sectionId,
		},
		lastQA:   now.Unix(),
	}, nil
}
