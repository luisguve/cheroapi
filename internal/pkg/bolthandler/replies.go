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
		return nil, dbmodel.ErrSectionNotFound
	}
	pbThread, err := h.GetThreadContent(comment.ThreadCtx)
	if err != nil {
		return nil, err
	}
	pbComment, err := h.GetCommentContent(comment)
	if err != nil {
		return nil, err
	}

	// Format, marshal and save comment.
	err = sectionDB.Contents.Update(func(tx *bolt.Tx) error {
		subcommentsBucket, err := getActiveSubcommentsBucket(tx, threadId, comment.Id)
		if err != nil {
			if errors.Is(err, dbmodel.ErrSubcommentsBucketNotFound) {
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
		// Generate Id for the subcomment.
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
			log.Printf("Could not marshal comment: %v\n", err)
			return err
		}
		return subcommentsBucket.Put([]byte(subcommentId), pbSubcommentBytes)
	})
	if err != nil {
		log.Println(err)
		return nil, err
	}
	var (
		done = make(chan error)
		quit = make(chan error)
	)
	// Update user; append subcomment to list of activity of author.
	go func() {
		pbUser, err := h.User(reply.Submitter)
		if err == nil {
			if pbUser.RecentActivity == nil {
				pbUser.RecentActivity = new(pbDataFormat.Activity)
			}
			subcommentCtx := &pbContext.Subcomment{
				Id:         subcommentId,
				CommentCtx: comment,
			}
			pbUser.RecentActivity.Subcomments = append(pbUser.RecentActivity.Subcomments, subcommentCtx)
			err = h.UpdateUser(pbUser, reply.Submitter)
		}
		select {
		case done<- err:
		case <-quit:
		}
	}()
	var wg sync.WaitGroup
	// Update thread metadata.
	wg.Add(1)
	go func() {
		defer wg.Done()
		pbThread.Replies++
		incInteractions(pbThread.Metadata)
		err := h.SetThreadContent(comment.ThreadCtx, pbThread) {
		select {
		case done<- err:
		case <-quit:
		}
	}()
	// Update comment metadata.
	wg.Add(1)
	go func() {
		defer wg.Done()
		pbComment.Replies++
		pbComment.ReplierIds = append(pbComment.ReplierIds, reply.Submitter)
		incInteractions(pbComment.Metadata)
		err := h.SetCommentContent(comment, pbComment) {
		select {
		case done<- err:
		case <-quit:
		}
	}()
	// Check for errors. It terminastes every go-routine hung on the statement
	// "case done<- err" by closing the channel quit and returns the first err
	// read.
	for i := 0; i < 3; i++ {
		err = <-done
		if err != nil {
			log.Println(err)
			close(quit)
			break
		}
	}
	wg.Wait()
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
		notifType := pbDataFormat.Notif_SUBCOMMENT.
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
		pbComment *pbDataFormat.Content
	)
	// check whether the section exists
	if sectionDB, ok := h.sections[sectionId]; !ok {
		return nil, dbmodel.ErrSectionNotFound
	}
	pbThread, err := h.GetThreadContent(thread)
	if err != nil {
		return nil, err
	}
	// format, marshal and save comment
	err := sectionDB.Contents.Update(func(tx *bolt.Tx) error {
		commentsBucket, err := getActiveCommentsBucket(tx, thread.Id)
		if err != nil {
			log.Printf("Could not find comments bucket: %v\n", err)
			return err
		}
		// generate Id for the comment
		sequence, _ := commentsBucket.NextSequence()
		commentId, _ = strconv.Itoa(int(sequence))
		permalink := fmt.Sprintf("%s#c_id=%s", pbThread.Permalink, commentId)

		pbComment = &pbDataFormat.Content{
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
			log.Printf("Could not marshal comment: %v\n", err)
			return err
		}
		return commentsBucket.Put([]byte(commentId), pbCommentBytes)
	})
	if err != nil {
		log.Println(err)
		return nil, err
	}
	var (
		done = make(chan error)
		quit = make(chan error)
	)
	go func() {
		// Update user; append comment to list of activity of user.
		pbUser, err := h.User(reply.Submitter)
		if err == nil {
			if pbUser.RecentActivity == nil {
				pbUser.RecentActivity = new(pbDataFormat.Activity)
			}
			commentCtx := &pbContext.Comment{
				Id:        commentId,
				ThreadCtx: thread,
			}
			pbUser.RecentActivity.Comments = append(pbUser.RecentActivity.Comments, commentCtx)
			err = h.UpdateUser(pbUser, reply.Submitter)
		}
		select {
		case done<- err:
		case <-quit:
		}
	}()
	var wg sync.WaitGroup
	// Update thread metadata.
	wg.Add(1)
	go func() {
		defer wg.Done()
		pbThread.Replies++
		replied, _ := inSlice(pbThread.ReplierIds, reply.Submitter)
		if !replied {
			pbThread.ReplierIds = append(pbThread.ReplierIds, reply.Submitter)
		}
		incInteractions(pbThread.Metadata)
		err := h.SetThreadContent(thread, pbThread)
		select {
		case done<- err:
		case <-quit:
		}
	}()
	// Check for errors. It terminastes every go-routine hung on the statement
	// "case done<- err" by closing the channel quit and returns the first err
	// read.
	for i := 0; i < 2; i++ {
		err = <-done
		if err != nil {
			log.Println(err)
			close(quit)
			break
		}
	}
	wg.Wait()
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
		notifyUser := h.notifyInteraction(reply.Submitter, toNotify, msg, subj, notifType, pbComment)

		return notifyUser, err
	}
	return nil, err
}

// uitob returns an 8-byte big endian representation of v.
func uitob(v uint64) []byte {
    b := make([]byte, 8)
    binary.BigEndian.PutUint64(b, v)
    return b
}
