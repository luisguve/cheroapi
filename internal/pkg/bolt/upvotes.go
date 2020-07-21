package bolt

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
// where the user id is in the slice. Returns false an 0 if the user is not in
// users slice.
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
		pbContent *pbDataFormat.Content
	)
	// check whether the section exists
	sectionDB, ok := h.sections[sectionId]
	if !ok {
		return nil, dbmodel.ErrSectionNotFound
	}
	err := sectionDB.contents.Update(func(tx *bolt.Tx) error {
		var err error
		pbContent, err = h.GetThreadContent(thread)
		if err != nil {
			return err
		}
		if voted, _ := inSlice(pbContent.VoterIds, userId); voted {
			return dbmodel.ErrUserNotAllowed
		}
		pbContent.Upvotes++
		pbContent.VoterIds = append(pbContent.VoterIds, userId)
		// Increment interactions and calculata new average update time only if
		// this user has not undone an interaction on this content before.
		undoner, _ := inSlice(pbContent.UndonerIds, userId)
		if !undoner {
			incInteractions(pbContent.Metadata)
		}
		contentBytes, err := proto.Marshal(pbContent)
		if err != nil {
			log.Printf("Could not marshal content: %v\n", err)
			return err
		}
		return setThreadBytes(tx, id, contentBytes)
	})
	if err != nil {
		log.Println(err)
		return nil, err
	}
	// Set notification and notify user only if the submitter is not the author.
	toNotify := pbContent.AuthorId
	if userId != toNotify {
		var msg string
		if int(pbContent.Upvotes) > 1 {
			msg = fmt.Sprintf("%d users have upvoted your thread", pbContent.Upvotes)
		} else {
			msg = "1 user has upvoted your thread"
		}
		subj := fmt.Sprintf("On your thread %s", pbContent.Title)
		notifType := pbDataFormat.Notif_UPVOTE
		notifyUser := h.notifyInteraction(userId, toNotify, msg, subj, notifType, pbContent)
		return notifyUser, nil
	}
	return nil, nil
}

// UndoUpvoteThread decreases by one the number of upvotes that the thread has
// received, removes the user id from the list of voters and appends it to the
// list of undoners. It does not update the interactions of the thread.
func (h *handler) UndoUpvoteThread(userId string, thread *pbContext.Thread) error {
	pbContent, err := h.GetThreadContent(thread)
	if err != nil {
		return err
	}
	voted, idx := inSlice(pbContent.VoterIds, userId)
	if !voted {
		return dbmodel.ErrNotUpvoted
	}
	pbContent.Upvotes--

	last := len(pbContent.VoterIds) - 1
	pbContent.VoterIds[idx] = pbContent.VoterIds[last]
	pbContent.VoterIds = pbContent.VoterIds[:last]

	undoner, _ := inSlice(pbContent.UndonerIds, userId)
	if !undoner {
		pbContent.UndonerIds = append(pbContent.UndonerIds, userId)
	}
	return h.SetThreadContent(thread, pbContent)
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
		pbComment *pbDataFormat.Content
		pbThread  *pbDataFormat.Content
		notifs    []*pbApi.NotifyUser
	)
	// check whether the section exists
	sectionDB, ok := h.sections[sectionId]
	if !ok {
		return nil, dbmodel.ErrSectionNotFound
	}
	err := sectionDB.contents.Update(func(tx *bolt.Tx) error {
		var (
			commentBytes, threadBytes []byte
			done                      = make(chan error)
			quit                      = make(chan error)
		)
		// Get and update comment data.
		go func() {
			var err error
			pbComment, err = h.GetCommentContent(comment)
			if err == nil {
				if voted, _ := inSlice(pbComment.VoterIds, userId); voted {
					err = dbmodel.ErrUserNotAllowed
				} else {
					pbComment.Upvotes++
					pbComment.VoterIds = append(pbComment.VoterIds, userId)
					// Increment interactions and calculata new average update time only
					// if this user has not undone an interaction on this content before.
					undoner, _ := inSlice(pbComment.UndonerIds, userId)
					if !undoner {
						incInteractions(pbComment.Metadata)
					}
					commentBytes, err = proto.Marshal(pbComment)
				}
			}
			select {
			case done <- err:
			case <-quit:
			}
		}()
		go func() {
			var err error
			pbThread, err = h.GetThreadContent(comment.ThreadCtx)
			if err == nil {
				// increment interactions and calculata new average update time only
				// if this user has not undone an interaction on this content before.
				undoner, _ := inSlice(pbThread.UndonerIds, userId)
				if !undoner {
					incInteractions(pbThread.Metadata)
				}
				threadBytes, err = proto.Marshal(pbThread)
			}
			select {
			case done <- err:
			case <-quit:
			}
		}()
		var err error
		// Check for errors.
		for i := 0; i < 2; i++ {
			err = <-done
			if err != nil {
				close(quit)
				return err
			}
		}
		if err = setCommentBytes(tx, threadId, commentId, commentBytes); err != nil {
			return err
		}
		return setThreadBytes(tx, threadId, threadBytes)
	})
	if err != nil {
		log.Println(err)
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
	pbComment, err := h.GetCommentContent(comment)
	if err != nil {
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
	return h.SetCommentContent(comment, pbComment)
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
		notifs                            []*pbApi.NotifyUser
		pbSubcomment, pbComment, pbThread *pbDataFormat.Content
		comment                           = subcomment.CommentCtx
		thread                            = subcomment.CommentCtx.ThreadCtx
		section                           = subcomment.CommentCtx.ThreadCtx.SectionCtx
	)
	// check whether the section exists
	sectionDB, ok := h.sections[section.Id]
	if !ok {
		return nil, dbmodel.ErrSectionNotFound
	}
	err := sectionDB.contents.Update(func(tx *bolt.Tx) error {
		var (
			subcommentBytes, commentBytes, threadBytes []byte
			done                                       = make(chan error)
			quit                                       = make(chan error)
		)
		// Get and update subcomment data.
		go func() {
			var err error
			pbSubcomment, err = h.GetSubcommentContent(subcomment)
			if err == nil {
				if voted, _ := inSlice(pbSubcomment.VoterIds, userId); voted {
					err = dbmodel.ErrUserNotAllowed
				} else {
					pbSubcomment.Upvotes++
					pbSubcomment.VoterIds = append(pbSubcomment.VoterIds, userId)
					// increment interactions and calculata new average update time only
					// if this user has not undone an interaction on this content before.
					undoner, _ := inSlice(pbSubcomment.UndonerIds, userId)
					if !undoner {
						incInteractions(pbSubcomment.Metadata)
					}
					subcommentBytes, err = proto.Marshal(pbSubcomment)
				}
			}
			select {
			case done <- err:
			case <-quit:
			}
		}()
		// Get and update comment metadata.
		go func() {
			var err error
			pbComment, err = h.GetCommentContent(comment)
			if err == nil {
				// Increment interactions and calculata new average update time only
				// if this user has not undone an interaction on this content before.
				undoner, _ := inSlice(pbComment.UndonerIds, userId)
				if !undoner {
					incInteractions(pbComment.Metadata)
				}
				commentBytes, err = proto.Marshal(pbComment)
			}
			select {
			case done <- err:
			case <-quit:
			}
		}()
		// Get and update thread metadata.
		go func() {
			var err error
			pbThread, err = h.GetThreadContent(thread)
			if err == nil {
				// Increment interactions and calculata new average update time only
				// if this user has not undone an interaction on this content before.
				undoner, _ := inSlice(pbThread.UndonerIds, userId)
				if !undoner {
					incInteractions(pbThread.Metadata)
				}
				threadBytes, err = proto.Marshal(pbThread)
			}
			select {
			case done <- err:
			case <-quit:
			}
		}()
		// Check for errors.
		var err error
		for i := 0; i < 3; i++ {
			err = <-done
			if err != nil {
				close(quit)
				return err
			}
		}
		err = setSubcommentBytes(tx, thread.Id, comment.Id, subcomment.Id, subcommentBytes)
		if err != nil {
			return err
		}
		err = setCommentBytes(tx, thread.Id, comment.Id, commentBytes)
		if err != nil {
			return err
		}
		return setThreadBytes(tx, thread.Id, threadBytes)
	})
	if err != nil {
		log.Println(err)
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
	pbSubcomment, err := h.GetSubcommentContent(subcomment)
	if err != nil {
		return err
	}
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
	return h.SetSubcommentContent(subcomment, pbSubcomment)
}
