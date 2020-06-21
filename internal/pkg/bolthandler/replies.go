package bolthandler

import(
	"log"
	"encoding/binary"

	"github.com/luisguve/cheroapi/dbmodel"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	pbContext "github.com/luisguve/cheroproto-go/context"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
)

// ReplyComment performs a few tasks:
// + Creates a subcomment and associates it to the given comment.
// + Appends the newly formatted subcomment context to the list of subcomments
//   in the recent activity of the replier.
// + Updates thread metadata by incrementing its replies and interactions.
// + Updates comment metadata by incrementing its replies and interactions.
// + Append id of replier to list of repliers of the comment.
// + Formats the notifications, saves and returns them.
// 
// It may return an error if:
// - invalid section: ErrSectionNotFound
// - invalid thread context: ErrThreadNotFound
// - invalid comment context: ErrCommentNotFound
// - invalid user id: ErrUserNotFound
// - unprepared database or proto marshal/unmarshal error
func (h *handler) ReplyComment(comment *pbContext.Comment, reply dbmodel.Reply) ([]*pbApi.NotifyUser, error) {
	var (
		subcommentId string
		notifyUsers []*pbApi.NotifyUser
		threadId = comment.ThreadCtx.Id
		sectionId = comment.ThreadCtx.SectionCtx.Id
	)
	// check whether the section exists
	if sectionDB, ok := h.sections[sectionId]; !ok {
		return nil, ErrSectionNotFound
	}
	pbThread, err := h.GetThreadContent(comment.ThreadCtx)
	if err != nil {
		return nil, err
	}
	pbComment, err := h.GetCommentContent(comment)
	if err != nil {
		return nil, err
	}

	// format, marshal and save comment
	err := sectionDB.Contents.Update(func(tx *bolt.Tx) error {
		subcommentsBucket, err := getActiveSubcommentsBucket(tx, threadId, comment.Id)
		if err != nil {
			if errors.Is(err, ErrSubcommentsBucketNotFound) {
				// this is the first comment; create the subcomments bucket
				// associated to the comment.
				subcommentsBucket, err = createSubcommentsBucket(tx, threadId, comment.Id)
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}
		// generate Id for the subcomment
		sequence, _ := subcommentsBucket.NextSequence()
		subcommentId, _ = strconv.Itoa(int(sequence))
		permalink := fmt.Sprintf("%s-sc_id=%s", pbComment.Permalink, subcommentId)

		pbSubcomment := &pbDataFormat.Content{
			Title:       pbThread.Title,
			Content:     reply.Content,
			FtFile:      reply.FtFile,
			PublishDate: reply.PublishDate,
			AuthorId:    reply.Submitter,
			Id:          pbThread.Id,
			SectionName: sectionDB.Name,
			SectionId:   sectionId,
			Permalink:   permalink,
			Metadata:    &pbMetadata.Content{
				LastUpdated: reply.PublishDate,
				DataKey:     pbThread.Id,
			},
		}
		pbSubcommentBytes, err := proto.Marshal(pbSubcomment)
		if err != nil {
			return fmt.Errorf("Could not marshal comment: %v\n", err)
		}
		return subcommentsBucket.Put([]byte(subcommentId), pbSubcommentBytes)
	})
	if err != nil {
		log.Println(err)
		return nil, err
	}
	// update activity of submitter
	err = h.users.Update(func(tx *bolt.Tx) error {
		usersBucket := tx.Bucket(usersB)
		if usersBucket == nil {
			return fmt.Errorf("Bucket %s from users not found\n", usersB)
		}
		pbUserBytes := usersBucket.Get([]byte(reply.Submitter))
		if pbUserBytes == nil {
			log.Printf("Could not find user %v\n", reply.Submitter)
			return ErrUserNotFound
		}
		pbUser := new(pbDataFormat.User)
		err := proto.Unmarshal(pbUser, pbUserBytes)
		if err != nil {
			log.Printf("Could not unmarshal user %s: %v\n", reply.Submitter, err)
			return err
		}
		if pbUser.RecentActivity == nil {
			pbUser.RecentActivity = &pbDataFormat.Activity{}
		}
		subcommentCtx := &pbContext.Subcomment{
			Id: subcommentId,
			CommentCtx: comment,
		}
		pbUser.RecentActivity.Subcomments = append(pbUser.RecentActivity.Subcomments, subcommentCtx)
		pbUserBytes, err = proto.Marshal(pbUser)
		if err != nil {
			log.Printf("Could not marshal user %s: %v\n", reply.Submitter, err)
			return err
		}
		return usersBucket.Put([]byte(reply.Submitter), pbUserBytes)
	})
	if err != nil {
		log.Println(err)
		return nil, err
	}

	// Update thread metadata
	pbThread.Replies++
	incInteractions(pbThread.Metadata)
	if err = h.SetThreadContent(comment.ThreadCtx, pbThread); err != nil {
		log.Println(err)
		return nil, err
	}

	// Update comment metadata
	pbComment.Replies++
	replied, _ := inSlice(pbComment.ReplierIds, reply.Submitter)
	if !replied {
		pbComment.ReplierIds = append(pbComment.ReplierIds, reply.Submitter)
	}
	incInteractions(pbComment.Metadata)
	if err = h.SetCommentContent(comment, pbComment); err != nil {
		log.Println(err)
		return nil, err
	}

	// notify thread author
	userToNotif := pbThread.AuthorId
	if reply.Submitter != userToNotif {
		var notifMessage string
		if pbThread.Replies > 1 {
			notifMessage = fmt.Sprintf("%d users have commented out your thread", pbThread.Replies)
		} else {
			notifMessage = "1 user has commented out your thread"
		}
		notifSubject := fmt.Sprintf("On your thread %s", pbThread.Title)
		notifPermalink := pbThread.Permalink
		notifDetails := &pbDataFormat.Notif_NotifDetails{
			LastUserIdInvolved: reply.Submitter,
			Type:               pbDataFormat.Notif_SUBCOMMENT,
		}
		notifId := fmt.Sprintf("%s#%v", notifPermalink, notifDetails.Type)

		notif := &pbDataFormat.Notif{
			Message:   notifMessage,
			Subject:   notifSubject,
			Id:        notifId,
			Permalink: notifPermalink,
			Details:   notifDetails,
			Timestamp: reply.PublishDate,
		}
		go h.SaveNotif(userToNotif, notif)

		notifyUsers = append(notifyUsers, &pbApi.NotifyUser{
			UserId:       userToNotif,
			Notification: notif,
		})
	}

	// notify comment author
	userToNotif = pbComment.AuthorId
	if reply.Submitter != userToNotif {
		var notifMessage string
		if pbComment.Replies > 1 {
			notifMessage = fmt.Sprintf("%d users have commented out your comment", pbComment.Replies)
		} else {
			notifMessage = "1 user has commented out your comment"
		}
		notifSubject := fmt.Sprintf("On your comment on %s", pbComment.Title)
		notifPermalink := pbComment.Permalink
		notifDetails := &pbDataFormat.Notif_NotifDetails{
			LastUserIdInvolved: reply.Submitter,
			Type:               pbDataFormat.Notif_SUBCOMMENT,
		}
		notifId := fmt.Sprintf("%s#%v", notifPermalink, notifDetails.Type)

		notif := &pbDataFormat.Notif{
			Message:   notifMessage,
			Subject:   notifSubject,
			Id:        notifId,
			Permalink: notifPermalink,
			Details:   notifDetails,
			Timestamp: reply.PublishDate,
		}
		go h.SaveNotif(userToNotif, notif)

		notifyUsers = append(notifyUsers, &pbApi.NotifyUser{
			UserId:       userToNotif,
			Notification: notif,
		})
	}

	// notify other repliers of the comment
	for _, userToNotif = range pbComment.ReplierIds {
		if reply.Submitter != userToNotif {
			notifMessage := "Other users have followed the discussion"
			notifSubject := fmt.Sprintf("On your comment on %s", pbSubcomment.Title)
			notifPermalink := pbSubcomment.Permalink
			notifDetails := &pbDataFormat.Notif_NotifDetails{
				LastUserIdInvolved: reply.Submitter,
				Type:               pbDataFormat.Notif_SUBCOMMENT,
			}
			notifId := fmt.Sprintf("%s#%v", notifPermalink, notifDetails.Type)

			notif := &pbDataFormat.Notif{
				Message:   notifMessage,
				Subject:   notifSubject,
				Id:        notifId,
				Permalink: notifPermalink,
				Details:   notifDetails,
				Timestamp: reply.PublishDate,
			}
			go h.SaveNotif(userToNotif, notif)

			notifyUsers = append(notifyUsers, &pbApi.NotifyUser{
				UserId:       userToNotif,
				Notification: notif,
			})
		}
	}
	return notifyUsers, nil
}

// ReplyThread performs a few tasks:
// + Creates a comment and associates it to the given thread.
// + Appends the newly formatted comment context to the list of comments in the
//   recent activity of the replier.
// + Updates thread metadata by incrementing its replies and interactions.
// + Append id of replier to list of repliers of thread.
// + Formats the notification and saves it if the replier is not the author, and.
//   returns it.
// 
// It may return an error if:
// - invalid section: ErrSectionNotFound
// - invalid thread context: ErrThreadNotFound
// - invalid user id: ErrUserNotFound
// - unprepared database or proto marshal/unmarshal error
func (h *handler) ReplyThread(thread *pbContext.Thread, reply dbmodel.Reply) (*pbApi.NotifyUser, error) {
	var (
		commentId string
		sectionId = thread.SectionCtx.Id
	)
	// check whether the section exists
	if sectionDB, ok := h.sections[sectionId]; !ok {
		return nil, ErrSectionNotFound
	}

	pbThread, err := h.GetThreadContent(thread)
	if err != nil {
		return nil, err
	}
	// format, marshal and save comment
	err := sectionDB.Contents.Update(func(tx *bolt.Tx) error {
		commentsBucket, err := getActiveCommentsBucket(tx, thread.Id)
		if err != nil {
			return fmt.Errorf("Could not find comments bucket: %v\n", err)
		}
		// generate Id for the comment
		sequence, _ := commentsBucket.NextSequence()
		commentId, _ = strconv.Itoa(int(sequence))
		permalink := fmt.Sprintf("%s#c_id=%s", pbThread.Permalink, commentId)

		pbComment := &pbDataFormat.Content{
			Title:       pbThread.Title,
			Content:     reply.Content,
			FtFile:      reply.FtFile,
			PublishDate: reply.PublishDate,
			AuthorId:    reply.Submitter,
			Id:          pbThread.Id,
			SectionName: sectionDB.Name,
			SectionId:   sectionId,
			Permalink:   permalink,
			Metadata:    &pbMetadata.Content{
				LastUpdated: reply.PublishDate,
				DataKey:     pbThread.Id,
			},
		}
		pbCommentBytes, err := proto.Marshal(pbComment)
		if err != nil {
			return fmt.Errorf("Could not marshal comment: %v\n", err)
		}
		return commentsBucket.Put([]byte(commentId), pbCommentBytes)
	})
	if err != nil {
		log.Println(err)
		return nil, err
	}
	// update activity of submitter
	err = h.users.Update(func(tx *bolt.Tx) error {
		usersBucket := tx.Bucket(usersB)
		if usersBucket == nil {
			return fmt.Errorf("Bucket %s from users not found\n", usersB)
		}
		pbUserBytes := usersBucket.Get([]byte(reply.Submitter))
		if pbUserBytes == nil {
			log.Printf("Could not find user %v\n", reply.Submitter)
			return ErrUserNotFound
		}
		pbUser := new(pbDataFormat.User)
		err := proto.Unmarshal(pbUser, pbUserBytes)
		if err != nil {
			log.Printf("Could not unmarshal user %s: %v\n", reply.Submitter, err)
			return err
		}
		if pbUser.RecentActivity == nil {
			pbUser.RecentActivity = &pbDataFormat.Activity{}
		}
		commentCtx := &pbContext.Comment{
			Id: commentId,
			ThreadCtx: thread,
		}
		pbUser.RecentActivity.Comments = append(pbUser.RecentActivity.Comments, commentCtx)
		pbUserBytes, err = proto.Marshal(pbUser)
		if err != nil {
			log.Printf("Could not marshal user %s: %v\n", reply.Submitter, err)
			return err
		}
		return usersBucket.Put([]byte(reply.Submitter), pbUserBytes)
	})
	if err != nil {
		log.Println(err)
		return nil, err
	}

	// Update thread metadata
	pbThread.Replies++
	replied, _ := inSlice(pbThread.ReplierIds, reply.Submitter)
	if !replied {
		pbThread.ReplierIds = append(pbThread.ReplierIds, reply.Submitter)
	}
	incInteractions(pbThread.Metadata)
	if err = h.SetThreadContent(thread, pbThread); err != nil {
		log.Println(err)
		return nil, err
	}
	// set notification and notify user only if the submitter is not the author
	userToNotif := pbThread.AuthorId
	if reply.Submitter != userToNotif {
		var notifMessage string
		if pbThread.Replies > 1 {
			notifMessage = fmt.Sprintf("%d users have commented out your thread", pbThread.Replies)
		} else {
			notifMessage = "1 user has commented out your thread"
		}
		notifSubject := fmt.Sprintf("On your thread %s", pbThread.Title)
		notifPermalink := pbThread.Permalink
		notifDetails := &pbDataFormat.Notif_NotifDetails{
			LastUserIdInvolved: reply.Submitter,
			Type:               pbDataFormat.Notif_COMMENT,
		}
		notifId := fmt.Sprintf("%s#%v", notifPermalink, notifDetails.Type)

		notif := &pbDataFormat.Notif{
			Message:   notifMessage,
			Subject:   notifSubject,
			Id:        notifId,
			Permalink: notifPermalink,
			Details:   notifDetails,
			Timestamp: reply.PublishDate,
		}
		go h.SaveNotif(userToNotif, notif)

		return &pbApi.NotifyUser{
			UserId:       userToNotif,
			Notification: notif,
		}, nil
	}
	return nil, nil
}

// uitob returns an 8-byte big endian representation of v.
func uitob(v uint64) []byte {
    b := make([]byte, 8)
    binary.BigEndian.PutUint64(b, v)
    return b
}
