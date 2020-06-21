package bolthandler

import (
	"log"

	"google.golang.org/protobuf/proto"
	bolt "go.etcd.io/bbolt"
	pbContext "github.com/luisguve/cheroproto-go/context"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	pbMetadata "github.com/luisguve/cheroproto-go/metadata"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
)

func (h *handler) CreateThread(content *pbApi.Content, section *pbContext.Section, userId string) (string, error) {
	var (
		sectionId = section.Id
	)

	// check whether the section exists
	if sectionDB, ok := h.sections[sectionId]; !ok {
		return ErrSectionNotFound
	}

	pbContent := &pbDataFormat.Content{
		Title: content.Title,
		Content: content.Content,
		FtFile: content.FtFile,
		PublishDate: content.PublishDate,
		AuthorId: userId,
		Id: newId,
		SectionName: sectionDB.name,
		SectionId: sectionId,
		Permalink: permalink,
		Metadata: &pbMetadata.Content{
			LastUpdated: content.PublishDate,
			DataKey: newId,
		},
	}
	pbContentBytes, err := proto.Marshal(pbContent)
	if err != nil {
		log.Printf("Could not marshal content: %v\n", err)
		return "", err
	}
	err = sectionDB.contents.Update(func(tx *bolt.Tx) error {
		activeContentsBucket := tx.Bucket(activeContentsB)
		if activeContentsBucket == nil {
			log.Printf("Bucket %s not found\n", activeContentsB)
			return 
		}
	})
}

// SetThreadContent encodes the given content using Marshal from package proto,
// then updates the value of the given thread with the resulting []byte.
// 
// It may return an ErrSectionNotFound error in case of being called with an
// invalid section context, ErrThreadNotFound if the thread does not exist, or
// a proto marshalling error.
func (h *handler) SetThreadContent(thread *pbContext.Thread, content *pbDataFormat.Content) error {
	var (
		id = thread.Id
		sectionId = thread.SectionCtx.Id
	)

	// check whether the section exists
	if sectionDB, ok := h.sections[sectionId]; !ok {
		return ErrSectionNotFound
	}

	contentBytes, err := proto.Marshal(content)
	if err != nil {
		log.Printf("Could not marshal content: %v\n", err)
		return err
	}

	err = sectionDB.contents.Update(func(tx *bolt.Tx) error {
		return setThreadBytes(tx, id, contentBytes)
	})
	return err
}

// SetCommentContent encodes the given content using Marshal from package proto,
// then updates the value of the given comment with the resulting []byte.
// 
// It may return an ErrSectionNotFound error in case of being called with an
// invalid section context, ErrCommentNotFound if either the comment or the 
// thread it belongs to does not exist, or a proto marshalling error.
func (h *handler) SetCommentContent(comment *pbContext.Comment, content *pbDataFormat.Content) error {
	var (
		id = comment.Id
		threadId = comment.ThreadCtx.Id
		sectionId = comment.ThreadCtx.SectionCtx.Id
	)

	// check whether the section exists
	if sectionDB, ok := h.sections[sectionId]; !ok {
		return ErrSectionNotFound
	}

	contentBytes, err := proto.Marshal(content)
	if err != nil {
		log.Printf("Could not marshal content: %v\n", err)
		return err
	}

	err = sectionDB.contents.Update(func(tx *bolt.Tx) error {
		return setCommentBytes(tx, threadId, id, contentBytes)
	})
	return err
}

// SetSubcommentContent encodes the given content using Marshal from package proto,
// then updates the value of the given subcomment with the resulting []byte.
// 
// It may return an ErrSectionNotFound error in case of being called with an
// invalid section context, ErrSubcommentNotFound if either the subcomment, the
// comment it belongs to or the thread it belongs to does not exist, or a proto
// marshalling error.
func (h *handler) SetSubcommentContent(subcomment *pbContext.Subcomment, content *pbDataFormat.Content) error {
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
	contentBytes, err := proto.Marshal(content)
	if err != nil {
		log.Printf("Could not marshal content: %v\n", err)
		return err
	}

	err = sectionDB.contents.Update(func(tx *bolt.Tx) error {
		return setSubcommentBytes(tx, threadId, commentId, id, contentBytes)
	})
	return err
}