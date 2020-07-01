package bolt

import (
	"log"

	"github.com/luisguve/cheroapi/internal/pkg/dbmodel"
	pbContext "github.com/luisguve/cheroproto-go/context"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
	bolt "go.etcd.io/bbolt"
)

// DeleteThread removes the thread from the database only if the userId is the
// same as the one indicated by AuthorId on the given thread, then it updates the
// recent or old activity of the given user by removing the reference to the thread.
func (h *handler) DeleteThread(thread *pbContext.Thread, userId string) error {
	var (
		id = thread.Id
		sectionId = thread.SectionCtx.Id
		usersWhoSaved []string
	)
	// check whether the section exists
	if sectionDB, ok := h.sections[sectionId]; !ok {
		return dbmodel.ErrSectionNotFound
	}

	// Find thread, check whether the submitter is the author, insert thread into
	// the bucket of deleted threads if the thread is active and delete thread
	// from active contents.
	err := sectionDB.contents.Update(func(tx *bolt.Tx) error {
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
			return dbmodel.ErrUsetNotAllowed
		}
		// Check whether the thread is active. If so, it inserts it into the
		// bucket of deleted threads.
		if name == activeContentsB {
			delContents := contents.Bucket([]byte(deletedThreadsB))
			if delContents == nil {
				log.Printf("bucket %s not found\n", deletedThreadsB)
				return dbmodel.ErrBucketNotFound
			}
			if err = delContents.Put([]byte(id), threadBytes); err != nil {
				return err
			}
		}
		usersWhoSaved = pbThread.UsersWhoSaved
		return contents.Delete([]byte(id))
	})
	if err != nil {
		log.Println(err)
		return err
	}
	var (
		done = make(chan error)
		quit = make(chan error)
		// Users to be updated: the author of the thread and the users who
		// saved it.
		users = 1 + len(usersWhoSaved)
	)
	// Delete reference to the thread from the activity of the author.
	go func(userId string) {
		pbUser, err := h.User(userId)
		if err == nil {
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
			} else {
				err = h.UpdateUser(pbUser, userId)
			}
		}
		select {
		case done<- err:
		case <-quit:
		}
	}(userId)
	// Delete reference to the thread from the list of saved threads of every
	// user who saved it.
	for _, userId = range usersWhoSaved {
		go func(userId string) {
			pbUser, err := h.User(userId)
			if err == nil {
				// Find and remove saved thread.
				for idx, t := range pbUser.SavedThreads {
					if (t.SectionCtx.Id == thread.SectionCtx.Id) && (t.Id == thread.Id) {
						last := len(pbUser.SavedThreads) - 1
						pbUser.SavedThreads[idx] = pbUser.SavedThreads[last]
						pbUser.SavedThreads = pbUser.SavedThreads[:last]
						break
					}
				}
				err = h.UpdateUser(pbUser, userId)
			}
			select {
			case done<- err:
			case <-quit:
			}
		}(userId)
	}
	// Check for errors. It terminates every go-routine hung on the statement
	// case "done<- err" and returns the first err received.
	for i := 0; i < users; i++ {
		err = <-done
		if err != nil {
			log.Println(err)
			close(quit)
			return err
		}
	}
	return nil
}

// DeleteComment removes the comment from the database only if the userId is the
// same as the one indicated by AuthorId on the given comment, then it updates the
// recent or old activity of the given user by removing the reference to the comment.
func (h *handler) DeleteComment(comment *pbContext.Comment, userId string) error {
	var (
		id = comment.Id
		threadId = comment.ThreadCtx.Id
		sectionId = comment.ThreadCtx.SectionCtx.Id
	)
	// check whether the section exists
	if sectionDB, ok := h.sections[sectionId]; !ok {
		return dbmodel.ErrSectionNotFound
	}

	// Find comment, check whether the submitter is the author, insert comment
	// in the bucket of deleted comments if the comment is in an active thread,
	// decrease replies of thread by 1 and remove user from list of repliers.
	err := sectionDB.contents.Update(func(tx *bolt.Tx) error {
		commentsBucket, name, err := getCommentsBucket(tx, threadId)
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
		// Check whether the comment belongs to an active thread. If so, it
		// inserts it into the bucket of deleted comments.
		if name == activeContentsB {
			delContents := commentsBucket.Bucket([]byte(deletedCommentsB))
			if delContents == nil {
				log.Printf("bucket %s not found\n", deletedCommentsB)
				return dbmodel.ErrBucketNotFound
			}
			if err = delContents.Put([]byte(id), commentBytes); err != nil {
				return err
			}
		}
		err = commentsBucket.Delete([]byte(id))
		if err != nil {
			return err
		}
		// Update the thread which the comment belongs to; decrease Replies by 1
		// and remove user id from list of repliers.
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
	if err != nil {
		log.Println(err)
		return err
	}
	// Update user; remove comment from its activity.
	pbUser, err := h.User(userId)
	if err != nil {
		return err
	}
	if (pbUser.RecentActivity == nil) && (pbUser.OldActivity == nil) {
		log.Printf("Delete content: user %s has neither recent nor old activity\n", userId)
		return nil
	}
	var found bool
	if pbUser.RecentActivity != nil {
		// Find and remove comment from list of recent activity of user.
		for i, c := range pbUser.RecentActivity.Comments {
			if (c.ThreadCtx.SectionCtx.Id == sectionId) &&
			(c.ThreadCtx.Id == threadId) &&
			(c.Id == id) && {
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
				(c.Id == id) && {
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
		return nil
	}
	return h.UpdateUser(pbUser, userId)
}

// DeleteSubcomment removes the subcomment from the database only if the userId
// is the same as the one indicated by AuthorId on the given subcomment, then it
// updates the recent or old activity of the given user by removing the reference
// to the subcomment.
func (h *handler) DeleteSubcomment(subcomment *pbContext.Subcomment, userId string) error {
	var (
		id = subcomment.Id
		commentId = subcomment.CommentCtx.Id
		threadId = subcomment.CommentCtx.ThreadCtx.Id
		sectionId = subcomment.CommentCtx.ThreadCtx.SectionCtx.Id
	)
	// check whether the section exists
	if sectionDB, ok := h.sections[sectionId]; !ok {
		return dbmodel.ErrSectionNotFound
	}

	// Find subcomment, check whether the submitter is the author, delete
	// subcomment, decrease replies of both the comment and thread the subcomment
	// belongs to by 1 and remove user from list of repliers of the comment.
	err := sectionDB.contents.Update(func(tx *bolt.Tx) error {
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
			return dbmodel.ErrUsetNotAllowed
		}
		if err = subcommentsBucket.Delete([]byte(id)); err != nil {
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
	if err != nil {
		log.Println(err)
		return err
	}
	// Update user; remove subcomment from its activity.
	pbUser, err := h.User(userId)
	if err != nil {
		return err
	}
	if (pbUser.RecentActivity == nil) && (pbUser.OldActivity == nil) {
		log.Printf("Delete content: user %s has neither recent nor old activity\n", userId)
		return nil
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
		return nil
	}
	return h.UpdateUser(pbUser, userId)
}
