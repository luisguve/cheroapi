package bolt

import (
	"errors"
	"fmt"
	"log"
	"strconv"

	"github.com/golang/protobuf/proto"
	dbmodel "github.com/luisguve/cheroapi/internal/app/cheroapi"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	pbContext "github.com/luisguve/cheroproto-go/context"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
	pbMetadata "github.com/luisguve/cheroproto-go/metadata"
	bolt "go.etcd.io/bbolt"
)

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
		sectionId = thread.SectionCtx.Id
		pbComment = new(pbDataFormat.Content)
		pbThread  = new(pbDataFormat.Content)
	)
	// check whether the section exists
	sectionDB, ok := h.sections[sectionId]
	if !ok {
		return nil, dbmodel.ErrSectionNotFound
	}
	// Format, marshal and save comment and update user and thread content in
	// the same transaction.
	err := sectionDB.contents.Update(func(tx *bolt.Tx) error {
		var err error
		threadBytes, err := getThreadBytes(tx, thread.Id)
		if err != nil {
			log.Printf("Could not find thread %s in section %s: %v.\n", thread.Id, sectionId, err)
			return err
		}
		if err = proto.Unmarshal(threadBytes, pbThread); err != nil {
			log.Printf("Could not unmarshal content: %v\n", err)
			return err
		}
		commentsBucket, err := getActiveCommentsBucket(tx, thread.Id)
		if err != nil {
			if errors.Is(err, dbmodel.ErrCommentsBucketNotFound) {
				// this is the first comment; create the comments bucket
				// associated to the thread.
				commentsBucket, err = createCommentsBucket(tx, thread.Id)
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}
		// generate Id for the comment
		sequence, _ := commentsBucket.NextSequence()
		commentId := strconv.Itoa(int(sequence))
		permalink := fmt.Sprintf("%s#c_id=%s", pbThread.Permalink, commentId)

		pbComment = &pbDataFormat.Content{
			Title:       pbThread.Title,
			Content:     reply.Content,
			FtFile:      reply.FtFile,
			PublishDate: reply.PublishDate,
			AuthorId:    reply.Submitter,
			Id:          pbThread.Id,
			SectionName: sectionDB.name,
			SectionId:   sectionId,
			Permalink:   permalink,
			Metadata: &pbMetadata.Content{
				LastUpdated: reply.PublishDate,
				DataKey:     commentId,
			},
		}
		pbCommentBytes, err := proto.Marshal(pbComment)
		if err != nil {
			log.Printf("Could not marshal comment: %v\n", err)
			return err
		}
		if err = commentsBucket.Put([]byte(commentId), pbCommentBytes); err != nil {
			return err
		}
		// Update user; append comment to list of activity of user.
		err = h.UpdateUser(reply.Submitter, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
			if pbUser.RecentActivity == nil {
				pbUser.RecentActivity = new(pbDataFormat.Activity)
			}
			commentCtx := &pbContext.Comment{
				Id:        commentId,
				ThreadCtx: thread,
			}
			pbUser.RecentActivity.Comments = append(pbUser.RecentActivity.Comments, commentCtx)
			return pbUser
		})
		if err != nil {
			return err
		}
		// Update thread metadata.
		pbThread.Replies++
		pbThread.ReplierIds = append(pbThread.ReplierIds, reply.Submitter)
		incInteractions(pbThread.Metadata)
		contentBytes, err := proto.Marshal(pbThread)
		if err != nil {
			log.Printf("Could not marshal content: %v\n", err)
			return err
		}
		return setThreadBytes(tx, thread.Id, contentBytes)
	})
	if err != nil {
		return nil, err
	}

	// Set notification and notify user only if the submitter is not the author
	toNotify := pbThread.AuthorId
	if reply.Submitter != toNotify {
		replies := pbThread.Replies
		var msg string
		if replies > 1 {
			msg = fmt.Sprintf("%d users have commented out your thread", replies)
		} else {
			msg = "1 user has commented out your thread"
		}
		subj := fmt.Sprintf("On your thread %s", pbThread.Title)
		notifType := pbDataFormat.Notif_COMMENT
		// Add fragment comments to permalink.
		pbThread.Permalink += "#comments"
		notifyUser := h.notifyInteraction(reply.Submitter, toNotify, msg, subj, notifType, pbThread)

		return notifyUser, err
	}
	return nil, err
}

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
		notifyUsers []*pbApi.NotifyUser
		commentId   = comment.Id
		threadId    = comment.ThreadCtx.Id
		sectionId   = comment.ThreadCtx.SectionCtx.Id
		pbThread    = new(pbDataFormat.Content)
		pbComment   = new(pbDataFormat.Content)
	)
	// check whether the section exists
	sectionDB, ok := h.sections[sectionId]
	if !ok {
		return nil, dbmodel.ErrSectionNotFound
	}
	// Get thread, comment and user and format, marshal and save comment and
	// update user, thread and comment in the same transaction.
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
		// Get comment which the subcomment is being submitted on.
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
		subcommentsBucket, err := getActiveSubcommentsBucket(tx, threadId, commentId)
		if err != nil {
			if errors.Is(err, dbmodel.ErrSubcommentsBucketNotFound) {
				// this is the first comment; create the subcomments bucket
				// associated to the comment.
				subcommentsBucket, err = createSubcommentsBucket(tx, threadId, commentId)
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}
		// Generate Id for the subcomment.
		sequence, _ := subcommentsBucket.NextSequence()
		subcommentId := strconv.Itoa(int(sequence))
		permalink := fmt.Sprintf("%s-sc_id=%s", pbComment.Permalink, subcommentId)

		pbSubcomment := &pbDataFormat.Content{
			Title:       pbThread.Title,
			Content:     reply.Content,
			FtFile:      reply.FtFile,
			PublishDate: reply.PublishDate,
			AuthorId:    reply.Submitter,
			Id:          pbThread.Id,
			SectionName: sectionDB.name,
			SectionId:   sectionId,
			Permalink:   permalink,
			Metadata: &pbMetadata.Content{
				LastUpdated: reply.PublishDate,
				DataKey:     subcommentId,
			},
		}
		pbSubcommentBytes, err := proto.Marshal(pbSubcomment)
		if err != nil {
			log.Printf("Could not marshal comment: %v\n", err)
			return err
		}
		err = subcommentsBucket.Put([]byte(subcommentId), pbSubcommentBytes)
		if err != nil {
			return err
		}
		// Update user; append subcomment to list of activity of author.
		err = h.UpdateUser(reply.Submitter, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
			if pbUser.RecentActivity == nil {
				pbUser.RecentActivity = new(pbDataFormat.Activity)
			}
			subcommentCtx := &pbContext.Subcomment{
				Id:         subcommentId,
				CommentCtx: comment,
			}
			pbUser.RecentActivity.Subcomments = append(pbUser.RecentActivity.Subcomments, subcommentCtx)
			return pbUser
		})
		if err != nil {
			return err
		}
		// Update thread metadata.
		pbThread.Replies++
		incInteractions(pbThread.Metadata)
		threadBytes, err = proto.Marshal(pbThread)
		if err != nil {
			log.Printf("Could not marshal content: %v\n", err)
			return err
		}
		err = setThreadBytes(tx, threadId, threadBytes)
		if err != nil {
			return err
		}
		// Update comment metadata.
		pbComment.Replies++
		pbComment.ReplierIds = append(pbComment.ReplierIds, reply.Submitter)
		incInteractions(pbComment.Metadata)
		commentBytes, err = proto.Marshal(pbComment)
		if err != nil {
			log.Printf("Could not marshal content: %v\n", err)
			return err
		}
		return setCommentBytes(tx, threadId, commentId, commentBytes)
	})
	if err != nil {
		log.Println(err)
		return nil, err
	}
	// notify thread author
	toNotify := pbThread.AuthorId
	if reply.Submitter != toNotify {
		var msg string
		if pbThread.Replies > 1 {
			msg = fmt.Sprintf("%d users have commented out your thread", pbThread.Replies)
		} else {
			msg = "1 user has commented out your thread"
		}
		subj := fmt.Sprintf("On your thread %s", pbThread.Title)
		notifType := pbDataFormat.Notif_SUBCOMMENT
		notifyUser := h.notifyInteraction(reply.Submitter, toNotify, msg, subj, notifType, pbComment)

		notifyUsers = append(notifyUsers, notifyUser)
	}
	// notify comment author
	toNotify = pbComment.AuthorId
	if reply.Submitter != toNotify {
		var msg string
		if pbComment.Replies > 1 {
			msg = fmt.Sprintf("%d users have commented out your comment", pbComment.Replies)
		} else {
			msg = "1 user has commented out your comment"
		}
		subj := fmt.Sprintf("On your comment on %s", pbComment.Title)
		notifType := pbDataFormat.Notif_SUBCOMMENT
		notifyUser := h.notifyInteraction(reply.Submitter, toNotify, msg, subj, notifType, pbComment)

		notifyUsers = append(notifyUsers, notifyUser)
	}
	// notify other repliers of the comment
	for _, toNotify = range pbComment.ReplierIds {
		if reply.Submitter != toNotify {
			msg := "Other users have followed the discussion"
			subj := fmt.Sprintf("On your comment on %s", pbComment.Title)
			notifType := pbDataFormat.Notif_SUBCOMMENT
			notifyUser := h.notifyInteraction(reply.Submitter, toNotify, msg, subj, notifType, pbComment)

			notifyUsers = append(notifyUsers, notifyUser)
		}
	}
	return notifyUsers, err
}
