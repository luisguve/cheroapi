package bolt

import (
	"fmt"
	"log"
	"time"

	"github.com/golang/protobuf/proto"
	dbmodel "github.com/luisguve/cheroapi/internal/app/cheroapi"
	pbContext "github.com/luisguve/cheroproto-go/context"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
	bolt "go.etcd.io/bbolt"
)

type resultErr struct {
	err    error
	result string
}

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
// 
// It returns the result of moving the contents in a string and an error.
func (h *handler) QA() (string, error) {
	var (
		summary string
		numGR int
		done = make(chan resultErr)
		quit = make(chan struct{})
		now = time.Now()
	)
	defer close(quit)
	summary = fmt.Sprintf("[%v] Starting QA.\n", now.Format(time.Stamp))
	for _, s := range h.sections {
		numGR++
		go func(s section) {
			var (
				sectionSummary string
				localDone = make(chan resultErr)
				localQuit = make(chan struct{})
				numGR       int
			)
			defer close(localQuit)
			sectionSummary = fmt.Sprintf("\t[%v]\n", s.name)
			err := s.contents.View(func(tx *bolt.Tx) error {
				activeContents := tx.Bucket([]byte(activeContentsB))
				if activeContents == nil {
					log.Printf("Could not find bucket %s\n", activeContentsB)
					return dbmodel.ErrBucketNotFound
				}
				// Iterate over every key/value pair of active contents.
				var (
					c = activeContents.Cursor()
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
						sectionSummary += fmt.Sprintln("-----------------------------------------------------")
						sectionSummary += fmt.Sprintf("%s has been around for less than one day, ", pbThread.Title)
						sectionSummary += fmt.Sprintf("hence it is not a candidate for moving to archived contents.\n")
						continue
					}
					m := pbThread.Metadata

					lastUpdated := time.Unix(m.LastUpdated.Seconds, 0)

					diff = now.Sub(lastUpdated)
					diff += time.Duration(m.Diff) * time.Second

					var avgUpdateTime float64
					if m.Interactions > 0 {
						avgUpdateTime = diff.Seconds() / float64(m.Interactions)
					} else {
						avgUpdateTime = diff.Seconds()
					}
					// Check whether the thread is still relevant. It should have more
					// than 100 interactions and the average time difference between
					// interactions must be no longer than 1 hour.
					// If so, it will be skipped.
					min := 1 * time.Hour
					if (m.Interactions > 49) && (avgUpdateTime <= min.Seconds()) {
						sectionSummary += fmt.Sprintln("-----------------------------------------------------")
						sectionSummary += fmt.Sprintf("With an average update time difference of %v (< %v), ", avgUpdateTime, min.Seconds())
						sectionSummary += fmt.Sprintf("and a total of %v interactions (> 49), %s keeps active.\n", m.Interactions, 
							pbThread.Title)
						continue
					}
					// Otherwise, it will be moved to the bucket of archived contents,
					// along with the contents associated to it; comments and
					// subcomments.
					copyKey := make([]byte, len(k))
					copy(copyKey, k)
					copyVal := make([]byte, len(v))
					copy(copyVal, v)
					numGR++
					go func(k, v []byte, c *pbDataFormat.Content, avgUpdateTime float64) {
						var (
							result string
							resErr resultErr
						)
						resErr.result += fmt.Sprintln("-----------------------------------------------------")
						resErr.result += fmt.Sprintf("With an average update time difference of %v, ", avgUpdateTime)
						resErr.result += fmt.Sprintf("and a total of %v interactions, %s will be moved to archived contents.\n", 
							c.Metadata.Interactions, pbThread.Title)
						result, resErr.err = h.moveContents(s, k, v, c)
						resErr.result += result
						select {
						case localDone<- resErr:
						case <-localQuit:
						}
					}(copyKey, copyVal, pbThread, avgUpdateTime)
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
					copyKey := make([]byte, len(k))
					copy(copyKey, k)
					numGR++
					go func(k []byte) {
						var resErr resultErr
						resErr.result, resErr.err = h.deleteThread(s, k)
						select {
						case localDone<- resErr:
						case <-localQuit:
						}
					}(copyKey)
				}
				return nil
			})
			if err != nil {
				if numGR == 0 {
					// Nothing to do; there are no summaries to receive.
					select {
					case done<- resultErr{err: err}:
					case <-quit:
					}
					return
				}
				// There are summaries to receive, print the error and continue.
				log.Println(err)
			}
			var (
				resErr resultErr
				// Flag to indicate that an error was found and that no more
				// errors will be checked.
				foundErr bool
			)
			// Check for errors, concatenate the resulting summaries and send
			// the result along with the first error, if any, to the outer
			// go-routine.
			for i := 0; i < numGR; i++ {
				resErr = <-localDone
				if resErr.result != "" {
					sectionSummary += resErr.result
				}
				if !foundErr {
					if resErr.err != nil {
						foundErr = true
						err = resErr.err
					}
				}
			}
			resErr.result = sectionSummary
			resErr.err = err
			select {
			case done<- resErr:
			case <-quit:
			}
		}(s)
	}
	var (
		resErr resultErr
		err error
		// Flag to indicate that an error was found and that no more errors
		// will be checked.
		foundErr bool
	)
	// Check for errors, concatenate the resulting summaries and return the
	// result along with the first error, if any.
	for i := 0; i < numGR; i++ {
		resErr = <-done
		if resErr.result != "" {
			summary += resErr.result
		}
		if !foundErr {
			if resErr.err != nil {
				foundErr = true
				err = resErr.err
			}
		}
	}
	return summary, err
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
func (h *handler) moveContents(s section, threadId, threadBytes []byte, pbContent *pbDataFormat.Content) (string, error) {
	var (
		result string
		err    error
	)
	result += fmt.Sprintf("Moving %s to archived contents... ", threadId)
	err = s.contents.Update(func(tx *bolt.Tx) error {
		activeContents := tx.Bucket([]byte(activeContentsB))
		if activeContents == nil {
			result += fmt.Sprintf("\nCould not find bucket %s. Contents moving aborted.\n", activeContentsB)
			return dbmodel.ErrBucketNotFound
		}
		archivedContents := tx.Bucket([]byte(archivedContentsB))
		if archivedContents == nil {
			result += fmt.Sprintf("\nCould not find bucket %s. Contents moving aborted.\n", archivedContentsB)
			return dbmodel.ErrBucketNotFound
		}
		var (
			done  = make(chan resultErr)
			quit  = make(chan struct{})
			numGR = 0
		)
		defer close(quit)
		// Update activity of user. This can be done in another go-routine.
		numGR++
		go func() {
			ctx := &pbContext.Thread{
				Id: string(threadId),
				SectionCtx: &pbContext.Section{
					Id: pbContent.SectionId,
				},
			}
			userId := pbContent.AuthorId
			var resErr resultErr
			err := h.markThreadAsOld(userId, ctx)
			if err != nil {
				resErr.result = fmt.Sprintf("Could not mark thread %s as old to user %s: %v. Contents moving aborted.\n", 
					ctx.Id, userId, resErr.err)
				resErr.err = err
			} else {
				resErr.result = fmt.Sprintln("Updated thread author's activity successfully.")
			}
			select {
			case done<- resErr:
			case <-quit:
			}
		}()

		// Put thread into archived contents.
		err = archivedContents.Put(threadId, threadBytes)
		if err != nil {
			result += fmt.Sprintf("\nCould not put thread %s into archived contents: %v.\n", threadId, err)
			return err
		}
		result += "Done.\n"

		// Check whether there are comments and move them to archived contents.
		commentsBucket := activeContents.Bucket([]byte(commentsB))
		if commentsBucket == nil {
			result += fmt.Sprintf("Could not find bucket %s\n", commentsB)
			return dbmodel.ErrBucketNotFound
		}
		if actComments := commentsBucket.Bucket(threadId); actComments != nil {
			result += fmt.Sprintf("Found %d comments in thread %s to move.\n", actComments.Stats().KeyN, threadId)
			// Get comments bucket in archived contents.
			commentsBucket = archivedContents.Bucket([]byte(commentsB))
			if commentsBucket == nil {
				result += fmt.Sprintf("Bucket %s not found. Contents moving aborted.\n", commentsB)
				return dbmodel.ErrBucketNotFound
			}
			archComments, err := commentsBucket.CreateBucketIfNotExists(threadId)
			if err != nil {
				result += fmt.Sprintf("Could not create archived comments bucket %s: %v. Contents moving aborted.\n", threadId, err)
				return err
			}
			resErr := h.moveComments(string(threadId), actComments, archComments)
			if resErr.result != "" {
				result += resErr.result
			}
			if resErr.err != nil {
				// Abort contents moving.
				return err
			}

			// Check whether there are subcomments and move them to archived contents.
			actSubcomKeys := actComments.Bucket([]byte(subcommentsB))
			if actSubcomKeys != nil {
				result += fmt.Sprintf("Moving subcomments from %d comments.\n", actSubcomKeys.Stats().BucketN)
				archSubcomKeys, err := archComments.CreateBucketIfNotExists([]byte(subcommentsB))
				if err != nil {
					result += fmt.Sprintf("Could not create bucket %s: %v. Contents moving aborted.\n", subcommentsB, err)
					return err
				}
				resErr := h.moveSubcomments(actSubcomKeys, archSubcomKeys)
				if resErr.result != "" {
					result += resErr.result
				}
				if resErr.err != nil {
					// Abort contents moving.
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
				result += fmt.Sprintf("Could not find bucket %s. Contents moving aborted.\n", commentsB)
				return dbmodel.ErrBucketNotFound
			}
			// Delete the comments and subcomments from the bucket of active
			// contents. Deleting the comments bucket will also delete the
			// subcomments bucket.
			if err = commentsBucket.DeleteBucket(threadId); err != nil {
				result += fmt.Sprintf("Could not DEL comments bucket of thread %s: %v. Contents moving aborted.\n", threadId, err)
				return err
			}
		}
		// Finally, delete thread from active contents.
		if err := activeContents.Delete(threadId); err != nil {
			result += fmt.Sprintf("Could not DEL thread from active contents: %v. Contents moving aborted.\n", err)
			return err
		}
		var (
			resErr resultErr
			err error
			foundErr bool
		)
		// Check for errors. All the results will be concatenated but only the first
		// err read, if any, will be returned.
		for i := 0; i < numGR; i++ {
			resErr = <-done
			if resErr.result != "" {
				result += resErr.result
			}
			if !foundErr {
				if resErr.err != nil {
					foundErr = true
					err = resErr.err
				}
			}
		}
		return err
	})
	return result, err
}

// Move comments and subcomments associated to the given thread, which has been
// deleted, to the bucket of archived contents under the thread id as the key,
// then remove the reference to the deleted thread from the bucket of deleted
// contents.
func (h *handler) deleteThread(s section, threadId []byte) (string, error) {
	var (
		result string
		err    error
	)
	err = s.contents.Update(func(tx *bolt.Tx) error {
		// These buckets must have been defined on setup.
		activeContents := tx.Bucket([]byte(activeContentsB))
		if activeContents == nil {
			result += fmt.Sprintf("Could not find bucket %s.\n", activeContentsB)
			return dbmodel.ErrBucketNotFound
		}
		archivedContents := tx.Bucket([]byte(archivedContentsB))
		if archivedContents == nil {
			result += fmt.Sprintf("Could not find bucket %s.\n", archivedContentsB)
			return dbmodel.ErrBucketNotFound
		}
		deletedContents := activeContents.Bucket([]byte(deletedThreadsB))
		if deletedContents == nil {
			result += fmt.Sprintf("Could not find bucket %s.\n", deletedThreadsB)
			return dbmodel.ErrBucketNotFound
		}
		commentsBucket := activeContents.Bucket([]byte(commentsB))
		if commentsBucket == nil {
			result += fmt.Sprintf("Could not find bucket %s.\n", commentsB)
			return dbmodel.ErrBucketNotFound
		}

		result = fmt.Sprintln("-----------------------------------------------------")
		result += fmt.Sprintf("Removing reference to deleted thread %s... ", threadId)
		if err := deletedContents.Delete(threadId); err != nil {
			result += fmt.Sprintf("\nCould not DEL thread from deleted contents: %v. Aborting contents moving.\n", err)
			return err
		}
		result += "Done.\n"

		// Check whether there are comments and move them to archived contents.
		if actComments := commentsBucket.Bucket(threadId); actComments != nil {
			result += fmt.Sprintf("Found %d comments in (deleted) thread %s to move.\n", actComments.Stats().KeyN, threadId)
			// Get comments bucket in archived contents.
			commentsBucket = archivedContents.Bucket([]byte(commentsB))
			if commentsBucket == nil {
				result += fmt.Sprintf("Bucket %s not found.\n", commentsB)
				return dbmodel.ErrBucketNotFound
			}
			archComments, err := commentsBucket.CreateBucketIfNotExists(threadId)
			if err != nil {
				result += fmt.Sprintf("Could not create archived comments bucket %s: %v\n", threadId, err)
				return err
			}
			resErr := h.moveComments(string(threadId), actComments, archComments)
			if resErr.result != "" {
				result += resErr.result
			}
			if resErr.err != nil {
				return err
			}
			// Check whether there are subcomments and move them to archived
			// contents.
			actSubcomKeys := actComments.Bucket([]byte(subcommentsB))
			if actSubcomKeys != nil {
				result += fmt.Sprintf("Moving subcomments from %d comments.\n", actSubcomKeys.Stats().BucketN)
				archSubcomKeys, err := archComments.CreateBucketIfNotExists([]byte(subcommentsB))
				if err != nil {
					result += fmt.Sprintf("Could not create bucket %s: %v\n", subcommentsB, err)
					return err
				}
				resErr = h.moveSubcomments(actSubcomKeys, archSubcomKeys)
				if resErr.result != "" {
					result += resErr.result
				}
				if resErr.err != nil {
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
				result += fmt.Sprintf("Could not find bucket %s\n", commentsB)
				return dbmodel.ErrBucketNotFound
			}
			// Delete the comments and subcomments from the bucket of active
			// contents. Deleting the comments bucket will also delete the
			// subcomments bucket.
			if err = commentsBucket.DeleteBucket(threadId); err != nil {
				result += fmt.Sprintf("Could not DEL comments bucket of thread %s: %v\n", string(threadId), err)
				return err
			}
		}
		return nil
	})
	return result, err
}

// Move the comments associated to the given thread, from actComments to
// archComments.
func (h *handler) moveComments(threadId string, actComments, archComments *bolt.Bucket) resultErr {
	// Put comments from active comments into archived comments.
	var (
		result string
		numGR  = 0
		c      = actComments.Cursor()
		done   = make(chan resultErr)
		quit   = make(chan struct{})
	)
	defer close(quit)
	for k, v := c.First(); k != nil; k, v = c.Next() {
		// Check whether the value is a nested bucket. If so, just continue.
		// Cursors see nested buckets with value == nil.
		if v == nil {
			continue
		}
		result += fmt.Sprintf("Moving comment %s into archived contents... ", k)
		if err := archComments.Put(k, v); err != nil {
			// Finish early.
			return resultErr {
				result: result + fmt.Sprintf("\nCould not put comment %s into archived contents: %v.\n", k, err),
				err:    err,
			}
		}
		result += "Done.\n"
		// Update activity of user. This can be done in another go-routine.
		numGR++
		go func(k, v []byte) {
			var resErr resultErr
			pbContent := new(pbDataFormat.Content)
			err := proto.Unmarshal(v, pbContent)
			if err != nil {
				resErr.result = fmt.Sprintf("Could not unmarshal comment %s: %v. Aborting comment moving.\n", k, err)
				resErr.err = err
			} else {
				ctx := &pbContext.Comment{
					Id: string(k),
					ThreadCtx: &pbContext.Thread{
						Id: pbContent.Id,
						SectionCtx: &pbContext.Section{
							Id: pbContent.SectionId,
						},
					},
				}
				userId := pbContent.AuthorId
				err = h.markCommentAsOld(userId, ctx)
				if err != nil {
					resErr.result = fmt.Sprintf("Could not mark comment %s as old to user %s: %v. Aborting comment moving.\n",
						ctx.Id, userId, resErr.err)
					resErr.err = err
				} else {
					resErr.result = fmt.Sprintf("Updated comment %s author's activity successfully.\n", ctx.Id)
				}
			}
			select {
			case done<- resErr:
			case <-quit:
			}
		}(k, v)
	}
	var (
		resErr resultErr
		err error
		foundErr bool
	)
	// Check for errors. All the results will be concatenated but only the first
	// err read, if any, will be returned.
	for i := 0; i < numGR; i++ {
		resErr = <-done
		if resErr.result != "" {
			result += resErr.result
		}
		if !foundErr {
			if resErr.err != nil {
				foundErr = true
				err = resErr.err
			}
		}
	}
	resErr.result = result
	resErr.err = err
	return resErr
}

// Move subcoments from every bucket in actSubcomKeys to a new bucket with the
// same key in archSubcomKeys.
func (h *handler) moveSubcomments(actSubcomKeys, archSubcomKeys *bolt.Bucket) resultErr {
	var (
		numGR  = 0
		err    error
		c      = actSubcomKeys.Cursor()
		done   = make(chan resultErr)
		quit   = make(chan struct{})
		result string
	)
	defer close(quit)
	// actSubcomKeys bucket only holds nested buckets, hence the values are
	// discarded and the keys are used to find the buckets, which store the
	// actual active subcomments.
	for comKey, _ := c.First(); comKey != nil; comKey, _ = c.Next() {
		activeSubcom := actSubcomKeys.Bucket(comKey)
		if activeSubcom == nil {
			result += fmt.Sprintf("Could not find subcomments bucket %s.\n", string(comKey))
			continue
		}
		result = fmt.Sprintf("Found %d subcomments in comment %s to move to archived contents.\n", activeSubcom.Stats().KeyN, comKey)
		// Create the corresponding bucket in archSubcomKeys.
		var archivedSubcom *bolt.Bucket
		archivedSubcom, err = archSubcomKeys.CreateBucketIfNotExists(comKey)
		if err != nil {
			result += fmt.Sprintf("Could not create bucket %s: %v. Aborting subcomments moving.\n", comKey, err)
			break
		}
		// Put subcomments from active comments into archived comments.
		subcomCursor := activeSubcom.Cursor()
		for k, v := subcomCursor.First(); k != nil; k, v = subcomCursor.Next() {
			result += fmt.Sprintf("Moving subcomment %s into archived contents... ", k)
			if err = archivedSubcom.Put(k, v); err != nil {
				// Finish early.
				return resultErr{
					err:    err,
					result: result + fmt.Sprintf("\nCould not put subcomment %s into archived contents: %v. Aborting subcomment moving\n",
						k, err),
				}
			}
			result += "Done.\n"
			// Update activity of user. This can be done in another go-routine.
			numGR++
			go func(k, v, comKey []byte) {
				var resErr resultErr
				pbContent := new(pbDataFormat.Content)
				err := proto.Unmarshal(v, pbContent)
				if err != nil {
					resErr.result = fmt.Sprintf("Could not unmarshal subcomment %s: %v. Aborting subcomment moving.\n", k, err)
					resErr.err = err
				} else {
					ctx := &pbContext.Subcomment{
						Id: string(k),
						CommentCtx: &pbContext.Comment{
							Id: string(comKey),
							ThreadCtx: &pbContext.Thread{
								Id: pbContent.Id,
								SectionCtx: &pbContext.Section{
									Id: pbContent.SectionId,
								},
							},
						},
					}
					userId := pbContent.AuthorId
					err = h.markSubcommentAsOld(userId, ctx)
					if err != nil {
						resErr.err = err
						resErr.result = fmt.Sprintf("Could not mark subcomment %s as old to user %s: %v. Aborting subcomment moving.\n",
							ctx.Id, userId, err)
					} else {
						resErr.result = fmt.Sprintf("Updated subcomment %s author's activity successfully.\n", ctx.Id)
					}
				}
				select {
				case done<- resErr:
				case <-quit:
				}
			}(k, v, comKey)
		}
	}
	var (
		resErr resultErr
		foundErr bool
	)
	// Check for errors. All the results will be concatenated but only the first
	// err read, if any, will be returned.
	for i := 0; i < numGR; i++ {
		resErr = <-done
		if resErr.result != "" {
			result += resErr.result
		}
		if !foundErr {
			if resErr.err != nil {
				foundErr = true
				err = resErr.err
			}
		}
	}
	resErr.result = result
	resErr.err = err
	return resErr
}

func (h *handler) markThreadAsOld(userId string, ctx *pbContext.Thread) error {
	var (
		id        = ctx.Id
		sectionId = ctx.SectionCtx.Id
		found     bool
	)
	return h.UpdateUser(userId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
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
			if !found {
				log.Printf("Thread %v is not in recent activity of user %v.\n", ctx, userId)
			}
		}
		return pbUser
	})
}

func (h *handler) markCommentAsOld(userId string, ctx *pbContext.Comment) error {
	var (
		id        = ctx.Id
		threadId  = ctx.ThreadCtx.Id
		sectionId = ctx.ThreadCtx.SectionCtx.Id
		found     bool
	)
	return h.UpdateUser(userId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
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
			if !found {
				log.Printf("Comment %v is not in recent activity of user %v.\n", ctx, userId)
			}
		}
		return pbUser
	})
}

func (h *handler) markSubcommentAsOld(userId string, ctx *pbContext.Subcomment) error {
	var (
		id        = ctx.Id
		commentId = ctx.CommentCtx.Id
		threadId  = ctx.CommentCtx.ThreadCtx.Id
		sectionId = ctx.CommentCtx.ThreadCtx.SectionCtx.Id
		found     bool
	)
	return h.UpdateUser(userId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
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
			if !found {
				log.Printf("Subcomment %v is not in recent activity of user %v.\n", ctx, userId)
			}
		}
		return pbUser
	})
}
