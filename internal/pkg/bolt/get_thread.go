package bolt

import (
	"context"
	"log"
	"sync"

	"github.com/golang/protobuf/proto"
	dbmodel "github.com/luisguve/cheroapi/internal/app/cheroapi"
	"github.com/luisguve/cheroapi/internal/pkg/patillator"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	pbContext "github.com/luisguve/cheroproto-go/context"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
	pbMetadata "github.com/luisguve/cheroproto-go/metadata"
	pbUsers "github.com/luisguve/cheroproto-go/userapi"
	bolt "go.etcd.io/bbolt"
)

// Get metadata of all the active threads in a section. It returns
// ErrSectionNotFound if the section is invalid and ErrBucketNotFound if the
// section doesn't have a bucket for active contents.
func (h *handler) GetThreadsOverview(section *pbContext.Section, setSDF ...patillator.SetSDF) ([]patillator.SegregateDiscarderFinder, error) {
	var (
		contents  []patillator.SegregateDiscarderFinder
		sectionId = section.Id
		err       error
	)
	// check whether the section exists
	sectionDB, ok := h.sections[sectionId]
	if !ok {
		return nil, dbmodel.ErrSectionNotFound
	}

	var setContent func(c *pbDataFormat.Content) patillator.SegregateDiscarderFinder

	if len(setSDF) > 0 {
		setContent = setSDF[0]
	} else {
		setContent = func(c *pbDataFormat.Content) patillator.SegregateDiscarderFinder {
			return patillator.Content(*c.Metadata)
		}
	}

	// query database
	err = sectionDB.contents.View(func(tx *bolt.Tx) error {
		activeContents := tx.Bucket([]byte(activeContentsB))
		if activeContents == nil {
			log.Printf("bucket %s of section %s not found\n", activeContentsB, sectionId)
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

// Get content of the given thread ids in the given section. It manages the
// unmarshaling of bytes
//
// It only queries threads from the bucket of active contents of the given
// section.
func (h *handler) GetThreads(section *pbContext.Section, ids []patillator.Id) ([]*pbApi.ContentRule, error) {
	var (
		err          error
		sectionId    = section.Id
		contentRules = make([]*pbApi.ContentRule, len(ids))
	)

	// check whether the section exists
	sectionDB, ok := h.sections[sectionId]
	if !ok {
		return nil, dbmodel.ErrSectionNotFound
	}
	err = sectionDB.contents.View(func(tx *bolt.Tx) error {
		activeContents := tx.Bucket([]byte(activeContentsB))
		if activeContents == nil {
			log.Printf("bucket %s of section %s not found\n", activeContentsB, sectionId)
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

// Get metadata of all the active threads in every section. It calls
// h.GetThreadsOverview for each section in a concurrent fashion and returns
// a map of section ids to []patillator.SegregateDiscarderFinder and a []error
// that returns each call to GetThreadsOverview.
func (h *handler) GetGeneralThreadsOverview() (map[string][]patillator.SegregateDiscarderFinder, []error) {
	var (
		contents map[string][]patillator.SegregateDiscarderFinder
		errs     []error
		m        sync.Mutex
		wg       sync.WaitGroup
		once     sync.Once
	)

	setContent := func(c *pbDataFormat.Content) patillator.SegregateDiscarderFinder {
		gc := &pbMetadata.GeneralContent{
			SectionId: c.SectionId,
			Content:   c.Metadata,
		}
		return patillator.GeneralContent(*gc)
	}

	for section, _ := range h.sections {
		// query each section database concurrently; use a Mutex to synchronize
		// write access to contents.
		wg.Add(1)
		go func(section string) {
			defer wg.Done()

			ctx := &pbContext.Section{
				Id: section,
			}
			threadsOverview, err := h.GetThreadsOverview(ctx, setContent)
			if err != nil {
				log.Printf("Could not get threads overview: %v\n", err)
				errs = append(errs, err)
			}
			// Allocate map (only once) and assign the given threadsOverview
			// only if there could be gotten some thread overviews.
			if len(threadsOverview) > 0 {
				once.Do(func() {
					contents = make(map[string][]patillator.SegregateDiscarderFinder)
				})
				m.Lock()
				defer m.Unlock()
				contents[section] = threadsOverview
			}
		}(section)
	}
	wg.Wait()
	return contents, errs
}

// Get threads with the given id and section. It calls h.GetThread for each
// thread in a concurrent fashion and returns a []*pbApi.ContentRule and a []error
// composed up of the error and *pbApi.ContentRule returned by each call to
// h.GetThread.
func (h *handler) GetGeneralThreads(threadsInfo []patillator.GeneralId) ([]*pbApi.ContentRule, []error) {
	var (
		contentRules = make([]*pbApi.ContentRule, len(threadsInfo))
		errs         []error
		m            sync.Mutex
		wg           sync.WaitGroup
	)

	// query each thread concurrently; use a Mutex to synchronise write access
	// to contentRules.
	for idx, threadInfo := range threadsInfo {
		wg.Add(1)
		go func(idx int, threadInfo patillator.GeneralId) {
			defer wg.Done()

			ctx := &pbContext.Thread{
				Id: threadInfo.Id,
				SectionCtx: &pbContext.Section{
					Id: threadInfo.SectionId,
				},
			}
			contentRule, err := h.GetThread(ctx)
			if err != nil {
				contentRule = &pbApi.ContentRule{}
				m.Lock()
				errs = append(errs, err)
				m.Unlock()
			}
			contentRule.Status = threadInfo.Status
			contentRules[idx] = contentRule
		}(idx, threadInfo)
	}
	wg.Wait()
	return contentRules, errs
}

// GetThreadContent queries the section database of the given thread looking for
// the thread with the given id in both the active and archived contents bucket.
//
// If the section does not exist, it returns an ErrSectionNotFound error.
// If it found the thread, it marshals it into a *pbDataFormat.Content and
// returns it. Otherwise, it returns a nil Content and a ErrThreadNotFound error
// or a proto unmarshal error.
func (h *handler) GetThreadContent(thread *pbContext.Thread) (*pbDataFormat.Content, error) {
	var (
		err       error
		id        = thread.Id
		sectionId = thread.SectionCtx.Id
		pbContent = new(pbDataFormat.Content)
	)

	// check whether the section exists
	sectionDB, ok := h.sections[sectionId]
	if !ok {
		return nil, dbmodel.ErrSectionNotFound
	}

	err = sectionDB.contents.View(func(tx *bolt.Tx) error {
		threadBytes, err := getThreadBytes(tx, id)
		if err != nil {
			log.Printf("Could not find thread (id: %s) [root]->[%s]: %v", id, sectionId, err)
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

// GetThread queries the section database of the given thread looking for the
// thread with the given id in both the active and archived contents bucket.
//
// If the section does not exist, it returns an ErrSectionNotFound error.
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

func (h *handler) GetSavedThreadsOverview(userId string) (map[string][]patillator.SegregateDiscarderFinder, []error) {
	var (
		contents map[string][]patillator.SegregateDiscarderFinder
		errs     []error
	)

	setContent := func(c *pbDataFormat.Content) patillator.SegregateDiscarderFinder {
		gc := &pbMetadata.GeneralContent{
			SectionId: c.SectionId,
			Content:   c.Metadata,
		}
		return patillator.GeneralContent(*gc)
	}

	req := &pbUsers.SavedThreadsRequest{
		UserId: userId,
	}
	savedThreads, err := h.users.SavedThreads(context.Background(), req)

	if err != nil {
		log.Println(err)
		errs = append(errs, err)
		return nil, errs
	}
	if len(savedThreads.References) == 0 {
		log.Printf("GetSavedThreadsOverview: user %s has not saved any thread.\n", userId)
		errs = append(errs, dbmodel.ErrNoSavedThreads)
		return nil, errs
	}
	var (
		m    sync.Mutex
		once sync.Once
		wg   sync.WaitGroup
	)
	// Get and set threads metadata.
	for _, ctx := range savedThreads.References {
		wg.Add(1)
		// Do the content getting, setting and appending in its own go-routine.
		// Should it get an error and it will append it to errs. Otherwise, it
		// will format the content and append it to the section it belongs to.
		go func(ctx *pbContext.Thread) {
			defer wg.Done()
			pbContent, err := h.GetThreadContent(ctx)
			if err != nil {
				m.Lock()
				errs = append(errs, err)
				m.Unlock()
				return
			}
			content := setContent(pbContent)
			section := ctx.SectionCtx.Id
			once.Do(func() {
				contents = make(map[string][]patillator.SegregateDiscarderFinder)
			})
			m.Lock()
			contents[section] = append(contents[section], content)
			m.Unlock()
		}(ctx)
	}
	wg.Wait()
	return contents, errs
}
