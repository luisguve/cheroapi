package bolthandler

import (
	"log"
	"time"
	"fmt"

	bolt "go.etcd.io/bbolt"
	pbTime "github.com/golang/protobuf/ptypes/timestamp"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	pbContext "github.com/luisguve/cheroproto-go/context"
	pbMetadata "github.com/luisguve/cheroproto-go/metadata"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
)

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
	now := &pbTime.Timestamp{
		Seconds: time.Now().Unix(),
	}

	pbContent, err := h.GetThreadContent(thread)
	if err != nil {
		return nil, err
	}
	pbContent.Upvotes++
	pbContent.VoterIds = append(pbContent.VoterIds, userId)
	// increment interactions and calculata new average update time only if this
	// user has not undone an interaction on this content before.
	undoner, _ := inSlice(pbContent.UndonerIds, userId)
	if !undoner {
		incInteractions(pbContent.Metadata)
	}

	if err = h.SetThreadContent(thread, pbContent); err != nil {
		log.Println(err)
		return nil, err
	}
	// set notification and notify user only if the submitter is not the author
	userToNotif := pbContent.AuthorId
	if userId != userToNotif {
		var notifMessage string
		if pbContent.Upvotes > 1 {
			notifMessage = fmt.Sprintf("%d users have upvoted your thread", pbContent.Upvotes)
		} else {
			notifMessage = "1 user has upvoted your thread"
		}
		notifSubject := fmt.Sprintf("On your thread %s", pbContent.Title)
		notifPermalink := pbContent.Permalink
		notifDetails := &pbDataFormat.Notif_NotifDetails{
			LastUserIdInvolved: userId,
			Type:               pbDataFormat.Notif_UPVOTE,
		}
		notifId := fmt.Sprintf("%s#%v", notifPermalink, notifDetails.Type)

		notif := &pbDataFormat.Notif{
			Message:   notifMessage,
			Subject:   notifSubject,
			Id:        notifId,
			Permalink: notifPermalink,
			Details:   notifDetails,
			Timestamp: now,
		}
		go h.SaveNotif(userToNotif, notif)

		return &pbApi.NotifyUser{
			UserId:       userToNotif,
			Notification: notif,
		}, nil
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
	pbContent.Upvotes--
	voted, idx := inSlice(pbContent.VoterIds, userId)
	if voted {
		last := len(pbContent.VoterIds) - 1
		pbContent.VoterIds[idx] = pbContent.VoterIds[last]
		pbContent.VoterIds = pbContent.VoterIds[:last]
	}
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
		notifs [2]*pbApi.NotifyUser
		done  = make(chan error)
		quit  = make(chan error)
		upvotes = make(chan int, 1)
		now = &pbTime.Timestamp{ Seconds: time.Now().Unix() }
	)

	// get and update comment data and set notification
	go func(upvotes chan<- int) {
		pbComment, err := h.GetCommentContent(comment)
		if err == nil {
			pbComment.Upvotes++
			pbComment.VoterIds = append(pbComment.VoterIds, userId)
			// increment interactions and calculata new average update time only
			// if this user has not undone an interaction on this content before.
			undoner, _ := inSlice(pbComment.UndonerIds, userId)
			if !undoner {
				incInteractions(pbComment.Metadata)
			}
			// save content
			err = h.SetCommentContent(comment, pbComment)
			if (err == nil) {
				upvotes<- int(pbComment.Upvotes)
				userToNotif := pbComment.AuthorId
				if (userId != userToNotif) {
					// set notification
					var notifMessage string
					if pbComment.Upvotes > 1 {
						notifMessage = fmt.Sprintf("%d users have upvoted your comment", pbComment.Upvotes)
					} else {
						notifMessage = "1 user has upvoted your comment"
					}
					notifSubject := fmt.Sprintf("On your comment on %s", pbComment.Title)
					notifPermalink := pbComment.Permalink
					notifDetails := &pbDataFormat.Notif_NotifDetails{
						LastUserIdInvolved: userId,
						Type:               pbDataFormat.Notif_UPVOTE_COMMENT,
					}
					notifId := fmt.Sprintf("%s#%v", notifPermalink, notifDetails.Type)

					notif := &pbDataFormat.Notif{
						Message:   notifMessage,
						Subject:   notifSubject,
						Id:        notifId,
						Permalink: notifPermalink,
						Details:   notifDetails,
						Timestamp: now,
					}
					go h.SaveNotif(userToNotif, notif)
					notifs[0] = &pbApi.NotifyUser{
						UserId:       userToNotif,
						Notification: notif,
					}
				}
			}
		}
		close(upvotes)
		select {
		case done<- err:
		case <-quit:
		}
	}(upvotes)
	// get and update thread data and set notification
	go func(upvotes <-chan int) {
		pbThread, err := h.GetThreadContent(comment.ThreadCtx)
		if err == nil {
			// increment interactions and calculata new average update time only
			// if this user has not undone an interaction on this content before.
			undoner, _ := inSlice(pbThread.UndonerIds, userId)
			if !undoner {
				incInteractions(pbThread.Metadata)
			}
			err = h.SetThreadContent(comment.ThreadCtx, pbThread)
			userToNotif := pbThread.AuthorId
			if (err == nil) && (userId != userToNotif) {
				if contentUpvotes, ok := <-upvotes; ok {
					// set notification
					var notifMessage string
					if contentUpvotes > 1 {
						notifMessage = fmt.Sprintf("%d users have upvoted a comment on your thread", 
							contentUpvotes)
					} else {
						notifMessage = "1 user has upvoted a comment on your thread"
					}
					notifSubject := fmt.Sprintf("On your thread %s", pbThread.Title)
					notifPermalink := pbThread.Permalink
					notifDetails := &pbDataFormat.Notif_NotifDetails{
						LastUserIdInvolved: userId,
						Type:               pbDataFormat.Notif_UPVOTE_COMMENT,
					}
					notifId := fmt.Sprintf("%s#%v", notifPermalink, notifDetails.Type)
					notif := &pbDataFormat.Notif{
						Message:   notifMessage,
						Subject:   notifSubject,
						Id:        notifId,
						Permalink: notifPermalink,
						Details:   notifDetails,
						Timestamp: now,
					}
					go h.SaveNotif(userToNotif, notif)
					notifs[1] = &pbApi.NotifyUser{
						UserId:       userToNotif,
						Notification: notif,
					}
				}
			}
		}
		select {
		case done<- err:
		case <-quit:
		}
	}()

	// check for errors
	for i := 0; i < 2; i++ {
		err := <-done
		if err != nil {
			log.Println(err)
			close(quit)
			return nil, err
		}
	}
	return notifs[:], nil
}

// UndoUpvoteComment decreases by one the number of upvotes that the comment has
// received, removes the user id from the list of voters and appends it to the
// list of undoners. It does not update the interactions of the comment.
func (h *handler) UndoUpvoteComment(userId string, comment *pbContext.Comment) error {
	pbComment, err := h.GetCommentContent(comment)
	if err != nil {
		return err
	}
	pbComment.Upvotes--
	voted, idx := inSlice(pbComment.VoterIds, userId)
	if voted {
		last := len(pbComment.VoterIds) - 1
		pbComment.VoterIds[idx] = pbComment.VoterIds[last]
		pbComment.VoterIds = pbComment.VoterIds[:last]
	}
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
		notifs [2]*pbApi.NotifyUser
		done = make(chan error)
		quit = make(chan error)
		upvotes = make(chan int, 1)
		now = &pbTime.Timestamp{ Seconds: time.Now().Unix() }
	)
	// get and update subcomment data and set notification
	go func(upvotes chan<- int) {
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
				userToNotif := pbSubcomment.AuthorId
				upvotes<- int(pbSubcomment.Upvotes)
				if userId != userToNotif {
					// set notification
					var notifMessage string
					if pbSubcomment.Upvotes > 1 {
						notifMessage = fmt.Sprintf("%d users have upvoted your comment", pbSubcomment.Upvotes)
					} else {
						notifMessage = "1 user has upvoted your comment"
					}
					notifSubject := fmt.Sprintf("On your comment on %s", pbSubcomment.Title)
					notifPermalink := pbSubcomment.Permalink
					notifDetails := &pbDataFormat.Notif_NotifDetails{
						LastUserIdInvolved: userId,
						Type:               pbDataFormat.Notif_UPVOTE_SUBCOMMENT,
					}
					notifId := fmt.Sprintf("%s#%v", notifPermalink,	notifDetails.Type)

					notif := &pbDataFormat.Notif{
						Message:   notifMessage,
						Subject:   notifSubject,
						Id:        notifId,
						Permalink: notifPermalink,
						Details:   notifDetails,
						Timestamp: now,
					}
					go h.SaveNotif(userToNotif, notif)
					notifs[0] = &pbApi.NotifyUser{
						UserId:       userToNotif,
						Notification: notif,
					}
				}
			}
		}
		close(upvotes)
		select {
		case done<- err:
		case <-quit:
		}
	}(upvotes)
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
	go func(thread *pbContext.Thread, upvotes <-chan int) {
		pbThread, err := h.GetThreadContent(thread)
		if err == nil {
			// increment interactions and calculata new average update time only
			// if this user has not undone an interaction on this content before.
			undoner, _ := inSlice(pbThread.UndonerIds, userId)
			if !undoner {
				incInteractions(pbThread.Metadata)
			}
			err = h.SetThreadContent(thread, pbThread)
			userToNotif := pbThread.AuthorId
			if (err == nil) && (userId != userToNotif) {
				if contentUpvotes, ok := <-upvotes; ok {
					// set notification
					var notifMessage string
					if contentUpvotes > 1 {
						notifMessage = fmt.Sprintf("%d users have upvoted a comment in your thread", contentUpvotes)
					} else {
						notifMessage = "1 user has upvoted a comment in your thread"
					}
					notifSubject := fmt.Sprintf("On your thread %s", pbThread.Title)
					notifPermalink := pbThread.Permalink
					notifDetails := &pbDataFormat.Notif_NotifDetails{
						LastUserIdInvolved: userId,
						Type:               pbDataFormat.Notif_UPVOTE_SUBCOMMENT,
					}
					notifId := fmt.Sprintf("%s#%v", notifPermalink, notifDetails.Type)
					notif := &pbDataFormat.Notif{
						Message:   notifMessage,
						Subject:   notifSubject,
						Id:        notifId,
						Permalink: notifPermalink,
						Details:   notifDetails,
						Timestamp: now,
					}
					go h.SaveNotif(userToNotif, notif)
					notifs[1] = &pbApi.NotifyUser{
						UserId:       userToNotif,
						Notification: notif,
					}
				}
			}
		}
		select {
		case done<- err:
		case <-quit:
		}
	}(subcomment.CommentCtx.ThreadCtx, upvotes)

	// check for errors
	for i := 0; i < 2; i++ {
		err := <-done
		if err != nil {
			log.Println(err)
			close(quit)
			return nil, err
		}
	}
	return notifs[:], nil
}

// UndoUpvoteSubcomment decreases by one the number of upvotes that the subcomment
// has received, removes the user id from the list of voters and appends it to the
// list of undoners. It does not update the interactions of the subcomment.
func (h *handler) UndoUpvoteSubcomment(userId string, subcomment *pbContext.Subcomment) error {
	pbSubcomment, err := h.GetSubcommentContent(subcomment)
	if err != nil {
		return err
	}
	pbSubcomment.Upvotes--
	voted, idx := inSlice(pbSubcomment.VoterIds, userId)
	if voted {
		last := len(pbSubcomment.VoterIds) - 1
		pbSubcomment.VoterIds[idx] = pbSubcomment.VoterIds[last]
		pbSubcomment.VoterIds = pbSubcomment.VoterIds[:last]
	}
	undoner, _ := inSlice(pbSubcomment.UndonerIds, userId)
	if !undoner {
		pbSubcomment.UndonerIds = append(pbSubcomment.UndonerIds, userId)
	}
	return h.SetSubcommentContent(subcomment, pbSubcomment)
}
