package bolt

import (
	"time"
	"log"
	"sync"

	"github.com/golang/protobuf/proto"
	dbmodel "github.com/luisguve/cheroapi/internal/app/cheroapi"
	bolt "go.etcd.io/bbolt"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
	pbContext"github.com/luisguve/cheroproto-go/context"
)

// Return the last time a clean up was done.
func (h *handler) LastQA() int64 {
	return h.lastQA
}

// Clean up every section database.
// 
// It moves unpopular contents from the bucket of active contents to the bucket
// of archived contents. It will only test the relevance of threads with 1 day
// or longer and move them accordingly, along with its comments and subcomments.
// 
// It will also move comments and subcomments of deleted threads to the bucket
// of archived contents, even if the thread has been around for less than one
// day.
// 
// In addition to moving the contents to the bucket of archived contents, it also
// updates the activity of the users involved, moving contexts from the list of
// recent activity of the users to their list of old activity.
func (h *handler) QA() {
	now := time.Now()
	for _, s := range h.sections {
		go func(s section) {
			s.contents.View(func(tx *bolt.Tx) error {
				activeContents := tx.Bucket([]byte(activeContentsB))
				if activeContents == nil {
					log.Printf("Could not find bucket %s\n", activeContentsB)
					return dbmodel.ErrBucketNotFound
				}
				// Iterate over every key/value pair of active contents.
				var (
					c = activeContents.Cursor()
					wg sync.WaitGroup
				)
				for k, v := c.First(); k != nil; k, v = c.Next() {
					// Check whether the value is a nested bucket. If so, just continue.
					// Cursors see nested buckets with value == nil.
					if v == nil {
						continue
					}
					pbThread := new(pbDataFormat.Content)
					if err := proto.Unmarshal(v, pbThread); err != nil {
						log.Printf("Could not unmarshal content %s: %v\n", string(k), err)
						return err
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
					min := 1 * time.Hour
					if (m.Interactions > 100) && (avgUpdateTime <= min.Seconds()) {
						continue
					}
					// Otherwise, it will be moved to the bucket of archived contents for
					// read only, along with the contents associated to it; comments and
					// subcomments.
					wg.Add(1)
					go h.moveContents(s, k, v, wg)
				}
				// Move comments and subcomments associated to deleted threads.
				deletedContents := activeContents.Bucket([]byte(deletedThreadsB))
				if deletedContents == nil {
					log.Printf("Could not find bucket %s\n", deletedThreadsB)
					return dbmodel.ErrBucketNotFound
				}
				c = deletedContents.Cursor()
				for k, v := c.First(); k != nil; k, v = c.Next() {
					// Check whether the value is a nested bucket. If so, just continue.
					// Cursors see nested buckets with value == nil.
					if v == nil {
						continue
					}
					wg.Add(1)
					go h.deleteThread(s, k, wg)
				}
				wg.Wait()
				return nil
			})
		}(s)
	}
}

// Update the given section by moving the thread, its comments and subcomments
// from the bucket of active contents to the bucket of archived contents.
// 
// It will copy the contents in almost exactly the same format as in the bucket
// of active contents; the only difference is that the resulting structure will
// not have any bucket that registers deleted content.
// 
// Note that it will also move all of the subcomments of the deleted comments,
// if any, to the bucket of archived contents under the same Id of the deleted
// comment.
func (h *handler) moveContents(s section, threadId, threadBytes []byte, wg sync.WaitGroup) {
	defer wg.Done()
	s.contents.Update(func(tx *bolt.Tx) error {
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
		var (
			done = make(chan error)
			quit = make(chan error)
			count = 0
		)
		defer close(quit)
		// Put thread into archived contents.
		count += 2 // Two go-routines will be launched.
		go func() {
			pbContent := new(pbDataFormat.Content)
			err := proto.Unmarshal(threadBytes, pbContent)
			if  err == nil {
				ctx := &pbContext.Thread{
					Id:         string(threadId),
					SectionCtx: &pbContext.Section{
						Id: pbContent.SectionId,
					},
				}
				userId := pbContent.AuthorId
				go h.markThreadAsOld(userId, ctx, done, quit)
				err = archivedContents.Put(threadId, threadBytes)
			}
			select {
			case done<- err:
			case <-quit:
			}
		}()

		commentsBucket := activeContents.Bucket([]byte(commentsB))
		if commentsBucket == nil {
			log.Printf("Could not find bucket %s\n", commentsB)
			return dbmodel.ErrBucketNotFound
		}
		// Check whether there are comments and move them to archived contents.
		if actComments := commentsBucket.Bucket(threadId); actComments != nil {
			// Get comments bucket in archived contents.
			commentsBucket = archivedContents.Bucket([]byte(commentsB))
			if commentsBucket == nil {
				log.Printf("Bucket %s not found\n", commentsB)
				return dbmodel.ErrBucketNotFound
			}
			archComments, err := commentsBucket.CreateBucketIfNotExists(threadId)
			if err != nil {
				log.Printf("Could not create archived comments bucket %s: %v\n", threadId, err)
				return err
			}
			count++
			go func() {
				err := h.moveComments(string(threadId), actComments, archComments)
				select {
				case done<- err:
				case <-quit:
				}
			}()
			// Check whether there are subcomments and move them to archived
			// contents.
			actSubcomKeys := actComments.Bucket([]byte(subcommentsB))
			if actSubcomKeys != nil {
				archSubcomKeys, err := archComments.CreateBucketIfNotExists([]byte(subcommentsB))
				if err != nil {
					log.Printf("Could not create bucket %s: %v\n", subcommentsB, err)
					return err
				}
				count++
				go func() {
					err := h.moveSubcomments(actSubcomKeys, archSubcomKeys)
					select {
					case done<- err:
					case <-quit:
					}
				}()
			}
			// Now the comments and subcomments are in the bucket of archived
			// contents, with exactly the same structure as it were in the bucket
			// of active contents: under the commentsB bucket in a bucket with
			// the same key as the thread id they belong to.
			// 
			// Get bucket of comments from active contents again, since
			// commentsBucket was set to the comments bucket from archived
			// contents.
			commentsBucket = activeContents.Bucket([]byte(commentsB))
			if commentsBucket == nil {
				log.Printf("Could not find bucket %s\n", commentsB)
				return dbmodel.ErrBucketNotFound
			}
			// Delete the comments and subcomments from the bucket of active
			// contents. Deleting the comments bucket will also delete the
			// subcomments bucket.
			if err = commentsBucket.DeleteBucket(threadId); err != nil {
				log.Printf("Could not DEL comments bucket of thread %s: %v\n", string(threadId), err)
				return err
			}
		}
		// Finally, delete thread from active contents.
		if err := activeContents.Delete(threadId); err != nil {
			log.Printf("Could not DEL thread from active contents: %v\n", err)
			return err
		}
		// Check for errors.
		for i := 0; i < count; i++ {
			if err := <-done; err != nil {
				log.Println(err)
				return err
			}
		}
		return nil
	})
}

// Move comments and subcomments associated to the given thread, which has been
// deleted, to the bucket of archived contents under the thread id as the key,
// then remove the reference to the deleted thread from the bucket of deleted
// contents.
func (h *handler) deleteThread(s section, threadId []byte, wg sync.WaitGroup) error {
	defer wg.Done()
	return s.contents.Update(func(tx *bolt.Tx) error {
		// These buckets must have been defined.
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
		deletedContents := activeContents.Bucket([]byte(deletedThreadsB))
		if deletedContents == nil {
			log.Printf("Could not find bucket %s\n", deletedThreadsB)
			return dbmodel.ErrBucketNotFound
		}
		commentsBucket := activeContents.Bucket([]byte(commentsB))
		if commentsBucket == nil {
			log.Printf("Could not find bucket %s\n", commentsB)
			return dbmodel.ErrBucketNotFound
		}

		// Check whether there are comments and move them to archived contents.
		if actComments := commentsBucket.Bucket(threadId); actComments != nil {
			// Get comments bucket in archived contents.
			commentsBucket = archivedContents.Bucket([]byte(commentsB))
			if commentsBucket == nil {
				log.Printf("Bucket %s not found\n", commentsB)
				return dbmodel.ErrBucketNotFound
			}
			archComments, err := commentsBucket.CreateBucketIfNotExists(threadId)
			if err != nil {
				log.Printf("Could not create archived comments bucket %s: %v\n", threadId, err)
				return err
			}
			var (
				count = 1 // At least 1 go-routine will be launched.
				done = make(chan error)
				quit = make(chan error)
			)
			defer close(quit)
			go func() {
				err := h.moveComments(string(threadId), actComments, archComments)
				select {
				case done<- err:
				case <-quit:
				}
			}()
			// Check whether there are subcomments and move them to archived
			// contents.
			actSubcomKeys := actComments.Bucket([]byte(subcommentsB))
			if actSubcomKeys != nil {
				archSubcomKeys, err := archComments.CreateBucketIfNotExists([]byte(subcommentsB))
				if err != nil {
					log.Printf("Could not create bucket %s: %v\n", subcommentsB, err)
					return err
				}
				count++
				go func() {
					err := h.moveSubcomments(actSubcomKeys, archSubcomKeys)
					select {
					case done<- err:
					case <-quit:
					}
				}()
			}
			// Check for errors. No need to close quit channel since it was
			// deferred.
			for i := 0; i < count; i++ {
				err = <-done
				if err != nil {
					log.Println(err)
					return err
				}
			}
			// Now the comments and subcomments are in the bucket of archived
			// contents, with exactly the same structure as it were in the bucket
			// of active contents: under the commentsB bucket in a bucket with
			// the same key as the thread id they belong to.
			// 
			// Get bucket of comments from active contents again, since
			// commentsBucket was set to the comments bucket from archived
			// contents.
			commentsBucket = activeContents.Bucket([]byte(commentsB))
			if commentsBucket == nil {
				log.Printf("Could not find bucket %s\n", commentsB)
				return dbmodel.ErrBucketNotFound
			}
			// Delete the comments and subcomments from the bucket of active
			// contents. Deleting the comments bucket will also delete the
			// subcomments bucket.
			if err = commentsBucket.DeleteBucket(threadId); err != nil {
				log.Printf("Could not DEL comments bucket of thread %s: %v\n", string(threadId), err)
				return err
			}
		}
		// Finally, delete thread from deleted contents.
		if err := deletedContents.Delete(threadId); err != nil {
			log.Printf("Could not DEL thread from deleted contents: %v\n", err)
			return err
		}
		return nil
	})
}

// Move the comments associated to the given thread, from actComments to
// archComments.
func (h *handler) moveComments(threadId string, actComments, archComments *bolt.Bucket) error {
	// Put comments from active comments into archived comments.
	var (
		count = 0
		c = actComments.Cursor()
		done = make(chan error)
		quit = make(chan error)
	)
	for k, v := c.First(); k != nil; k, v = c.Next() {
		// Check whether the value is a nested bucket. If so, just continue.
		// Cursors see nested buckets with value == nil.
		if v == nil {
			continue
		}
		count += 2 // Two go-routines will be launched.
		go func(k, v []byte) {
			pbContent := new(pbDataFormat.Content)
			err := proto.Unmarshal(v, pbContent)
			if err == nil {
				ctx := &pbContext.Comment{
					Id:        string(k),
					ThreadCtx: &pbContext.Thread{
						Id:         pbContent.Id,
						SectionCtx: &pbContext.Section{
							Id: pbContent.SectionId,
						},
					},
				}
				userId := pbContent.AuthorId
				go h.markCommentAsOld(userId, ctx, done, quit)
				err = archComments.Put(k, v)
			}
			select {
			case done<- err:
			case <-quit:
			}
		}(k, v)
	}
	// Check for errors. It terminates every go-routine hung on the statement
	// "case done<- err:" by closing the channel quit and returns the first err
	// received.
	for i := 0; i < count; i++ {
		err := <-done
		if err != nil {
			close(quit)
			return err
		}
	}
	return nil
}

// Move subcoments from every bucket in actSubcomKeys to a new bucket with the
// same key in archSubcomKeys.
func (h *handler) moveSubcomments(actSubcomKeys, archSubcomKeys *bolt.Bucket) error {
	var (
		count = 0
		c = actSubcomKeys.Cursor()
		done = make(chan error)
		quit = make(chan error)
	)
	// actSubcomKeys bucket only holds nested buckets, hence the values are
	// discarded and the keys are used to find the buckets, which store the
	// actual active subcomments.
	for comKey, _ := c.First(); comKey != nil; comKey, _ = c.Next() {
		activeSubcom := actSubcomKeys.Bucket(comKey)
		if activeSubcom == nil {
			log.Printf("Could not FIND subcomments bucket %s\n", string(comKey))
			continue
		}
		// Create the corresponding bucket in archSubcomKeys.
		archivedSubcom, err := archSubcomKeys.CreateBucketIfNotExists(comKey)
		if err != nil {
			log.Printf("Could not create bucket %s: %v\n", string(comKey), err)
			return err
		}
		// Put subcomments from active comments into archived comments.
		subcomCursor := activeSubcom.Cursor()
		for k, v := subcomCursor.First(); k != nil; k, v = subcomCursor.Next() {
			count += 2 // Two go-routines will be launched.
			go func(k, v []byte) {
				pbContent := new(pbDataFormat.Content)
				if err := proto.Unmarshal(v, pbContent); err == nil {
					ctx := &pbContext.Subcomment{
						Id: string(k),
						CommentCtx: &pbContext.Comment{
							Id:        string(comKey),
							ThreadCtx: &pbContext.Thread{
								Id:         pbContent.Id,
								SectionCtx: &pbContext.Section{
									Id: pbContent.SectionId,
								},
							},
						},
					}
					userId := pbContent.AuthorId
					go h.markSubcommentAsOld(userId, ctx, done, quit)
					err = archivedSubcom.Put(k, v)
				}
				select {
				case done<- err:
				case <-quit:
				}
			}(k, v)
		}
	}
	// Check for errors. It terminates every go-routine hung on the statement
	// "case done<- err:" by closing the channel quit and returns the first err
	// received.
	for i := 0; i < count; i++ {
		err := <-done
		if err != nil {
			close(quit)
			return err
		}
	}
	return nil
}

func (h *handler) markThreadAsOld(userId string, ctx *pbContext.Thread, done, quit chan error) {
	var (
		id = ctx.Id
		sectionId = ctx.SectionCtx.Id
		found bool
	)
	pbUser, err := h.User(userId)
	if err == nil {
		if pbUser.RecentActivity != nil {
			// Find and copy thread from recent activity to old activity of
			// the user, then remove it from recent activity.
			for i, t := range pbUser.RecentActivity.ThreadsCreated {
				if (t.SectionCtx.Id == sectionId) && (t.Id == id) {
					found = true
					if pbUser.OldActivity == nil {
						pbUser.OldActivity = new(pbDataFormat.Activity)
					}
					// Append to old activity.
					tc := pbUser.OldActivity.ThreadsCreated
					pbUser.OldActivity.ThreadsCreated = append(tc, t)
					// Remove from recent activity.
					last := len(pbUser.RecentActivity.ThreadsCreated) - 1
					pbUser.RecentActivity.ThreadsCreated[i] = pbUser.RecentActivity.ThreadsCreated[last]
					pbUser.RecentActivity.ThreadsCreated = pbUser.RecentActivity.ThreadsCreated[:last]
					break
				}
			}
			if found {
				err = h.UpdateUser(pbUser, userId)
			} else {
				log.Printf("Could not find thread %v\n", ctx)
			}
		}
	}
	select {
	case done<- err:
	case <-quit:
	}
}

func (h *handler) markCommentAsOld(userId string, ctx *pbContext.Comment, done, quit chan error) {
	var (
		id = ctx.Id
		threadId = ctx.ThreadCtx.Id
		sectionId = ctx.ThreadCtx.SectionCtx.Id
		found bool
	)
	pbUser, err := h.User(userId)
	if err == nil {
		if pbUser.RecentActivity != nil {
			// Find and copy comment from recent activity to old activity of
			// the user, then remove it from recent activity.
			for i, c := range pbUser.RecentActivity.Comments {
				if (c.ThreadCtx.SectionCtx.Id == sectionId) &&
				(c.ThreadCtx.Id == threadId) &&
				(c.Id == id) {
					found = true
					if pbUser.OldActivity == nil {
						pbUser.OldActivity = new(pbDataFormat.Activity)
					}
					// Append to old activity.
					comments := pbUser.OldActivity.Comments
					pbUser.OldActivity.Comments = append(comments, c)
					// Remove from recent activity.
					last := len(pbUser.RecentActivity.Comments) - 1
					pbUser.RecentActivity.Comments[i] = pbUser.RecentActivity.Comments[last]
					pbUser.RecentActivity.Comments = pbUser.RecentActivity.Comments[:last]
					break
				}
			}
			if found {
				err = h.UpdateUser(pbUser, userId)
			} else {
				log.Printf("Could not find comment %v\n", ctx)
			}
		}
	}
	select {
	case done<- err:
	case <-quit:
	}
}

func (h *handler) markSubcommentAsOld(userId string, ctx *pbContext.Subcomment, done, quit chan error) {
	var (
		id = ctx.Id
		commentId = ctx.CommentCtx.Id
		threadId = ctx.CommentCtx.ThreadCtx.Id
		sectionId = ctx.CommentCtx.ThreadCtx.SectionCtx.Id
		found bool
	)
	pbUser, err := h.User(userId)
	if err == nil {
		if pbUser.RecentActivity != nil {
			// Find and copy thread from recent activity to old activity of
			// the user, then remove it from recent activity.
			for i, s := range pbUser.RecentActivity.Subcomments {
				if (s.CommentCtx.ThreadCtx.SectionCtx.Id == sectionId) &&
				(s.CommentCtx.ThreadCtx.Id == threadId) &&
				(s.CommentCtx.Id == commentId) && 
				(s.Id == id) {
					found = true
					if pbUser.OldActivity == nil {
						pbUser.OldActivity = new(pbDataFormat.Activity)
					}
					// Append to old activity.
					subcomments := pbUser.OldActivity.Subcomments
					pbUser.OldActivity.Subcomments = append(subcomments, s)
					// Remove from recent activity.
					last := len(pbUser.RecentActivity.Subcomments) - 1
					pbUser.RecentActivity.Subcomments[i] = pbUser.RecentActivity.Subcomments[last]
					pbUser.RecentActivity.Subcomments = pbUser.RecentActivity.Subcomments[:last]
					break
				}
			}
			if found {
				err = h.UpdateUser(pbUser, userId)
			} else {
				log.Printf("Could not find subcomment %v\n", ctx)
			}
		}
	}
	select {
	case done<- err:
	case <-quit:
	}
}
