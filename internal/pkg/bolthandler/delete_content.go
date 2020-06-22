package bolthandler

import (
	"log"

	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	pbContext "github.com/luisguve/cheroproto-go/context"
)

// DeleteThread removes the thread from the database only if the userId is the
// same as the one indicated by AuthorId on the given thread, then it updates the
// recent or old activity of the given user by removing the reference to the thread.
func (h *handler) DeleteThread(thread *pbContext.Thread, userId string) error {
	var (
		id = thread.Id
		sectionId = thread.SectionCtx.Id
	)
	// check whether the section exists
	if sectionDB, ok := h.sections[sectionId]; !ok {
		return ErrSectionNotFound
	}

	err := sectionDB.contents.Update(func(tx *bolt.Tx) error {
		threadsBucket, err := getThreadBucket(tx, threadId)
		if err != nil {
			return err
		}
		threadBytes := threadsBucket.Get([]byte(id))
		if threadBytes == nil {
			return ErrThreadNotFound
		}
		pbThread := new(pbDataFormat.Content)
		if err = proto.Unmarshal(pbThread, threadBytes); err != nil {
			log.Printf("Could not unmarshal content: %v\n", err)
			return err
		}
		if pbThread.AuthorId != userId {
			return ErrUsetNotAllowed
		}
		return threadsBucket.Delete([]byte(id))
	})
	if err != nil {
		log.Println(err)
		return err
	}
	err = h.users.Update(func(tx *bolt.Tx) error {
		usersBucket := tx.Bucket(usersB)
		if usersB == nil {
			log.Printf("Bucket %s of users not found\n", usersB)
			return ErrBucketNotFound
		}
		pbUserBytes := usersBucket.Get([]byte(userId))
		if pbUserBytes == nil {
			log.Printf("User %s not found\n", userId)
			return ErrUserNotFound
		}
		pbUser := new(pbDataFormat.User)
		err = proto.Unmarshal(pbUser, pbUserBytes)
		if err != nil {
			log.Printf("Could not unmarshal user: %v\n", err)
			return err
		}
		if pbUser.RecentActivity == nil {
			return nil
		}
		var found bool
		// remove thread from list of recent activity of user
		for i, t := range pbUser.RecentActivity.ThreadsCreated {
			if (t.SectionCtx.Id == sectionId) && (t.Id == id) {
				found = true
				last := len(pbUser.RecentActivity.ThreadsCreated) - 1
				pbUser.RecentActivity.ThreadsCreated[i] = pbUser.RecentActivity.ThreadsCreated[last]
				pbUser.RecentActivity.ThreadsCreated = pbUser.RecentActivity.ThreadsCreated[:last]
				break
			}
		}
		if !found {
			// remove thread from list of old activity of user
			for i, t := range pbUser.OldActivity.ThreadsCreated {
				if (t.SectionCtx.Id == sectionId) && (t.Id == id) {
					last := len(pbUser.RecentActivity.ThreadsCreated) - 1
					pbUser.RecentActivity.ThreadsCreated[i] = pbUser.RecentActivity.ThreadsCreated[last]
					pbUser.RecentActivity.ThreadsCreated = pbUser.RecentActivity.ThreadsCreated[:last]
					break
				}
			}
		}
		pbUserBytes, err = proto.Marshal(pbUser)
		if err != nil {
			log.Printf("Could not marshal user: %v\n", err)
			return err
		}
		return usersBucket.Put([]byte(userId), pbUserBytes)
	})
	if err != nil {
		log.Println(err)
		return err
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
		return ErrSectionNotFound
	}

	err := sectionDB.contents.Update(func(tx *bolt.Tx) error {
		commentsBucket, err := getCommentsBucket(tx, threadId)
		if err != nil {
			return err
		}
		commentBytes := commentsBucket.Get([]byte(id))
		if commentBytes == nil {
			return ErrCommentNotFound
		}
		pbComment := new(pbDataFormat.Content)
		if err = proto.Unmarshal(pbComment, commentBytes); err != nil {
			log.Printf("Could not unmarshal content: %v\n", err)
			return err
		}
		if pbComment.AuthorId != userId {
			return ErrUsetNotAllowed
		}
		err = commentsBucket.Delete([]byte(id))
		if err != nil {
			return err
		}
		// Update the thread which the comment belongs to; decrease Replies by 1
		// and remove user id from list of repliers.
		threadsBucket, err := getThreadBucket(tx, threadId)
		if err != nil {
			return err
		}
		threadBytes := threadsBucket.Get([]byte(threadId))
		if threadBytes == nil {
			return ErrThreadNotFound
		}
		pbThread := new(pbDataFormat.Content)
		if err = proto.Unmarshal(pbThread, threadBytes); err != nil {
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
	err = h.users.Update(func(tx *bolt.Tx) error {
		usersBucket := tx.Bucket(usersB)
		if usersB == nil {
			log.Printf("Bucket %s of users not found\n", usersB)
			return ErrBucketNotFound
		}
		pbUserBytes := usersBucket.Get([]byte(userId))
		if pbUserBytes == nil {
			log.Printf("User %s not found\n", userId)
			return ErrUserNotFound
		}
		pbUser := new(pbDataFormat.User)
		err = proto.Unmarshal(pbUser, pbUserBytes)
		if err != nil {
			log.Printf("Could not unmarshal user: %v\n", err)
			return err
		}
		if pbUser.RecentActivity == nil {
			return nil
		}
		var found bool
		// remove comment from list of recent activity of user
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
		if !found {
			// remove comment from list of old activity of user
			for i, c := range pbUser.OldActivity.Comments {
				if (c.ThreadCtx.SectionCtx.Id == sectionId) &&
				(c.ThreadCtx.Id == threadId) &&
				(c.Id == id) && {
					last := len(pbUser.RecentActivity.Comments) - 1
					pbUser.RecentActivity.Comments[i] = pbUser.RecentActivity.Comments[last]
					pbUser.RecentActivity.Comments = pbUser.RecentActivity.Comments[:last]
					break
				}
			}
		}
		pbUserBytes, err = proto.Marshal(pbUser)
		if err != nil {
			log.Printf("Could not marshal user: %v\n", err)
			return err
		}
		return usersBucket.Put([]byte(userId), pbUserBytes)
	})
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
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
		return ErrSectionNotFound
	}

	err := sectionDB.contents.Update(func(tx *bolt.Tx) error {
		subcommentsBucket, err := getSubcommentsBucket(tx, threadId, commentId)
		if err != nil {
			return err
		}
		subcommentBytes := subcommentsBucket.Get([]byte(id))
		if subcommentBytes == nil {
			return ErrSubcommentNotFound
		}
		pbSubcomment := new(pbDataFormat.Content)
		if err = proto.Unmarshal(pbSubcomment, subcommentBytes); err != nil {
			log.Printf("Could not unmarshal content: %v\n", err)
			return err
		}
		if pbSubcomment.AuthorId != userId {
			return ErrUsetNotAllowed
		}
		err = subcommentsBucket.Delete([]byte(id))
		if err != nil {
			return err
		}
		// Update the comment which the subcomment belongs to; decrease replies
		// by 1 and remove user id from list of repliers.
		commentsBucket, err := getCommentsBucket(tx, threadId)
		if err != nil {
			return err
		}
		commentBytes := commentsBucket.Get([]byte(commentId))
		if commentBytes == nil {
			return ErrCommentNotFound
		}
		pbComment := new(pbDataFormat.Content)
		if err = proto.Unmarshal(pbComment, commentBytes); err != nil {
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
		threadsBucket, err := getThreadBucket(tx, threadId)
		if err != nil {
			return err
		}
		threadBytes := threadsBucket.Get([]byte(threadId))
		if threadBytes == nil {
			return ErrThreadNotFound
		}
		pbThread := new(pbDataFormat.Content)
		if err = proto.Unmarshal(pbThread, threadBytes); err != nil {
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
	err = h.users.Update(func(tx *bolt.Tx) error {
		usersBucket := tx.Bucket(usersB)
		if usersB == nil {
			log.Printf("Bucket %s of users not found\n", usersB)
			return ErrBucketNotFound
		}
		pbUserBytes := usersBucket.Get([]byte(userId))
		if pbUserBytes == nil {
			log.Printf("User %s not found\n", userId)
			return ErrUserNotFound
		}
		pbUser := new(pbDataFormat.User)
		err = proto.Unmarshal(pbUser, pbUserBytes)
		if err != nil {
			log.Printf("Could not unmarshal user: %v\n", err)
			return err
		}
		if pbUser.RecentActivity == nil {
			return nil
		}
		var found bool
		// remove subcomment from list of recent activity of user
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
		if !found {
			// remove subcomment from list of old activity of user
			for i, s := range pbUser.OldActivity.Subcomments {
				if (s.CommentCtx.ThreadCtx.SectionCtx.Id == sectionId) &&
				(s.CommentCtx.ThreadCtx.Id == threadId) &&
				(s.CommentCtx.Id == commentId) && 
				(s.Id == id) {
					last := len(pbUser.RecentActivity.Subcomments) - 1
					pbUser.RecentActivity.Subcomments[i] = pbUser.RecentActivity.Subcomments[last]
					pbUser.RecentActivity.Subcomments = pbUser.RecentActivity.Subcomments[:last]
					break
				}
			}
		}
		pbUserBytes, err = proto.Marshal(pbUser)
		if err != nil {
			log.Printf("Could not marshal user: %v\n", err)
			return err
		}
		return usersBucket.Put([]byte(userId), pbUserBytes)
	})
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}
