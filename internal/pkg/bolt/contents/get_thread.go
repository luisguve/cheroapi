package contents

import (
	"log"
	"sync"

	"github.com/golang/protobuf/proto"
	dbmodel "github.com/luisguve/cheroapi/internal/app/cheroapi"
	"github.com/luisguve/cheroapi/internal/pkg/patillator"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	pbContext "github.com/luisguve/cheroproto-go/context"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
	bolt "go.etcd.io/bbolt"
)

// Get metadata of the threads with the given ids. If setSDF is set, it is
// used as the callback to set the metadata, otherwise a default one will be
// used.
func (h *handler) GetThreadsOverview(ids []string, setSDF ...patillator.SetSDF) ([]patillator.SegregateDiscarderFinder, error) {
	var (
		contents []patillator.SegregateDiscarderFinder
	)

	var setContent patillator.SetSDF

	if len(setSDF) > 0 {
		setContent = setSDF[0]
	} else {
		setContent = func(c *pbDataFormat.Content) patillator.SegregateDiscarderFinder {
			return patillator.Content(*c.Metadata)
		}
	}

	// query database
	err := h.section.contents.View(func(tx *bolt.Tx) error {
		for _, id := range ids {
			threadBytes, err := getThreadBytes(tx, id)
			if err != nil {
				return err
			}
			pbContent := new(pbDataFormat.Content)
			if err := proto.Unmarshal(threadBytes, pbContent); err != nil {
				log.Printf("Could not unmarshal content: %v\n", err)
				return err
			}
			content := setContent(pbContent)
			contents = append(contents, content)
		}
		return nil
	})
	return contents, err
}

// Get metadata of all the active threads in the section. If setSDF is set, it is
// used as the callback to set the metadata, otherwise a default one will be
// used.
func (h *handler) GetActiveThreadsOverview(setSDF ...patillator.SetSDF) ([]patillator.SegregateDiscarderFinder, error) {
	var (
		contents  []patillator.SegregateDiscarderFinder
		err       error
	)

	var setContent func(c *pbDataFormat.Content) patillator.SegregateDiscarderFinder

	if len(setSDF) > 0 {
		setContent = setSDF[0]
	} else {
		setContent = func(c *pbDataFormat.Content) patillator.SegregateDiscarderFinder {
			return patillator.Content(*c.Metadata)
		}
	}

	// query database
	err = h.section.contents.View(func(tx *bolt.Tx) error {
		activeContents := tx.Bucket([]byte(activeContentsB))
		if activeContents == nil {
			log.Printf("bucket %s not found\n", activeContentsB)
			return dbmodel.ErrBucketNotFound
		}
		var (
			done  = make(chan error)
			quit  = make(chan error)
			wg    sync.WaitGroup
			m     sync.Mutex
			c     = activeContents.Cursor()
			elems = 0
		)
		for k, v := c.First(); k != nil; k, v = c.Next() {
			// Check whether the value is a nested bucket. If so, just continue.
			// Cursors see nested buckets with value == nil.
			if v == nil {
				continue
			}
			elems++
			wg.Add(1)
			// Do the unmarshaling, content setting and content appending in its
			// own go-routine. Should it get an error and it will send it to the
			// channel done, otherwise it will be sending nil to the same channel,
			// meaning it could complete its work successfully.
			go func(contentBytes []byte) {
				defer wg.Done()
				pbContent := new(pbDataFormat.Content)
				if err := proto.Unmarshal(contentBytes, pbContent); err != nil {
					log.Printf("Could not unmarshal content: %v\n", err)
				} else {
					content := setContent(pbContent)
					m.Lock()
					contents = append(contents, content)
					m.Unlock()
				}
				select {
				case done <- err:
				case <-quit: // exit in case of getting stuck on above statement.
				}
			}(v)
		}
		// Check for errors. It terminates every go-routine hung on the statement
		// "case done<- err" by closing the channel quit and returns the first err
		// read.
		// Note that the tx must wait until all the goroutines are done, since all
		// the byte slices are valid only during the life of the transaction.
		var err error
		for i := 0; i < elems; i++ {
			err = <-done
			if err != nil {
				close(quit)
				break
			}
		}
		wg.Wait()
		return err
	})
	return contents, err
}

// Get content of the given thread ids in the given section.
//
// It only queries threads from the bucket of active contents.
func (h *handler) GetThreads(ids []patillator.Id) ([]*pbApi.ContentRule, error) {
	var (
		err          error
		contentRules = make([]*pbApi.ContentRule, len(ids))
		section = &pbContext.Section{
			Id: h.section.id,
		}
	)

	err = h.section.contents.View(func(tx *bolt.Tx) error {
		activeContents := tx.Bucket([]byte(activeContentsB))
		if activeContents == nil {
			log.Printf("bucket %s not found\n", activeContentsB)
			return dbmodel.ErrBucketNotFound
		}
		var (
			done  = make(chan error)
			quit  = make(chan error)
			wg    sync.WaitGroup
			elems = 0
		)
		for idx, id := range ids {
			// Do the content querying, unmarshaling, formatting and appending
			// in its own go-routine. Should it get an error and it will send
			// it to the channel done, otherwise it will be sending nil to the
			// same channel, meaning it could complete its work successfully.
			elems++
			wg.Add(1)
			go func(idx int, id, status string) {
				contentRules[idx] = &pbApi.ContentRule{}
				defer wg.Done()
				v := activeContents.Get([]byte(id))
				// Check whether the content was found (is currently active)
				var err error
				if v != nil {
					pbContent := new(pbDataFormat.Content)
					if err = proto.Unmarshal(v, pbContent); err != nil {
						log.Printf("Could not unmarshal content: %v\n", err)
					} else {
						contentRule := h.formatThreadContentRule(pbContent, section, id)
						contentRule.Status = status
						contentRules[idx] = contentRule
					}
				}
				select {
				case done <- err:
				case <-quit: // exit in case of getting stuck on above statement.
				}
			}(idx, id.Id, id.Status)
		}
		// Check for errors. It terminates every go-routine hung on the statement
		// "case done<- err" by closing the channel quit and returns the first err
		// read.
		// Note that the tx must wait until all the goroutines are done, since all
		// the byte slices are valid only during the life of the transaction.
		var err error
		for i := 0; i < elems; i++ {
			err = <-done
			if err != nil {
				close(quit)
				break
			}
		}
		wg.Wait()
		return err
	})
	return contentRules, err
}

// GetThreadContent looks for the thread with the given id in both the active
// and archived contents bucket.
//
// If it found the thread, it marshals it into a *pbDataFormat.Content and
// returns it. Otherwise, it returns a nil Content and a ErrThreadNotFound error
// or a proto unmarshal error.
func (h *handler) GetThreadContent(thread *pbContext.Thread) (*pbDataFormat.Content, error) {
	var (
		err       error
		id        = thread.Id
		pbContent = new(pbDataFormat.Content)
	)

	err = h.section.contents.View(func(tx *bolt.Tx) error {
		threadBytes, err := getThreadBytes(tx, id)
		if err != nil {
			log.Printf("Could not find thread %s: %v\n", id, err)
			return err
		}

		if err = proto.Unmarshal(threadBytes, pbContent); err != nil {
			log.Printf("Could not unmarshal content: %v\n", err)
			return err
		}
		return nil
	})
	return pbContent, err
}

// GetThread queries looks for the thread with the given id in both the active
// and archived contents bucket.
//
// If it found the thread, it converts it into a ContentRule and returns it.
// Otherwise, it returns a nil ContentRule and an error.
func (h *handler) GetThread(thread *pbContext.Thread) (*pbApi.ContentRule, error) {
	pbContent, err := h.GetThreadContent(thread)
	if err != nil {
		return nil, err
	}

	contentRule := h.formatThreadContentRule(pbContent, thread.SectionCtx, thread.Id)
	return contentRule, nil
}
