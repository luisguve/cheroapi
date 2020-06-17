package bolthandler

import(

)

// Get metadata of all the active threads in a section. It returns
// ErrSectionNotFound if the section is invalid and ErrBucketNotFound if the
// section doesn't have a bucket for active contents.
func (h *handler) GetThreadsOverview(section *pbContext.Section, 
	setContent func(*pbDataFormat.Content) patillator.SegregateDiscarderFinder) ([]patillator.SegregateDiscarderFinder, error) {
	var (
		contents []patillator.SegregateDiscarderFinder
		id = section.Id
		err error
	)
	// check whether the section exists
	if sectionDB, ok := h.sections[id]; !ok {
		return nil, ErrSectionNotFound
	}

	// query database
	err = sectionDB.contents.View(func(tx *bolt.Tx) error {
		activeContents := tx.Bucket([]byte(activeContentsB)
		if activeContents == nil {
			log.Printf("bucket %s of section %s not found\n", activeContentsB, id)
			return ErrBucketNotFound
		}
		var (
			done = make(chan error)
			quit = make(chan error)
			wg sync.WaitGroup
			m sync.Mutex
			c = activeContents.Cursor()
			elems = 0
		)
		for _, v := c.First(); v != nil; _, v = c.Next() {
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
				case done<- err:
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
func (h *handler) GetThreads(section *pbContext.Section, ids []string) ([]*pbApi.ContentRule, error) {
	var (
		err error
		id = section.Id
		contentRules []*pbApi.ContentRule
	)

	// check whether the section exists
	if sectionDB, ok := h.sections[id]; !ok {
		return nil, ErrSectionNotFound
	}

	err = sectionDB.contents.View(func(tx *bolt.Tx) error {
		activeContents := tx.Bucket([]byte(activeContentsB))
		if activeContents == nil {
			return fmt.Errorf("bucket %s of section %s not found\n", activeContentsB, id)
		}

		var (
			done = make(chan error)
			quit = make(chan error)
			m sync.Mutex
			wg sync.WaitGroup
			elems = 0
		)
		for _, id = range ids {
			// Do the content querying, unmarshaling, formatting and appending
			// in its own go-routine. Should it get an error and it will send
			// it to the channel done, otherwise it will be sending nil to the
			// same channel, meaning it could complete its work successfully.
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				v := activeContents.Get([]byte(id))
				// Check whether the content was found (is currently active)
				if v != nil {
					elems++
					pbContent := new(pbDataFormat.Content)
					if err := proto.Unmarshal(v, pbContent); err != nil {
						log.Printf("Could not unmarshal content: %v\n", err)
					} else {
						contentRule := h.formatThreadContentRule(pbContent, section, id)
						m.Lock()
						contentRules = append(contentRules, contentRule)
						m.Unlock()
					}
					select {
					case done<- err:
					case <-quit: // exit in case of getting stuck on above statement.
					}
				}
			}(id)
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
// a []error and a map of section ids to the []patillator.SegregateDiscarderFinder
// that returns each call to GetThreadsOverview.
func (h *handler) GetGeneralThreadsOverview(setContent func(*pbDataFormat.Content) patillator.SegregateDiscarderFinder) 
	(map[string][]patillator.SegregateDiscarderFinder, []error) {
	var (
		contents map[string][]patillator.SegregateDiscarderFinder
		errs []error
		m sync.Mutex
		wg sync.WaitGroup
		once sync.Once
	)

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
			if len(threadsOverview > 0) {
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
		contentRules []*pbApi.ContentRule
		errs []error
		m sync.Mutex
		wg sync.WaitGroup
	)

	// query each thread concurrently; use a Mutex to synchronise write access
	// to contentRules.
	for _, threadInfo := range threadsInfo {
		wg.Add(1)
		go func(threadInfo patillator.GeneralId) {
			defer wg.Done()

			ctx := &pbContext.Thread{
				Id:         threadInfo.Id,
				SectionCtx: &pbContext.Section{
					Id: threadInfo.SectionId,
				},
			}
			m.Lock()
			defer m.Unlock()
			if contentRule, err := h.GetThread(ctx); err != nil {
				errs = append(errs, err)
			} else {
				contentRules = append(contentRules, contentRule)
			}
		}(threadInfo)
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
		err error
		id = thread.Id
		sectionId = thread.SectionCtx.Id
		pbContent = new(pbDataFormat.Content)
	)

	// check whether the section exists
	if sectionDB, ok := h.sections[sectionId]; !ok {
		return nil, ErrSectionNotFound
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

func (h *handler) GetSavedThreadsOverview(user string, 
	setContent func(metadata *pbDataFormat.Content) patillator.SegregateDiscarderFinder) (map[string][]patillator.SegregateDiscarderFinder, error) {
	var (
		contents map[string][]patillator.SegregateDiscarderFinder
		err error
		pbUser = new(pbDataFormat.User)
	)

	err = h.users.View(func(tx *bolt.Tx) error {
		usersBucket := tx.Bucket([]byte(usersB))
		if usersBucket == nil {
			log.Printf("Bucket %s of users not found\n", usersB)
			return ErrBucketNotFound
		}

		userBytes := usersBucket.Get([]byte(user))
		if userBytes == nil {
			log.Printf("User (id: %s) not found\n", user)
			return ErrUserNotFound
		}

		if err := proto.Unmarshal(userBytes, pbUser); err != nil {
			log.Printf("Could not unmarshal user: %v\n", err)
			return err
		}

		if len(pbUser.SavedThreads) == 0 {
			log.Println("This user has not saved any thread yet.")
			return ErrNoSavedThreads
		}
		return nil
	}
	if err != nil {
		// it's ok to get an ErrNoSavedThreads error
		if errors.Is(ErrNoSavedThreads) {
			return nil, nil
		}
		// if it's not such an error, it must be reported to the caller by
		// returning it.
		return nil, err
	}
	var (
		m sync.Mutex
		once sync.Once
		wg sync.WaitGroup
		done = make(chan error)
		quit = make(chan error)
		elems = 0
	)
	// get and set threads metadata
	for _, ctx := range pbUser.SavedThreads {
		elems++
		wg.Add(1)
		// Do the content getting, setting and appending in its own go-routine.
		// Should it get an error and it will send it to the channel done,
		// otherwise it will be sending nil to the same channel, meaning it
		// could complete its work successfully.
		go func(ctx *pbContext.Thread) {
			defer wg.Done()
			pbContent, err := h.GetThreadContent(ctx)
			if err == nil {
				m.Lock()
				defer m.Unlock()
				once.Do(func() {
					contents = make(map[string][]patillator.SegregateDiscarderFinder)
				})
				section := ctx.SectionCtx.Id
				content := setContent(pbContent)
				contents[section] = append(contents[section], content)
			}
			select {
			case done<- err:
			case <-quit: // exit in case of getting stuck on above statement.
			}
		}(ctx)
	}
	// Check for errors. It terminates every go-routine hung on the statement
	// "case done<- err" by closing the channel quit and returns the first err
	// read.
	for i := 0; i < elems; i++ {
		err = <-done
		if err != nil {
			close(quit)
			break
		}
	}
	wg.Wait()
	return contents, err
}
