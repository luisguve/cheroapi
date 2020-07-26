package bolt

import (
	"log"

	"github.com/golang/protobuf/proto"
	dbmodel "github.com/luisguve/cheroapi/internal/app/cheroapi"
	pbContext "github.com/luisguve/cheroproto-go/context"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
	bolt "go.etcd.io/bbolt"
)

// DeleteThread removes the thread from the database only if the userId is the
// same as the one indicated by AuthorId on the given thread, then it updates the
// recent or old activity of the given user by removing the reference to the thread.
func (h *handler) DeleteThread(thread *pbContext.Thread, userId string) error {
	var (
		id        = thread.Id
		sectionId = thread.SectionCtx.Id
	)
	// check whether the section exists
	sectionDB, ok := h.sections[sectionId]
	if !ok {
		return dbmodel.ErrSectionNotFound
	}

	// Find thread, check whether the submitter is the author, update activity
	// of author and remove reference to thread from the list of saved threads
	// of every user who saved it, insert thread into the bucket of deleted
	// threads if the thread is active and delete thread from active contents.
	return sectionDB.contents.Update(func(tx *bolt.Tx) error {
		contents, name, err := getThreadBucket(tx, id)
		if err != nil {
			return err
		}
		threadBytes := contents.Get([]byte(id))
		if threadBytes == nil {
			return dbmodel.ErrThreadNotFound
		}
		pbThread := new(pbDataFormat.Content)
		if err = proto.Unmarshal(threadBytes, pbThread); err != nil {
			log.Printf("Could not unmarshal content: %v\n", err)
			return err
		}
		if pbThread.AuthorId != userId {
			return dbmodel.ErrUserNotAllowed
		}
		usersWhoSaved := pbThread.UsersWhoSaved
		var (
			done = make(chan error)
			quit = make(chan struct{})
			// Users to be updated: the author of the thread and the users who
			// saved it.
			users = 1 + len(usersWhoSaved)
		)
		defer close(quit)
		// Delete reference to the thread from the activity of the author.
		go func(userId string) {
			err := h.UpdateUser(userId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
				var found bool
				if pbUser.RecentActivity != nil {
					// Find and remove thread from list of recent activity of user.
					for i, t := range pbUser.RecentActivity.ThreadsCreated {
						if (t.SectionCtx.Id == sectionId) && (t.Id == id) {
							found = true
							last := len(pbUser.RecentActivity.ThreadsCreated) - 1
							pbUser.RecentActivity.ThreadsCreated[i] = pbUser.RecentActivity.ThreadsCreated[last]
							pbUser.RecentActivity.ThreadsCreated = pbUser.RecentActivity.ThreadsCreated[:last]
							break
						}
					}
				}
				if !found {
					if pbUser.OldActivity != nil {
						// Find and remove thread from list of old activity of user.
						for i, t := range pbUser.OldActivity.ThreadsCreated {
							if (t.SectionCtx.Id == sectionId) && (t.Id == id) {
								found = true
								last := len(pbUser.RecentActivity.ThreadsCreated) - 1
								pbUser.RecentActivity.ThreadsCreated[i] = pbUser.RecentActivity.ThreadsCreated[last]
								pbUser.RecentActivity.ThreadsCreated = pbUser.RecentActivity.ThreadsCreated[:last]
								break
							}
						}
					}
				}
				if !found {
					log.Printf("Delete content: could not find thread %v in neither recent nor old activity of user %s\n", thread, userId)
				}
				return pbUser
			})
			select {
			case done<- err:
			case <-quit:
			}
		}(userId)
		// Delete reference to the thread from the list of saved threads of every
		// user who saved it.
		for _, userId = range usersWhoSaved {
			go func(userId string) {
				err := h.UpdateUser(userId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
					// Find and remove saved thread.
					for idx, t := range pbUser.SavedThreads {
						if (t.SectionCtx.Id == thread.SectionCtx.Id) && (t.Id == thread.Id) {
							last := len(pbUser.SavedThreads) - 1
							pbUser.SavedThreads[idx] = pbUser.SavedThreads[last]
							pbUser.SavedThreads = pbUser.SavedThreads[:last]
							break
						}
					}
					return pbUser
				})
				select {
				case done<- err:
				case <-quit:
				}
			}(userId)
		}
		// Check whether the thread is active. If so, insert it into the bucket
		// of deleted threads.
		if name == activeContentsB {
			delContents := contents.Bucket([]byte(deletedThreadsB))
			if delContents == nil {
				log.Printf("Bucket %s not found.\n", deletedThreadsB)
				return dbmodel.ErrBucketNotFound
			}
			if err = delContents.Put([]byte(id), threadBytes); err != nil {
				return err
			}
		}
		err = contents.Delete([]byte(id))
		if err != nil {
			log.Printf("Could not delete thread: %v.\n", err)
			return err
		}
		// Check for errors. It terminates every go-routine hung on the statement
		// case "done<- err" and returns the first err received.
		for i := 0; i < users; i++ {
			err = <-done
			if err != nil {
				log.Println(err)
				break
			}
		}
		return err
	})
}

// DeleteComment removes the comment from the database only if the userId is the
// same as the one indicated by AuthorId on the given comment, then it updates the
// recent or old activity of the given user by removing the reference to the comment.
func (h *handler) DeleteComment(comment *pbContext.Comment, userId string) error {
	var (
		id        = comment.Id
		threadId  = comment.ThreadCtx.Id
		sectionId = comment.ThreadCtx.SectionCtx.Id
	)
	// check whether the section exists
	sectionDB, ok := h.sections[sectionId]
	if !ok {
		return dbmodel.ErrSectionNotFound
	}

	// Find comment, check whether the submitter is the author, delete comment,
	// update user by removing the comment from its activity, decrease replies
	// of thread by 1 and remove user from list of repliers.
	return sectionDB.contents.Update(func(tx *bolt.Tx) error {
		commentsBucket, _, err := getCommentsBucket(tx, threadId)
		if err != nil {
			return err
		}
		commentBytes := commentsBucket.Get([]byte(id))
		if commentBytes == nil {
			return dbmodel.ErrCommentNotFound
		}
		pbComment := new(pbDataFormat.Content)
		if err = proto.Unmarshal(commentBytes, pbComment); err != nil {
			log.Printf("Could not unmarshal content: %v\n", err)
			return err
		}
		if pbComment.AuthorId != userId {
			return dbmodel.ErrUserNotAllowed
		}
		err = commentsBucket.Delete([]byte(id))
		if err != nil {
			return err
		}
		// Update user; remove comment from its activity.
		err = h.UpdateUser(userId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
			if (pbUser.RecentActivity == nil) && (pbUser.OldActivity == nil) {
				log.Printf("Delete content: user %s has neither recent nor old activity\n", userId)
				return pbUser
			}
			var found bool
			if pbUser.RecentActivity != nil {
				// Find and remove comment from list of recent activity of user.
				for i, c := range pbUser.RecentActivity.Comments {
					if (c.ThreadCtx.SectionCtx.Id == sectionId) &&
						(c.ThreadCtx.Id == threadId) &&
						(c.Id == id) {
						found = true
						last := len(pbUser.RecentActivity.Comments) - 1
						pbUser.RecentActivity.Comments[i] = pbUser.RecentActivity.Comments[last]
						pbUser.RecentActivity.Comments = pbUser.RecentActivity.Comments[:last]
						break
					}
				}
			}
			if !found {
				if pbUser.OldActivity != nil {
					// Find and remove comment from list of old activity of user.
					for i, c := range pbUser.OldActivity.Comments {
						if (c.ThreadCtx.SectionCtx.Id == sectionId) &&
							(c.ThreadCtx.Id == threadId) &&
							(c.Id == id) {
							found = true
							last := len(pbUser.RecentActivity.Comments) - 1
							pbUser.RecentActivity.Comments[i] = pbUser.RecentActivity.Comments[last]
							pbUser.RecentActivity.Comments = pbUser.RecentActivity.Comments[:last]
							break
						}
					}
				}
			}
			if !found {
				log.Printf("Delete content: could not find comment %v in neither recent nor old activity of user %s\n", comment, userId)
			}
			return pbUser
		})
		if err != nil {
			return err
		}
		// Update the thread which the comment belongs to; decrease Replies by 1
		// and remove user id from list of repliers.
		threadsBucket, _, err := getThreadBucket(tx, threadId)
		if err != nil {
			// The thread does not exist anymore.
			log.Println(err)
			return nil
		}
		threadBytes := threadsBucket.Get([]byte(threadId))
		if threadBytes == nil {
			return dbmodel.ErrThreadNotFound
		}
		pbThread := new(pbDataFormat.Content)
		if err = proto.Unmarshal(threadBytes, pbThread); err != nil {
			log.Printf("Could not unmarshal content: %v\n", err)
			return err
		}
		pbThread.Replies--
		replied, idx := inSlice(pbThread.ReplierIds, userId)
		if replied {
			last := len(pbThread.ReplierIds) - 1
			pbThread.ReplierIds[idx] = pbThread.ReplierIds[last]
			pbThread.ReplierIds = pbThread.ReplierIds[:last]
		}
		threadBytes, err = proto.Marshal(pbThread)
		if err != nil {
			log.Printf("Could not marshal content: %v\n", err)
			return err
		}
		return threadsBucket.Put([]byte(threadId), threadBytes)
	})
}

// DeleteSubcomment removes the subcomment from the database only if the userId
// is the same as the one indicated by AuthorId on the given subcomment, then it
// updates the recent or old activity of the given user by removing the reference
// to the subcomment.
func (h *handler) DeleteSubcomment(subcomment *pbContext.Subcomment, userId string) error {
	var (
		id        = subcomment.Id
		commentId = subcomment.CommentCtx.Id
		threadId  = subcomment.CommentCtx.ThreadCtx.Id
		sectionId = subcomment.CommentCtx.ThreadCtx.SectionCtx.Id
	)
	// check whether the section exists
	sectionDB, ok := h.sections[sectionId]
	if !ok {
		return dbmodel.ErrSectionNotFound
	}

	// Find subcomment, check whether the submitter is the author, delete
	// subcomment, update activity of subcomment author, decrease replies of
	// both the comment and thread the subcomment belongs to by 1 and remove
	// user from list of repliers of the comment.
	return sectionDB.contents.Update(func(tx *bolt.Tx) error {
		subcommentsBucket, _, err := getSubcommentsBucket(tx, threadId, commentId)
		if err != nil {
			return err
		}
		subcommentBytes := subcommentsBucket.Get([]byte(id))
		if subcommentBytes == nil {
			return dbmodel.ErrSubcommentNotFound
		}
		pbSubcomment := new(pbDataFormat.Content)
		if err = proto.Unmarshal(subcommentBytes, pbSubcomment); err != nil {
			log.Printf("Could not unmarshal content: %v\n", err)
			return err
		}
		if pbSubcomment.AuthorId != userId {
			return dbmodel.ErrUserNotAllowed
		}
		if err = subcommentsBucket.Delete([]byte(id)); err != nil {
			return err
		}
		// Update user; remove subcomment from its activity.
		err = h.UpdateUser(userId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
			if (pbUser.RecentActivity == nil) && (pbUser.OldActivity == nil) {
				log.Printf("Delete content: user %s has neither recent nor old activity\n", userId)
				return pbUser
			}
			var found bool
			if pbUser.RecentActivity != nil {
				// Find and remove subcomment from list of recent activity of user.
				for i, s := range pbUser.RecentActivity.Subcomments {
					if (s.CommentCtx.ThreadCtx.SectionCtx.Id == sectionId) &&
						(s.CommentCtx.ThreadCtx.Id == threadId) &&
						(s.CommentCtx.Id == commentId) &&
						(s.Id == id) {
						found = true
						last := len(pbUser.RecentActivity.Subcomments) - 1
						pbUser.RecentActivity.Subcomments[i] = pbUser.RecentActivity.Subcomments[last]
						pbUser.RecentActivity.Subcomments = pbUser.RecentActivity.Subcomments[:last]
						break
					}
				}
			}
			if !found {
				if pbUser.OldActivity != nil {
					// Find and remove subcomment from list of old activity of user.
					for i, s := range pbUser.OldActivity.Subcomments {
						if (s.CommentCtx.ThreadCtx.SectionCtx.Id == sectionId) &&
							(s.CommentCtx.ThreadCtx.Id == threadId) &&
							(s.CommentCtx.Id == commentId) &&
							(s.Id == id) {
							found = true
							last := len(pbUser.RecentActivity.Subcomments) - 1
							pbUser.RecentActivity.Subcomments[i] = pbUser.RecentActivity.Subcomments[last]
							pbUser.RecentActivity.Subcomments = pbUser.RecentActivity.Subcomments[:last]
							break
						}
					}
				}
			}
			if !found {
				log.Printf("Delete content: could not find subcomment %v in neither recent nor old activity of user %s\n", subcomment, userId)
			}
			return pbUser
		})
		if err != nil {
			return err
		}
		// Update the comment which the subcomment belongs to; decrease replies
		// by 1 and remove user id from list of repliers.
		commentsBucket, _, err := getCommentsBucket(tx, threadId)
		if err != nil {
			return err
		}
		commentBytes := commentsBucket.Get([]byte(commentId))
		if commentBytes == nil {
			return dbmodel.ErrCommentNotFound
		}
		pbComment := new(pbDataFormat.Content)
		if err = proto.Unmarshal(commentBytes, pbComment); err != nil {
			log.Printf("Could not unmarshal content: %v\n", err)
			return err
		}
		pbComment.Replies--
		replied, idx := inSlice(pbComment.ReplierIds, userId)
		if replied {
			last := len(pbComment.ReplierIds) - 1
			pbComment.ReplierIds[idx] = pbComment.ReplierIds[last]
			pbComment.ReplierIds = pbComment.ReplierIds[:last]
		}
		commentBytes, err = proto.Marshal(pbComment)
		if err != nil {
			log.Printf("Could not marshal content: %v\n", err)
			return err
		}
		err = commentsBucket.Put([]byte(commentId), commentBytes)
		if err != nil {
			return err
		}
		// Update the thread which both the subcomment and the comment belongs
		// to; decrease replies by 1.
		threadsBucket, _, err := getThreadBucket(tx, threadId)
		if err != nil {
			return err
		}
		threadBytes := threadsBucket.Get([]byte(threadId))
		if threadBytes == nil {
			return dbmodel.ErrThreadNotFound
		}
		pbThread := new(pbDataFormat.Content)
		if err = proto.Unmarshal(threadBytes, pbThread); err != nil {
			log.Printf("Could not unmarshal content: %v\n", err)
			return err
		}
		pbThread.Replies--
		threadBytes, err = proto.Marshal(pbThread)
		if err != nil {
			log.Printf("Could not marshal content: %v\n", err)
			return err
		}
		return threadsBucket.Put([]byte(threadId), threadBytes)
	})
}
