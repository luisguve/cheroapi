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
// archived contents. It will only evaluate the relevante of threads with 1 day
// or longer and move them accordingly, along with its comments and subcomments.
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
			// Check whether the thread has been around for less than one day.
			// If so, it doesn't qualify for the relevance evaluation and it
			// will be skipped.
			published := time.Unix(pbThread.PublishDate.Seconds, 0)
			diff := now.Sub(published)
			if diff < (24 * time.Hour) {
				continue
			}
			m := pbThread.Metadata

			lastUpdated := time.Unix(m.LastUpdated.Seconds, 0)

			diff = now.Sub(lastUpdated)
			diff += time.Duration(m.Diff) * time.Second

			avgUpdateTime := diff.Seconds() / float64(m.Interactions)
			// Check whether the thread is still relevant. It should have more
			// than 100 interactions and the average time difference between
			// interactions must be no longer than 1 hour.
			// If so, it will be skipped.
			if (m.Interactions > 100) && (avgUpdateTime <= 1 * time.Hour) {
				continue
			}
			// Otherwise, it will be moved to the bucket of archived contents for
			// read only, along with the contents associated to it; comments and
			// subcomments.
			if err = archivedContents.Put(k, v); err != nil {
				log.Printf("Could not PUT thread to archived contents: %v\n", err)
				return err
			}
			if err = activeContents.Delete(k); err != nil {
				log.Printf("Could not DEL thread from active contents: %v\n", err)
				return err
			}
			// Move comments to archived contents bucket
			
		}
	})
}
