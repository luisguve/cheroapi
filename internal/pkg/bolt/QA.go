package bolt

import (
	"time"
	"log"

	"google.golang.org/protobuf/proto"
	"github.com/luisguve/cheroapi/internal/pkg/dbmodel"
	bolt "go.etcd.io/bbolt"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
)

// Return the last time a clean up was done.
func (h *handler) LastQA() int64 {
	return h.lastQA
}

// Trigger a clean up on every section database
func (h *handler) QA() {
	for _, section := range h.sections {
		go section.QA()
	}
}

// Move unpopular contents from the bucket of active contents to the bucket of
// archived contents. It will only evaluate threads with 1 day or longer for
// relevance and move them accordingly, along with its comments and subcomments.
func (s section) QA() {
	now := time.Now().Unix()
	err := s.contents.Update(func(tx *bolt.Tx) error {
		activeContents := tx.Bucket([]byte(activeContentsB))
		if activeContents == nil {
			log.Printf("Could not find bucket %s\n", activeContentsB)
			return dbmodel.ErrBucketNotFound
		}
		archivedContents := tx.Bucket([]byte(archivedContentsB))
		if archivedContents == nil {
			log.Printf("Could not find bucket %s\n", archivedContentsB)
			return dbmodel.ErrBucketNotFound
		}
		// Iterate over every key/value pair of active contents.
		c := activeContents.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			pbThread := new(pbDataFormat.Content)
			if err := proto.Unmarshal(pbThread, v); err != nil {
				log.Printf("Could not unmarshal content %s: %v\n", string(k), err)
				continue
			}
			// Check whether the thread has been around for more than one day.
			published := pbThread.PublishDate.Seconds
			diff := now - published
			if diff > time.Day()
		}
	})
}
