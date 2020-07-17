// package bolthandler provides a Handler for performing CRUD operations
// on a bolt database.

package bolt

import (
	"log"
	"os"
	"path/filepath"
	"time"

	dbmodel "github.com/luisguve/cheroapi/internal/app/cheroapi"
	bolt "go.etcd.io/bbolt"
)

// names of buckets
const (
	activeContentsB   = "ActiveContents"
	archivedContentsB = "ArchivedContents"
	usersB            = "Everyone"
	usernameIdsB      = "UsernameIdMappings"
	// store usernames in lowercase as the keys and the real usernames as
	// the values.
	lowercasedUsernamesB = "LowercasedUsernames"
	idUsernamesB         = "IdUsernameMappings"
	emailIdsB            = "EmailIdMappings"
	// store emails in lowercase as the keys and the real emails as the
	// values.
	lowercasedEmailsB = "LowercasedEmails"
	idEmailsB         = "IdEmailMappings"
	commentsB         = "Comments"
	subcommentsB      = "Subcomments"
	deletedThreadsB   = "DeletedThreads"
	deletedCommentsB  = "DeletedComments"
)

type handler struct {
	// database for user management.
	users *bolt.DB
	// section ids (lowercased, space-trimmed name) mapped to section.
	sections map[string]section
	// Last time a clean up was done.
	lastQA int64
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

// Close every section database and the database of users, return the first
// error, if any.
func (h *handler) Close() error {
	if err := h.users.Close(); err != nil {
		return err
	}
	for _, section := range h.sections {
		if err := section.contents.Close(); err != nil {
			return err
		}
	}
	return nil
}

// New returns a dbmodel.Handler with a few just open bolt databases under the
// directory specified by path; one for all the users and one for each section.
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
func New(path string) (dbmodel.Handler, error) {
	sectionsDBs := make(map[string]section)

	// open or create section databases
	for sectionName, sectionId := range dbmodel.SectionIds {
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
		// create bucket for archived contents
		sectionsDBs[sectionId] = section{
			contents: db,
			path:     dbFile,
			name:     sectionName,
		}
	}

	// open or create users database
	usersPath := filepath.Join(path, "users")
	if _, err := os.Stat(usersPath); os.IsNotExist(err) {
		os.MkdirAll(usersPath, os.ModeDir)
	}
	usersFile := filepath.Join(usersPath, "users.db")
	usersDB, err := bolt.Open(usersFile, 0600, nil)
	if err != nil {
		return nil, err
	}

	// create bucket for users
	err = usersDB.Update(func(tx *bolt.Tx) error {
		_, err = tx.CreateBucketIfNotExists([]byte(usersB))
		if err != nil {
			log.Printf("Could not create bucket %s: %v\n", usersB, err)
		}
		return err
	})
	if err != nil {
		return nil, err
	}

	// create bucket for usernames to user ids mapping
	err = usersDB.Update(func(tx *bolt.Tx) error {
		_, err = tx.CreateBucketIfNotExists([]byte(usernameIdsB))
		if err != nil {
			log.Printf("Could not create bucket %s: %v\n", usernameIdsB, err)
		}
		return err
	})
	if err != nil {
		return nil, err
	}

	// create bucket for lowercased usernames to real usernames mappings
	err = usersDB.Update(func(tx *bolt.Tx) error {
		_, err = tx.CreateBucketIfNotExists([]byte(lowercasedUsernamesB))
		if err != nil {
			log.Printf("Could not create bucket %s: %v\n", lowercasedUsernamesB, err)
		}
		return err
	})
	if err != nil {
		return nil, err
	}

	// create bucket for emails to user ids mapping
	err = usersDB.Update(func(tx *bolt.Tx) error {
		_, err = tx.CreateBucketIfNotExists([]byte(emailIdsB))
		if err != nil {
			log.Printf("Could not create bucket %s: %v\n", emailIdsB, err)
		}
		return err
	})
	if err != nil {
		return nil, err
	}

	// create bucket for ids to emails mapping
	err = usersDB.Update(func(tx *bolt.Tx) error {
		_, err = tx.CreateBucketIfNotExists([]byte(idEmailsB))
		if err != nil {
			log.Printf("Could not create bucket %s: %v\n", idEmailsB, err)
		}
		return err
	})
	if err != nil {
		return nil, err
	}

	// create bucket for lowercased emails to real emails mapping
	err = usersDB.Update(func(tx *bolt.Tx) error {
		_, err = tx.CreateBucketIfNotExists([]byte(lowercasedEmailsB))
		if err != nil {
			log.Printf("Could not create bucket %s: %v\n", lowercasedEmailsB, err)
		}
		return err
	})
	if err != nil {
		return nil, err
	}

	// create bucket for ids to usernames mapping
	err = usersDB.Update(func(tx *bolt.Tx) error {
		_, err = tx.CreateBucketIfNotExists([]byte(idUsernamesB))
		if err != nil {
			log.Printf("Could not create bucket %s: %v\n", idUsernamesB, err)
		}
		return err
	})
	if err != nil {
		return nil, err
	}

	now := time.Now()

	return &handler{
		users:    usersDB,
		sections: sectionsDBs,
		lastQA:   now.Unix(),
	}, nil
}
