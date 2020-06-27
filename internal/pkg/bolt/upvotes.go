package bolt

import (
	"log"
	"time"
	"fmt"

	"github.com/luisguve/cheroapi/internal/pkg/dbmodel"
	bolt "go.etcd.io/bbolt"
	pbTime "github.com/golang/protobuf/ptypes/timestamp"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	pbContext "github.com/luisguve/cheroproto-go/context"
	pbMetadata "github.com/luisguve/cheroproto-go/metadata"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
)

type errNotif struct {
	err error
	notifyUser *pbApi.NotifyUser
}

// incInteractions increases by one the number of interactions of the given
// *pbMetadata.Content, sets LastUpdated to time.Now().Unix() and calculates the
// average update time difference by dividing the difference between LastUpdated
// and now by Interactions + 1
func incInteractions(m *pbMetadata.Content) {
	now := &pbTime.Timestamp{ Seconds: time.Now().Unix() }
	m.Interactions++

	lastUpdated := m.LastUpdated
	m.LastUpdated = now

	diff := now.Seconds - lastUpdated.Seconds

	m.AvgUpdateTime = float64(diff) / float64(m.Interactions)
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
	pbContent, err := h.GetThreadContent(thread)
	if err != nil {
		return nil, err
	}
	pbContent.Upvotes++
	pbContent.VoterIds = append(pbContent.VoterIds, userId)
	// Increment interactions and calculata new average update time only if
	// this user has not undone an interaction on this content before.
	undoner, _ := inSlice(pbContent.UndonerIds, userId)
	if !undoner {
		incInteractions(pbContent.Metadata)
	}

	if err = h.SetThreadContent(thread, pbContent); err != nil {
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
		notifs []*pbApi.NotifyUser
		done = make(chan errNotif)
		quit = make(chan error)
		// Buffered channel to send the comment from the go-routine that
		// updates it to the go-routine that updates the thread it belongs to.
		com = make(chan *pbDataFormat.Content, 1)
	)

	// Get and update comment data and set notification for the comment author.
	go func(com chan<- *pbDataFormat.Content) {
		var errNotifyUser errNotif
		pbComment, err := h.GetCommentContent(comment)
		if err == nil {
			pbComment.Upvotes++
			pbComment.VoterIds = append(pbComment.VoterIds, userId)
			// Increment interactions and calculata new average update time only
			// if this user has not undone an interaction on this content before.
			undoner, _ := inSlice(pbComment.UndonerIds, userId)
			if !undoner {
				incInteractions(pbComment.Metadata)
			}
			// Save comment.
			err = h.SetCommentContent(comment, pbComment)
			if (err == nil) {
				com<- pbComment
				// Set notification only if the submitter is not the comment
				// author.
				toNotify := pbComment.AuthorId
				if (userId != toNotify) {
					// set notification
					var msg string
					if pbComment.Upvotes > 1 {
						msg = fmt.Sprintf("%d users have upvoted your comment", pbComment.Upvotes)
					} else {
						msg = "1 user has upvoted your comment"
					}
					subj := fmt.Sprintf("On your comment on %s", pbComment.Title)
					notifType := pbDataFormat.Notif_UPVOTE_COMMENT
					errNotifyUser.notifyUser = h.notifyInteraction(userId, toNotify, msg, notifType, pbComment)
				}
			}
		}
		errNotifyUser.err = err
		close(upvotes)
		select {
		case done<- errNotifyUser:
		case <-quit:
		}
	}(com)
	// Get and update thread data and set notification for the thread author.
	go func(thread *pbContext.Thread, com <-chan *pbDataFormat.Content) {
		var errNotifyUser errNotif
		pbThread, err := h.GetThreadContent(thread)
		if err == nil {
			// increment interactions and calculata new average update time only
			// if this user has not undone an interaction on this content before.
			undoner, _ := inSlice(pbThread.UndonerIds, userId)
			if !undoner {
				incInteractions(pbThread.Metadata)
			}
			err = h.SetThreadContent(thread, pbThread)
			toNotify := pbThread.AuthorId
			if (err == nil) && (userId != toNotify) {
				if pbComment, ok := <-com; ok {
					// set notification
					upvotes := int(pbComment.Upvotes)
					var msg string
					if upvotes > 1 {
						msg = fmt.Sprintf("%d users have upvoted a comment on your thread", upvotes)
					} else {
						msg = "1 user has upvoted a comment on your thread"
					}
					subj := fmt.Sprintf("On your thread %s", pbThread.Title)
					notifType := pbDataFormat.Notif_UPVOTE_COMMENT
					errNotifyUser.notifyUser = h.notifyInteraction(userId, toNotify, msg, subj, notifType, pbComment)
				}
			}
		}
		errNotifyUser.err = err
		select {
		case done<- errNotifyUser:
		case <-quit:
		}
	}(comment.ThreadCtx, com)

	// check for errors
	for i := 0; i < 2; i++ {
		errNotifyUser := <-done
		if errNotifyUser.err != nil {
			log.Println(errNotifyUser.err)
			close(quit)
			return nil, errNotifyUser.err
		}
		if errNotifyUser.notifyUser != nil {
			notifs = append(notifs, errNotifyUser.notifyUser)
		}
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
		notifs []*pbApi.NotifyUser
		done = make(chan errNotif)
		quit = make(chan error)
		// Buffered channel to send the subcomment from the go-routine that
		// updates it to the go-routine that updates the thread it belongs to.
		subcom = make(chan *pbDataFormat.Content, 1)
	)
	// get and update subcomment data and set notification
	go func(subcom chan<- *pbDataFormat.Content) {
		var errNotifyUser errNotif
		pbSubcomment, err := h.GetSubcommentContent(subcomment)
		if err == nil {
			pbSubcomment.Upvotes++
			pbSubcomment.VoterIds = append(pbSubcomment.VoterIds, userId)
			// increment interactions and calculata new average update time only
			// if this user has not undone an interaction on this content before.
			undoner, _ := inSlice(pbSubcomment.UndonerIds, userId)
			if !undoner {
				incInteractions(pbSubcomment.Metadata)
			}
			err = h.SetSubcommentContent(subcomment, pbSubcomment)
			if err == nil {
				subcom<- pbSubcomment
				toNotify := pbSubcomment.AuthorId
				if userId != toNotify {
					// set notification
					var msg string
					if pbSubcomment.Upvotes > 1 {
						msg = fmt.Sprintf("%d users have upvoted your comment", pbSubcomment.Upvotes)
					} else {
						msg = "1 user has upvoted your comment"
					}
					subj := fmt.Sprintf("On your comment on %s", pbSubcomment.Title)
					notifType := pbDataFormat.Notif_UPVOTE_SUBCOMMENT
					errNotifyUser.notifyUser = h.notifyInteraction(userId, toNotify, msg, subj, notifType, pbSubcomment)
				}
			}
		}
		errNotifyUser.err = err
		close(subcom)
		select {
		case done<- errNotifyUser:
		case <-quit:
		}
	}(subcom)
	// get and update comment data
	go func(comment *pbContext.Comment) {
		pbComment, err := h.GetCommentContent(comment)
		if err == nil {
			// increment interactions and calculata new average update time only
			// if this user has not undone an interaction on this content before.
			undoner, _ := inSlice(pbComment.UndonerIds, userId)
			if !undoner {
				incInteractions(pbComment.Metadata)
			}

			err = h.SetCommentContent(comment, pbComment)
		}
		select {
		case done<- err:
		case <-quit:
		}
	}(subcomment.CommentCtx)
	// get and update thread data and set notification
	go func(thread *pbContext.Thread, subcom <-chan *pbDataFormat.Content) {
		var errNotifyUser errNotif
		pbThread, err := h.GetThreadContent(thread)
		if err == nil {
			// increment interactions and calculata new average update time only
			// if this user has not undone an interaction on this content before.
			undoner, _ := inSlice(pbThread.UndonerIds, userId)
			if !undoner {
				incInteractions(pbThread.Metadata)
			}
			err = h.SetThreadContent(thread, pbThread)
			toNotify := pbThread.AuthorId
			if (err == nil) && (userId != toNotify) {
				if pbSubcomment, ok := <-subcom; ok {
					upvotes := int(pbSubcomment.Upvotes)
					// set notification
					var msg string
					if upvotes > 1 {
						msg = fmt.Sprintf("%d users have upvoted a comment in your thread", upvotes)
					} else {
						msg = "1 user has upvoted a comment in your thread"
					}
					subj := fmt.Sprintf("On your thread %s", pbThread.Title)
					notifType := pbDataFormat.Notif_UPVOTE_SUBCOMMENT
					errNotifyUser.notifyUser = h.notifyInteraction(userId, toNotify, msg, subj, notifType, pbSubcomment)
				}
			}
		}
		errNotifyUser.err = err
		select {
		case done<- errNotifyUser:
		case <-quit:
		}
	}(subcomment.CommentCtx.ThreadCtx, subcom)

	// check for errors
	for i := 0; i < 2; i++ {
		errNotifyUser := <-done
		if errNotifyUser.err != nil {
			log.Println(errNotifyUser.err)
			close(quit)
			return nil, errNotifyUser.err
		}
		if errNotifyUser.notifyUser != nil {
			notifs = append(notifs, errNotifyUser.notifyUser)
		}
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
