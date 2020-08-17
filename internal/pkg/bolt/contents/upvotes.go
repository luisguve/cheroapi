package contents

import (
	"fmt"
	"log"
	"time"

	"github.com/golang/protobuf/proto"
	pbTime "github.com/golang/protobuf/ptypes/timestamp"
	dbmodel "github.com/luisguve/cheroapi/internal/app/cheroapi"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	pbContext "github.com/luisguve/cheroproto-go/context"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
	pbMetadata "github.com/luisguve/cheroproto-go/metadata"
	bolt "go.etcd.io/bbolt"
)

// incInteractions increases by one the number of interactions of the given
// *pbMetadata.Content, sets LastUpdated to time.Now().Unix() and calculates the
// average update time difference by dividing the accumulated difference between
// interactions by the number of Interactions.
func incInteractions(m *pbMetadata.Content) {
	now := &pbTime.Timestamp{Seconds: time.Now().Unix()}
	m.Interactions++

	lastUpdated := m.LastUpdated
	m.LastUpdated = now

	diff := now.Seconds - lastUpdated.Seconds

	m.Diff += diff

	m.AvgUpdateTime = float64(m.Diff) / float64(m.Interactions)
}

// inSlice returns whether user is in users and an integer indicating the index
// where the user id is in the slice. Returns false and 0 if the user is not in
// the given slice.
func inSlice(users []string, user string) (bool, int) {
	for idx, u := range users {
		if u == user {
			return true, idx
		}
	}
	return false, 0
}

// UpvoteThread increases by one the number of upvotes that the given thread has
// received, updates the metadata related to its interactions and saves the
// notification in the list of unread notifications of the thread author.
//
// It returns the notification for the owner of the thread and its user id and
// a nil error on success, or a nil *pbApi.NotifyUser and a nil error if the
// submitter is the content author, or a nil *pbApi.NotifyUser and an
// ErrThreadNotFound or proto marshal/unmarshal error on failure.
func (h *handler) UpvoteThread(userId string, thread *pbContext.Thread) (*pbApi.NotifyUser, error) {
	var (
		id        = thread.Id
		sectionId = thread.SectionCtx.Id
		pbThread  = new(pbDataFormat.Content)
	)
	// check whether the section exists
	sectionDB, ok := h.sections[sectionId]
	if !ok {
		return nil, dbmodel.ErrSectionNotFound
	}
	err := sectionDB.contents.Update(func(tx *bolt.Tx) error {
		threadBytes, err := getThreadBytes(tx, id)
		if err != nil {
			log.Printf("Could not find thread %s in section %s: %v.\n", id, sectionId, err)
			return err
		}
		if err = proto.Unmarshal(threadBytes, pbThread); err != nil {
			log.Printf("Could not unmarshal content: %v.\n", err)
			return err
		}
		// Check whether the user has already upvoted this thread.
		if voted, _ := inSlice(pbThread.VoterIds, userId); voted {
			return dbmodel.ErrUserNotAllowed
		}
		pbThread.Upvotes++
		pbThread.VoterIds = append(pbThread.VoterIds, userId)
		// Increment interactions and calculata new average update time only if
		// this user has not undone an interaction on this content before.
		undoner, _ := inSlice(pbThread.UndonerIds, userId)
		if !undoner {
			incInteractions(pbThread.Metadata)
		}
		threadBytes, err = proto.Marshal(pbThread)
		if err != nil {
			log.Printf("Could not marshal content: %v\n", err)
			return err
		}
		return setThreadBytes(tx, id, threadBytes)
	})
	if err != nil {
		return nil, err
	}
	// Set notification and notify user only if the submitter is not the author.
	toNotify := pbThread.AuthorId
	if userId != toNotify {
		var msg string
		if int(pbThread.Upvotes) > 1 {
			msg = fmt.Sprintf("%d users have upvoted your thread", pbThread.Upvotes)
		} else {
			msg = "1 user has upvoted your thread"
		}
		subj := fmt.Sprintf("On your thread %s", pbThread.Title)
		notifType := pbDataFormat.Notif_UPVOTE
		notifyUser := h.notifyInteraction(userId, toNotify, msg, subj, notifType, pbThread)
		return notifyUser, nil
	}
	return nil, nil
}

// UndoUpvoteThread decreases by one the number of upvotes that the thread has
// received, removes the user id from the list of voters and appends it to the
// list of undoners. It does not update the interactions of the thread.
func (h *handler) UndoUpvoteThread(userId string, thread *pbContext.Thread) error {
	var (
		id        = thread.Id
		sectionId = thread.SectionCtx.Id
	)
	// check whether the section exists
	sectionDB, ok := h.sections[sectionId]
	if !ok {
		return dbmodel.ErrSectionNotFound
	}
	return sectionDB.contents.Update(func(tx *bolt.Tx) error {
		threadBytes, err := getThreadBytes(tx, id)
		if err != nil {
			log.Printf("Could not find thread %s in section %s: %v.\n", id, sectionId, err)
			return err
		}
		pbThread := new(pbDataFormat.Content)
		if err = proto.Unmarshal(threadBytes, pbThread); err != nil {
			log.Printf("Could not unmarshal content: %v.\n", err)
			return err
		}
		voted, idx := inSlice(pbThread.VoterIds, userId)
		if !voted {
			return dbmodel.ErrNotUpvoted
		}
		pbThread.Upvotes--

		last := len(pbThread.VoterIds) - 1
		pbThread.VoterIds[idx] = pbThread.VoterIds[last]
		pbThread.VoterIds = pbThread.VoterIds[:last]

		undoner, _ := inSlice(pbThread.UndonerIds, userId)
		if !undoner {
			pbThread.UndonerIds = append(pbThread.UndonerIds, userId)
		}
		threadBytes, err = proto.Marshal(pbThread)
		if err != nil {
			log.Printf("Could not marshal content: %v.\n", err)
			return err
		}
		return setThreadBytes(tx, id, threadBytes)
	})
}

// UpvoteComment increases by one the number of upvotes that the given comment
// has received, and updates the interactions and Average Update Time of both
// the comment and the thread it belongs to and saves the notification in the
// list of unread notifications of the comment author and the thread author.
//
// It returns the user id and the notification for both the thread author and
// the comment author and a nil error on success, or a nil []*pbApi.NotifyUser
// and an ErrThreadNotFound, ErrCommentNotFound or proto marshal/unmarshal error
// on failure.
//
// It may return just one *pbApi.NotifyUser if the submitter is the author of
// either the thread or the comment.
func (h *handler) UpvoteComment(userId string, comment *pbContext.Comment) ([]*pbApi.NotifyUser, error) {
	var (
		commentId = comment.Id
		threadId  = comment.ThreadCtx.Id
		sectionId = comment.ThreadCtx.SectionCtx.Id
		pbComment = new(pbDataFormat.Content)
		pbThread  = new(pbDataFormat.Content)
		notifs    []*pbApi.NotifyUser
	)
	// check whether the section exists
	sectionDB, ok := h.sections[sectionId]
	if !ok {
		return nil, dbmodel.ErrSectionNotFound
	}
	err := sectionDB.contents.Update(func(tx *bolt.Tx) error {
		// Get thread which the comment belongs to.
		threadBytes, err := getThreadBytes(tx, threadId)
		if err != nil {
			log.Printf("Could not find thread %s in section %s: %v.\n", threadId, sectionId, err)
			return err
		}
		if err = proto.Unmarshal(threadBytes, pbThread); err != nil {
			log.Printf("Could not unmarshal content: %v.\n", err)
			return err
		}
		// Update thread metadata: increment interactions and calculata new
		// average update time only if this user has not undone an interaction
		// on this content before.
		undoner, _ := inSlice(pbThread.UndonerIds, userId)
		if !undoner {
			incInteractions(pbThread.Metadata)
			threadBytes, err = proto.Marshal(pbThread)
			if err != nil {
				log.Printf("Could not marshal content: %v.\n", err)
				return err
			}
			if err = setThreadBytes(tx, threadId, threadBytes); err != nil {
				return err
			}
		}
		// Get comment.
		commentBytes, err := getCommentBytes(tx, threadId, commentId)
		if err != nil {
			log.Printf("Could not find comment %s in thread %s in section %s: %v.\n",
				commentId, threadId, sectionId, err)
			return err
		}
		if err = proto.Unmarshal(commentBytes, pbComment); err != nil {
			log.Printf("Could not unmarshal content: %v.\n", err)
			return err
		}
		// Check whether the user has already upvoted this comment.
		if voted, _ := inSlice(pbComment.VoterIds, userId); voted {
			return dbmodel.ErrUserNotAllowed
		}
		pbComment.Upvotes++
		pbComment.VoterIds = append(pbComment.VoterIds, userId)
		// Increment interactions and calculata new average update time only
		// if this user has not undone an interaction on this content before.
		undoner, _ = inSlice(pbComment.UndonerIds, userId)
		if !undoner {
			incInteractions(pbComment.Metadata)
		}
		commentBytes, err = proto.Marshal(pbComment)
		if err != nil {
			log.Printf("Could not marshal content: %v.\n", err)
			return err
		}
		return setCommentBytes(tx, threadId, commentId, commentBytes)
	})
	if err != nil {
		return nil, err
	}
	// Set notification only if the submitter is not the comment author.
	toNotify := pbComment.AuthorId
	if userId != toNotify {
		// set notification
		var msg string
		if pbComment.Upvotes > 1 {
			msg = fmt.Sprintf("%d users have upvoted your comment", pbComment.Upvotes)
		} else {
			msg = "1 user has upvoted your comment"
		}
		subj := fmt.Sprintf("On your comment on %s", pbComment.Title)
		notifType := pbDataFormat.Notif_UPVOTE_COMMENT
		notifyUser := h.notifyInteraction(userId, toNotify, msg, subj, notifType, pbComment)
		notifs = append(notifs, notifyUser)
	}
	// Set notification only if the submitter is not the thread author.
	toNotify = pbThread.AuthorId
	if userId != toNotify {
		// set notification
		var msg string
		if pbComment.Upvotes > 1 {
			msg = fmt.Sprintf("%d users have upvoted a comment on your thread", pbComment.Upvotes)
		} else {
			msg = "1 user has upvoted a comment on your thread"
		}
		subj := fmt.Sprintf("On your thread %s", pbThread.Title)
		notifType := pbDataFormat.Notif_UPVOTE_COMMENT
		notifyUser := h.notifyInteraction(userId, toNotify, msg, subj, notifType, pbComment)
		notifs = append(notifs, notifyUser)
	}
	return notifs, nil
}

// UndoUpvoteComment decreases by one the number of upvotes that the comment has
// received, removes the user id from the list of voters and appends it to the
// list of undoners. It does not update the interactions of the comment.
func (h *handler) UndoUpvoteComment(userId string, comment *pbContext.Comment) error {
	var (
		commentId = comment.Id
		sectionId = comment.ThreadCtx.SectionCtx.Id
		threadId  = comment.ThreadCtx.Id
	)
	// Check whether the section exists.
	sectionDB, ok := h.sections[sectionId]
	if !ok {
		return dbmodel.ErrSectionNotFound
	}
	return sectionDB.contents.Update(func(tx *bolt.Tx) error {
		commentBytes, err := getCommentBytes(tx, threadId, commentId)
		if err != nil {
			log.Printf("Could not find comment %s in thread %s in section %s: %v.\n",
				commentId, threadId, sectionId, err)
			return err
		}
		pbComment := new(pbDataFormat.Content)
		if err = proto.Unmarshal(commentBytes, pbComment); err != nil {
			log.Printf("Could not unmarshal content: %v.\n", err)
			return err
		}
		voted, idx := inSlice(pbComment.VoterIds, userId)
		if !voted {
			return dbmodel.ErrNotUpvoted
		}
		pbComment.Upvotes--

		last := len(pbComment.VoterIds) - 1
		pbComment.VoterIds[idx] = pbComment.VoterIds[last]
		pbComment.VoterIds = pbComment.VoterIds[:last]

		undoner, _ := inSlice(pbComment.UndonerIds, userId)
		if !undoner {
			pbComment.UndonerIds = append(pbComment.UndonerIds, userId)
		}
		commentBytes, err = proto.Marshal(pbComment)
		if err != nil {
			log.Printf("Could not marshal content: %v.\n", err)
		}
		return setCommentBytes(tx, threadId, commentId, commentBytes)
	})
}

// UpvoteSubcomment increases by one the number of upvotes that the given
// subcomment has received, updates the metadata related to the interactions of
// the subcomment, the comment and the thread and saves the notifications in the
// list of unread notifications of the subcomment author, and the thread author.
//
// It returns the user id and the notification for the thread author, and for
// the subcomment author and a nil error on success, or a nil []*pbApi.NotifyUser
// and an ErrThreadNotFound, ErrCommentNotFound, ErrSubcommentNotFound or proto
// marshal/unmarshal error on failure.
//
// It may return just one *pbApi.NotifyUser if the submitter is the author of
// either the thread, or the subcomment.
func (h *handler) UpvoteSubcomment(userId string, subcomment *pbContext.Subcomment) ([]*pbApi.NotifyUser, error) {
	var (
		notifs       []*pbApi.NotifyUser
		pbSubcomment = new(pbDataFormat.Content)
		pbComment    = new(pbDataFormat.Content)
		pbThread     = new(pbDataFormat.Content)
		subcommentId = subcomment.Id
		commentId    = subcomment.CommentCtx.Id
		threadId     = subcomment.CommentCtx.ThreadCtx.Id
		sectionId    = subcomment.CommentCtx.ThreadCtx.SectionCtx.Id
	)
	// check whether the section exists
	sectionDB, ok := h.sections[sectionId]
	if !ok {
		return nil, dbmodel.ErrSectionNotFound
	}
	err := sectionDB.contents.Update(func(tx *bolt.Tx) error {
		// Get thread which both the comment and subcomment belongs to.
		threadBytes, err := getThreadBytes(tx, threadId)
		if err != nil {
			log.Printf("Could not find thread %s in section %s: %v.\n", 
				threadId, sectionId, err)
			return err
		}
		if err = proto.Unmarshal(threadBytes, pbThread); err != nil {
			log.Printf("Could not unmarshal content: %v.\n", err)
			return err
		}
		// Increment interactions and calculata new average update time only
		// if this user has not undone an interaction on this content before.
		undoner, _ := inSlice(pbThread.UndonerIds, userId)
		if !undoner {
			incInteractions(pbThread.Metadata)
			threadBytes, err = proto.Marshal(pbThread)
			if err != nil {
				log.Printf("Could not marshal content: %v.\n", err)
				return err
			}
			if err = setThreadBytes(tx, threadId, threadBytes); err != nil {
				return err
			}
		}
		// Get comment which the subcomment belongs to.
		commentBytes, err := getCommentBytes(tx, threadId, commentId)
		if err != nil {
			log.Printf("Could not find comment %s in thread %s in section %s: %v.\n",
				commentId, threadId, sectionId, err)
			return err
		}
		if err = proto.Unmarshal(commentBytes, pbComment); err != nil {
			log.Printf("Could not unmarshal content: %v.\n", err)
			return err
		}
		// Increment interactions and calculata new average update time only
		// if this user has not undone an interaction on this content before.
		undoner, _ = inSlice(pbComment.UndonerIds, userId)
		if !undoner {
			incInteractions(pbComment.Metadata)
			commentBytes, err = proto.Marshal(pbComment)
			if err = setCommentBytes(tx, threadId, commentId, commentBytes); err != nil {
				return err
			}
		}
		// Get subcomment.
		subcommentBytes, err := getSubcommentBytes(tx, threadId, commentId, subcommentId)
		if err != nil {
			log.Printf("Could not find subcomment %s in comemnt %s in thread %s in section %s: %v.\n",
				subcommentId, commentId, threadId, sectionId, err)
			return err
		}
		if err = proto.Unmarshal(subcommentBytes, pbSubcomment); err != nil {
			log.Printf("Could not unmarshal content: %v.\n", err)
			return err
		}
		// Check whether the user has already upvoted this subcomment.
		if voted, _ := inSlice(pbSubcomment.VoterIds, userId); voted {
			return dbmodel.ErrUserNotAllowed
		}
		pbSubcomment.Upvotes++
		pbSubcomment.VoterIds = append(pbSubcomment.VoterIds, userId)
		// Increment interactions and calculata new average update time only
		// if this user has not undone an interaction on this content before.
		undoner, _ = inSlice(pbSubcomment.UndonerIds, userId)
		if !undoner {
			incInteractions(pbSubcomment.Metadata)
		}
		subcommentBytes, err = proto.Marshal(pbSubcomment)
		if err != nil {
			log.Printf("Could not marshal subcomment: %v\n", err)
			return err
		}
		return setSubcommentBytes(tx, threadId, commentId, subcommentId, subcommentBytes)
	})
	if err != nil {
		return nil, err
	}
	// Set notification only if the submitter is not the subcomment author.
	toNotify := pbSubcomment.AuthorId
	if userId != toNotify {
		var msg string
		if pbSubcomment.Upvotes > 1 {
			msg = fmt.Sprintf("%d users have upvoted your comment", pbSubcomment.Upvotes)
		} else {
			msg = "1 user has upvoted your comment"
		}
		subj := fmt.Sprintf("On your comment on %s", pbSubcomment.Title)
		notifType := pbDataFormat.Notif_UPVOTE_SUBCOMMENT
		notifyUser := h.notifyInteraction(userId, toNotify, msg, subj, notifType, pbSubcomment)
		notifs = append(notifs, notifyUser)
	}
	// Set notification only if the submitter is not the thread author.
	toNotify = pbThread.AuthorId
	if userId != toNotify {
		var msg string
		if pbSubcomment.Upvotes > 1 {
			msg = fmt.Sprintf("%d users have upvoted a comment in your thread", pbSubcomment.Upvotes)
		} else {
			msg = "1 user has upvoted a comment in your thread"
		}
		subj := fmt.Sprintf("On your thread %s", pbThread.Title)
		notifType := pbDataFormat.Notif_UPVOTE_SUBCOMMENT
		notifyUser := h.notifyInteraction(userId, toNotify, msg, subj, notifType, pbSubcomment)
		notifs = append(notifs, notifyUser)
	}
	return notifs, nil
}

// UndoUpvoteSubcomment decreases by one the number of upvotes that the subcomment
// has received, removes the user id from the list of voters and appends it to the
// list of undoners. It does not update the interactions of the subcomment.
func (h *handler) UndoUpvoteSubcomment(userId string, subcomment *pbContext.Subcomment) error {
	var (
		subcommentId = subcomment.Id
		commentId    = subcomment.CommentCtx.Id
		threadId     = subcomment.CommentCtx.ThreadCtx.Id
		sectionId    = subcomment.CommentCtx.ThreadCtx.SectionCtx.Id
	)
	// Check whether the section exists.
	sectionDB, ok := h.sections[sectionId]
	if !ok {
		return dbmodel.ErrSectionNotFound
	}
	return sectionDB.contents.Update(func(tx *bolt.Tx) error {
		// Get subcomment.
		subcommentBytes, err := getSubcommentBytes(tx, threadId, commentId, subcommentId)
		if err != nil {
			log.Printf("Could not find subcomment %s in comemnt %s in thread %s in section %s: %v.\n",
				subcommentId, commentId, threadId, sectionId, err)
			return err
		}
		pbSubcomment := new(pbDataFormat.Content)
		if err = proto.Unmarshal(subcommentBytes, pbSubcomment); err != nil {
			log.Printf("Could not unmarshal content: %v.\n", err)
			return err
		}
		// Check whether the user has upvoted this subcomment and get the index.
		voted, idx := inSlice(pbSubcomment.VoterIds, userId)
		if !voted {
			return dbmodel.ErrNotUpvoted
		}
		pbSubcomment.Upvotes--
		last := len(pbSubcomment.VoterIds) - 1
		pbSubcomment.VoterIds[idx] = pbSubcomment.VoterIds[last]
		pbSubcomment.VoterIds = pbSubcomment.VoterIds[:last]

		undoner, _ := inSlice(pbSubcomment.UndonerIds, userId)
		if !undoner {
			pbSubcomment.UndonerIds = append(pbSubcomment.UndonerIds, userId)
		}
		// Update subcomment.
		subcommentBytes, err = proto.Marshal(pbSubcomment)
		if err != nil {
			log.Printf("Could not marshal content: %v.\n", err)
			return err
		}
		return setSubcommentBytes(tx, threadId, commentId, subcommentId, subcommentBytes)
	})
}
