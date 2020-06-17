package bolthandler

import(
	"google.golang.org/protobuf/proto"
)

// Get metadata of comments in a thread. It returns ErrSectionNotFound if the
// section is invalid and ErrBucketNotFound if the section doesn't have a bucket
// for active contents or a bucket for archived contents, depending upon the
// current status of the thread which the comments belongs to.
func (h *handler) GetCommentsOverview(thread *pbContext.Thread, 
	setContent func(*pbDataFormat.Content) patillator.SegregateDiscarderFinder) ([]patillator.SegregateDiscarderFinder, error) {
	var (
		err error
		id = thread.Id
		sectionId = thread.SectionCtx.Id
		contents []patillator.SegregateDiscarderFinder
	)
	// check whether the section exists
	if sectionDB, ok := h.sections[sectionId]; !ok {
		return nil, ErrSectionNotFound
	}

	// query database
	err = sectionDB.contents.View(func(tx *bolt.Tx) error {
		comments, err := getCommentsBucket(tx, id)
		if err != nil {
			log.Printf("There are no comments for the thread %s. %v\n", id, err)
			return ErrNoComments
		}

		var (
			c = comments.Cursor()
			m sync.Mutex
			wg sync.WaitGroup
			quit = make(chan error)
			done = make(chan error)
			elems = 0
		)
		for _, v := c.First(); v != nil; _, v = c.Next() {
			elems++
			wg.Add(1)
			// Do the unmarshaling, content setting and content appending in its
			// own go-routine. Should it get an error and it will send it to the
			// channel done, otherwise it will be sending nil to the same channel,
			// meaning it could complete its work successfully.
			go func(commentBytes []byte) {
				defer wg.Done()
				pbContent := new(pbDataFormat.Content)
				if err := proto.Unmarshal(commentBytes, pbContent); err != nil {
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

// Get content of the given comment ids in a thread
func (h *handler) GetComments(thread *pbContext.Thread, ids []string) ([]*pbApi.ContentRule, error) {
	var (
		err error
		id = thread.Id
		sectionId = thread.SectionCtx.Id
		contentRules []*pbApi.ContentRule
	)

	// check whether the section exists
	if sectionDB, ok := h.sections[sectionId]; !ok {
		return nil, ErrSectionNotFound
	}

	err = sectionDB.contents.View(func(tx *bolt.Tx) error {
		comments, err := getCommentsBucket(tx, id)
		if err != nil {
			log.Printf("There are no comments for the thread %s. %v\n", id, err)
			return ErrNoComments
		}
		var (
			wg sync.WaitGroup
			m sync.Mutex
			elems = 0
			done = make(chan error)
			quit = make(chan error)
		)
		for _, id = range ids {
			// Do the content querying, unmarshaling, formatting and appending
			// in its own go-routine. Should it get an error and it will send
			// it to the channel done, otherwise it will be sending nil to the
			// same channel, meaning it could complete its work successfully.
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				v := comments.Get([]byte(id))
				// Check whether the comment exists
				if v != nil {
					elems++
					pbContent := new(pbDataFormat.Content)
					if err := proto.Unmarshal(v, pbContent); err != nil {
						log.Printf("Could not unmarshal content: %v\n", err)
					} else {
						contentRule := h.formatCommentContentRule(pbContent, section, id)
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

// GetCommentContent queries the section database of the given comment looking
// for the comment with the given id in both the active and archived contents
// bucket.
// 
// If the section does not exist, it returns an ErrSectionNotFound error.
// If it found the comment, it marshals it into a *pbDataFormat.Content and
// returns it. Otherwise, it returns a nil Content and an ErrCommentNotFound or
// a proto unmarshal error.
func (h *handler) GetCommentContent(comment *pbContext.Comment) (*pbDataFormat.Content, error) {
	var (
		err error
		id = comment.Id
		threadId = comment.ThreadCtx.Id
		sectionId = comment.ThreadCtx.SectionCtx.Id
		pbContent = new(pbDataFormat.Content)
	)

	// check whether the section exists
	if sectionDB, ok := h.sections[sectionId]; !ok {
		return nil, ErrSectionNotFound
	}

	err = sectionDB.contents.View(func(tx *bolt.Tx) error {
		commentBytes, err := getCommentBytes(tx, threadId, id)
		if err != nil {
			log.Printf("Could not find comment (id: %s) [root]->[%s]->[%s]: %v",
			id, sectionId, threadId, err)
			return err
		}

		if err = proto.Unmarshal(commentBytes, pbContent); err != nil {
			log.Printf("Could not unmarshal content: %v\n", err)
			return err
		}
		return nil
	})
	return pbContent, err
}

// GetSubcommentContent queries the section database of the given subcomment
// looking for the subcomment with the given id in both the active and archived
// contents bucket.
// 
// If the section does not exist, it returns an ErrSectionNotFound error.
// If it found the subcomment, it marshals it into a *pbDataFormat.Content and
// returns it. Otherwise, it returns a nil Content and an error.
func (h *handler) GetSubcommentContent(subcomment *pbContext.Subcomment) (*pbDataFormat.Content, error) {
	var (
		err error
		id = subcomment.Id
		commentId = subcomment.CommentCtx.Id
		threadId = subcomment.CommentCtx.ThreadCtx.Id
		sectionId = subcomment.CommentCtx.ThreadCtx.SectionCtx.Id
		pbContent = new(pbDataFormat.Content)
	)

	// check whether the section exists
	if sectionDB, ok := h.sections[sectionId]; !ok {
		return nil, ErrSectionNotFound
	}

	err = sectionDB.contents.View(func(tx *bolt.Tx) error {
		threadBytes, err := getSubcommentBytes(tx, threadId, commentId, id)
		if err != nil {
			log.Printf("Could not find subcomment (id: %s) [root]->[%s]->[%s]->[%s]: %v",
			id, sectionId, threadId, commentId, err)
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

// GetComment queries the section database of the given comment looking for the
// comment with the given id in both the active and archived contents bucket.
// 
// If the section does not exist, it returns an ErrSectionNotFound error.
// If it found the comment, it converts it into a ContentRule and returns it.
// Otherwise, it returns a nil ContentRule and an error.
func (h *handler) GetComment(comment *pbContext.Comment) (*pbApi.ContentRule, error) {
	pbContent, err := h.GetCommentContent(comment)
	if err != nil {
		return nil, err
	}

	contentRule := h.formatCommentContentRule(pbContent, comment.ThreadCtx, comment.Id)
	return contentRule, nil
}

// GetSubcomment queries the section database of the given subcomment looking for
// the subcomment with the given id in both the active and archived contents bucket.
// 
// If the section does not exist, it returns an ErrSectionNotFound error.
// If it found the subcomment, it converts it into a ContentRule and returns it.
// Otherwise, it returns a nil ContentRule and an error.
func (h *handler) GetSubcomment(subcomment *pbContext.Subcomment) (*pbApi.ContentRule, error) {
	pbContent, err := h.GetSubcommentContent(subcomment)
	if err != nil {
		return nil, err
	}

	contentRule := h.formatSubcommentContentRule(pbContent, subcomment.CommentCtx, subcomment.Id)
	return contentRule, nil
}
