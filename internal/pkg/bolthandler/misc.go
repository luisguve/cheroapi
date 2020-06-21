package bolthandler

import(
	"log"

	pbApi "github.com/luisguve/cheroapi/internal/protogen/cheroapi"
	pbDataFormat "github.com/luisguve/cheroapi/internal/protogen/dataformat"
	pbMetadata "github.com/luisguve/cheroapi/internal/protogen/metadata"
	pbContext "github.com/luisguve/cheroapi/internal/protogen/context"
)

// findUserIdByUsername looks for a user id with the given username as the key
// in the bucket usernamesB from the users database of h, and returns it and a
// nil error if it could be found, or a nil []byte and an ErrUsernameNotFound
// if the username could not be found or an ErrBucketNotFound if the query could
// not be completed.
func (h *handler) findUserIdByUsername(username string) ([]byte, error) {
	var (
		userId []byte
		err error
	)
	err = h.users.View(func(tx *bolt.Tx) error {
		usernamesBucket := tx.Bucket([]byte(usernamesB))
		if usernamesBucket == nil {
			log.Printf("Bucket %s of users not found\n", usernamesB)
			return ErrBucketNotFound
		}
		userIdBytes := usernamesBucket.Get([]byte(username))
		if userIdBytes == nil {
			return ErrUsernameNotFound
		}
		copy(userId, userIdBytes)
		return nil
	})
	return userId, err
}

// findUserIdByEmail looks for a user id with the given email as the key in the
// bucket usernamesB from the users database of h, and returns it and a nil
// error if it could be found, or a nil []byte and an ErrEmailNotFound if the
// username could not be found or an ErrBucketNotFound if the query could not
// not be completed.
func (h *handler) findUserIdByEmail(email string) ([]byte, error) {
	var (
		userId []byte
		err error
	)
	err = h.users.View(func(tx *bolt.Tx) error {
		emailsBucket := tx.Bucket([]byte(emailsB))
		if emailsBucket == nil {
			log.Printf("Bucket %s of users not found\n", emailsB)
			return ErrBucketNotFound
		}
		userIdBytes := emailsBucket.Get([]byte(email))
		if userIdBytes == nil {
			return ErrEmailNotFound
		}
		copy(userId, userIdBytes)
		return nil
	})
	return userId, err
}

// formatThreadContentRule takes in a *pbDataFormat.Content and a section context
// and converts them into a *pbApi.ContentRule. It calls h.formatContentData to
// convert the *pbDataFormat.Content into a *pbApi.ContentData.
func (h *handler) formatThreadContentRule(c *pbDataFormat.Content,
	section *pbContext.Section, id string) *pbApi.ContentRule {
	data := h.formatContentData(c)
	ctx := &pbApi.ContentRule_ThreadCtx{
		ThreadCtx: &pbContext.Thread{
			SectionCtx: section,
			Id:         id,
		},
	}
	return &pbApi.ContentRule{
		Data:           data,
		ContentContext: ctx,
	}
}

// formatCommentContentRule takes in a *pbDataFormat.Content and a thread context
// and converts them into a *pbApi.ContentRule. It calls h.formatContentData to
// convert the *pbDataFormat.Content into a *pbApi.ContentData.
func (h *handler) formatCommentContentRule(c *pbDataFormat.Content,
	thread *pbContext.Thread, id string) *pbApi.ContentRule {
	data := h.formatContentData(c)
	ctx := &pbApi.ContentRule_CommentCtx{
		CommentCtx: &pbContext.Comment{
			ThreadCtx: thread,
			Id:        id,
		},
	}
	return &pbApi.ContentRule{
		Data:           data,
		ContentContext: ctx,
	}
}

// formatSubcommentContentRule takes in a *pbDataFormat.Content and a comment context
// and converts them into a *pbApi.ContentRule. It calls h.formatContentData to
// convert the *pbDataFormat.Content into a *pbApi.ContentData.
func (h *handler) formatSubcommentContentRule(c *pbDataFormat.Content,
	comment *pbContext.Comment, id string) *pbApi.ContentRule {
	data := h.formatContentData(c)
	ctx := &pbApi.ContentRule_SubcommentCtx{
		SubcommentCtx: &pbContext.Subcomment{
			CommentCtx: comment,
			Id:         id,
		},
	}
	return &pbApi.ContentRule{
		Data:           data,
		ContentContext: ctx,
	}
}

// formatContentData takes in a *pbDataFormat.Content and converts it into a 
// *pbApi.ContentData. It calls h.getContentAuthor, which queries the database
// searching the author data.
func (h *handler) formatContentData(c *pbDataFormat.Content) *pbApi.ContentData {
	author, _ := h.getContentAuthor(c.AuthorId)

	content := &pbApi.Content{
		Title:       c.Title,
		Content:     c.Content,
		FtFile:      c.FtFile,
		PublishDate: c.PublishDate,
	}

	metadata := &pbApi.ContentMetadata{
		Id:            c.Id,
		Section:       c.SectionName,
		Permalink:     c.Permalink,
		Topvotes:      c.Topvotes,
		Replies:       c.Replies,
		VoterIds:      c.VoterIds,
		ReplierIds:    c.ReplierIds,
		UsersWhoSaved: c.UsersWhoSaved,
	}

	return &pbApi.ContentData{
		Author:   author,
		Content:  content,
		Metadata: metadata,
	}
}

// getContentAuthor searches for the user with the given id in the database and
// returns a formatted *pbApi.ContentAuthor. It may return an error if the user
// was not found or it was an error while unmarshaling the bytes.
func (h *handler) getContentAuthor(id string) (*pbApi.ContentAuthor, error) {
	var (
		pbAuthor *pbApi.ContentAuthor
		pbUser *pbDataFormat.User
		err error
	)
	err = h.users.View(func(tx *bolt.Tx) error {
		users := tx.Bucket(usersB)
		if users == nil {
			log.Printf("Users bucket \"%s\" not found\n", usersB)
			return ErrBucketNotFound
		}
		userBytes := users.Get([]byte(id))
		if userBytes == nil {
			log.Printf("User (id = %s) not found\n", id)
			return ErrUserNotFound
		}
		if err = proto.Unmarshal(userBytes, pbUser); err != nil {
			log.Printf("Could not unmarshal user: %v\n", err)
			return err
		}
	})
	if err != nil {
		return nil, err
	}
	pbAuthor = &pbApi.ContentAuthor{
		Id: id,
		Username: pbUser.BasicUserData.Username,
		Alias: pbUser.BasicUserData.Alias,
	}
	return pbAuthor, nil
}

// setThreadBytes puts the given thread bytes as the value of the threadId as
// the key.
// 
// It returns a nil error on success or an ErrThreadNotFound or bolt put error
// in case of failure.
func setThreadBytes(tx *bolt.Tx, threadId string, threadBytes []byte) error {
	contents, err := getThreadBucket(tx, threadId)
	if err != nil {
		return nil, ErrThreadNotFound
	}
	return contents.Put([]byte(threadId, threadBytes))
}

// getThreadBytes returns the thread with the given Id in protobuf-encoded bytes.
// 
// It returns an ErrThreadNotFound if the thread does not exist in the database
// associated to tx.
func getThreadBytes(tx *bolt.Tx, threadId string) ([]byte, error) {
	contents, err := getThreadBucket(tx, threadId)
	if err != nil {
		return nil, ErrThreadNotFound
	}

	threadBytes := contents.Get([]byte(threadId))
	if treadBytes == nil {
		return nil, ErrThreadNotFound
	}
	return threadBytes, nil
}

// setCommentBytes puts the given comment bytes as the value of the commentId as
// the key.
// 
// It returns a nil error on success or an ErrCommentNotFound or bolt put error
// in case of failure.
func setCommentBytes(tx *bolt.Tx, threadId, commentId string, commentBytes []byte) error {
	contents, err := getCommentsBucket(tx, threadId)
	if err != nil {
		return nil, ErrCommentNotFound
	}
	return contents.Put([]byte(commentId), commentBytes)
}

// getCommentBytes returns the comment with the given Id associated to the given
// thread Id in protobuf-encoded bytes.
// 
// It returns an ErrCommentNotFound if either the thread or the comment does not
// exist in the database associated to tx.
func getCommentBytes(tx *bolt.Tx, threadId, commentId string) ([]byte, error) {
	contents, err := getCommentsBucket(tx, threadId)
	if err != nil {
		return nil, ErrCommentNotFound
	}

	commentBytes := contents.Get([]byte(commentId))
	if commentBytes == nil {
		return nil, ErrCommentNotFound
	}
	return commentBytes, nil
}

// setSubCommentBytes puts the given subcomment bytes as the value of the
// subcommentId as the key.
// 
// It returns a nil error on success or an ErrSubcommentNotFound or bolt put error
// in case of failure.
func setSubcommentBytes(tx *bolt.Tx, threadId, commentId, subcommentId string,
	subcommentBytes []byte) error {
	contents, err := getSubcommentsBucket(tx, threadId, commentId)
	if err != nil {
		return nil, ErrSubcommentNotFound
	}
	return contents.Put([]byte(subcommentId), subcommentBytes)
}

// getSubcommentBytes returns the subcomment with the given Id associated to the
// given comment Id, which is associated to the given thread Id in
// protobuf-encoded bytes.
// 
// It returns an ErrSubcommentNotFound if either the thread or the comment or the
// subcomment does not exist in the database associated to tx.
func getSubcommentBytes(tx *bolt.Tx, threadId, commentId, subcommentId string) ([]byte, error) {
	contents, err := getSubcommentsBucket(tx, threadId, commentId)
	if err != nil {
		return nil, err
	}

	subcommentBytes := contents.Get([]byte(subcommentId))
	if subcommentBytes == nil {
		return nil, ErrSubcommentNotFound
	}
	return subcommentBytes, nil
}

// getThreadBucket returns the bucket which the given thread currently belongs to;
// either the bucket of active contents or the bucket of archived contents,
// from the database associated to tx.
// 
// It returns an ErrBucketNotFound error if the thread does not exist in the 
// database associated to tx.
func getThreadBucket(tx *bolt.Tx, threadId string) (*bolt.Bucket, error) {
	// get bucket of active contents.
	contents := tx.Bucket([]byte(activeContentsB)
	if contents == nil {
		log.Printf("bucket %s not found\n", activeContentsB)
		return nil, ErrBucketNotFound
	}
	// check whether the thread is in the bucket of active contents
	threadBytes := contents.Get([]byte(threadId))
	if threadBytes != nil {
		// The thread is in the bucket of active contents.
		return contents, nil
	}

	// get bucket of archived contents.
	contents = tx.Bucket([]byte(archivedContentsB))
	if contents == nil {
		log.Printf("bucket %s not found\n", archivedContentsB)
		return nil, ErrBucketNotFound
	}
	// check whether the thread is in the bucket of archived contents
	threadBytes = contents.Get([]byte(threadId))
	if threadBytes != nil {
		// The thread is in the bucket of archived contents.
		return contents, nil
	}

	// thread not found (FOR DEBUGGING)
	log.Printf("the thread id %s could not be found\n", threadId)
	return nil, ErrBucketNotFound
}

// getActiveThreadBucket returns the bucket of active contents if the given
// thread is currently active.
// 
// It returns an ErrBucketNotFound error if the thread does not exist in the 
// bucket of active contents in the database associated to tx.
func getActiveThreadBucket(tx *bolt.Tx, threadId string) (*bolt.Bucket, error) {
	// get bucket of active contents.
	contents := tx.Bucket([]byte(activeContentsB)
	if contents == nil {
		log.Printf("bucket %s not found\n", activeContentsB)
		return nil, ErrBucketNotFound
	}
	// check whether the thread is in the bucket of active contents
	threadBytes := contents.Get([]byte(threadId))
	if threadBytes != nil {
		// The thread is in the bucket of active contents.
		return contents, nil
	}
	// the given thread is either unactive or does not exist.
	return nil, ErrBucketNotFound
}

// getCommentsBucket looks for a comments bucket associated to the given thread
// id.
// 
// It returns an ErrBucketNotFound if either the thread or the comments bucket
// does not exist in the database associated to tx.
func getCommentsBucket(tx *bolt.Tx, threadId string) (*bolt.Bucket, error) {
	contents, err := getThreadBucket(tx, threadId)
	if err != nil {
		return nil, err
	}

	commentsBucket := contents.Bucket([]byte(commentsB))
	if commentsBucket == nil {
		log.Printf("subbucket %s not found\n", commentsB)
		return nil, ErrBucketNotFound
	}

	comments := commentsBucket.Bucket([]byte(threadId))
	if comments == nil {
		log.Printf("subbucket %s of bucket %s not found\n", threadId, commentsB)
		return nil, ErrBucketNotFound
	}
	return comments, nil
}

// getActiveCommentsBucket looks for a comments bucket associated to the given
// thread id IN the bucket of active contents.
// 
// It returns an ErrBucketNotFound if either the thread or the comments bucket
// does not exist in the bucket of active contents in the database associated
// to tx.
func getActiveCommentsBucket(tx *bolt.Tx, threadId string) (*bolt.Bucket, error) {
	activeContents, err := getActiveThreadBucket(tx, threadId)
	if err != nil {
		return nil, err
	}

	commentsBucket := activeContents.Bucket([]byte(commentsB))
	if commentsBucket == nil {
		log.Printf("subbucket %s not found\n", commentsB)
		return nil, ErrBucketNotFound
	}

	comments := commentsBucket.Bucket([]byte(threadId))
	if comments == nil {
		log.Printf("subbucket %s of bucket %s not found\n", threadId, commentsB)
		return nil, ErrBucketNotFound
	}
	return comments, nil
}

// getSubcommentsBucket looks for a subcomments bucket associated to the given
// comment id, which is associated to the given thread id.
// 
// It returns an ErrBucketNotFound it either the thread or the comments bucket
// or the subcomments bucket does not exist in the database associated to tx.
func getSubcommentsBucket(tx *bolt.Tx, threadId, commentId string) (*bolt.Bucket, error) {
	contents, err := getCommentsBucket(tx, threadId)
	if err != nil {
		return nil, err
	}

	subcommentsBucket := contents.Bucket([]byte(subcommentsB))
	if subcommentsBucket == nil {
		log.Printf("subbucket %s not found\n", subcommentsB)
		return nil, ErrBucketNotFound
	}

	subcomments := subcommentsBucket.Bucket([]byte(commentId))
	if subcomments == nil {
		log.Printf("subbucket %s of bucket %s not found\n", commentId, subcommentsB)
		return nil, ErrBucketNotFound
	}
	return subcomments, nil
}

// getActiveSubcommentsBucket looks for a subcomments bucket associated to the
// given comment id, which is associated to the given thread id IN the bucket of
// active contents.
// 
// It returns an ErrBucketNotFound it either the thread or the comments bucket
// does not exist in the bucket of active contents in the database associated
// to tx.
// 
// If the subcomments bucket associated to the given comment does not exist, it
// will return an ErrSubcommentsBucketNotFound error instead.
func getActiveSubcommentsBucket(tx *bolt.Tx, threadId, commentId string) (*bolt.Bucket, error) {
	contents, err := getActiveCommentsBucket(tx, threadId)
	if err != nil {
		return nil, err
	}

	subcommentsBucket := contents.Bucket([]byte(subcommentsB))
	if subcommentsBucket == nil {
		log.Printf("subbucket %s not found\n", subcommentsB)
		return nil, ErrBucketNotFound
	}

	subcomments := subcommentsBucket.Bucket([]byte(commentId))
	if subcomments == nil {
		return nil, ErrSubcommentsBucketNotFound
	}
	return subcomments, nil
}

// createSubcommentsBucket creates a bucket for subcomments, associated to the
// given comment, which is associated to the given thread.
// 
// It returns an ErrBucketNotFound it either the thread or the comments bucket
// does not exist in the bucket of active contents in the database associated
// to tx.
func createSubcommentsBucket(tx *bolt.Tx, threadId, commentId string) (*bolt.Bucket, error) {
	contents, err := getActiveCommentsBucket(tx, threadId)
	if err != nil {
		return nil, err
	}

	subcommentsBucket := contents.Bucket([]byte(subcommentsB))
	if subcommentsBucket == nil {
		log.Printf("subbucket %s not found\n", subcommentsB)
		return nil, ErrBucketNotFound
	}
	return subcommentsBucket.CreateBucketIfNotExists([]byte(commentId))
}
