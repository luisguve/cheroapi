package bolt

import (
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"log"
	"strings"

	"github.com/golang/protobuf/proto"
	dbmodel "github.com/luisguve/cheroapi/internal/app/cheroapi"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	pbContext "github.com/luisguve/cheroproto-go/context"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
	pbMetadata "github.com/luisguve/cheroproto-go/metadata"
	bolt "go.etcd.io/bbolt"
)

// itob returns an 8-byte big endian representation of v.
func itob(v uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, v)
	return b
}

// CreateThread inserts the given content in the given section, after formatting
// it into a pbDataFormat.Content and marshaling it in protobuf-encoded bytes.
//
// It assigns the title as the thread Id after replacing spaces with dashes and
// converting it to lowercase, and builds the permalink with the format
// /{section-id}/{thread-id}.
//
// Then, it appends the just created thread to the list of threads created in
// the recent activity of the author.
func (h *handler) CreateThread(content *pbApi.Content, section *pbContext.Section, userId string) (string, error) {
	var (
		sectionId = section.Id
		permalink string
	)

	// Check whether the section exists.
	sectionDB, ok := h.sections[sectionId]
	if !ok {
		return "", dbmodel.ErrSectionNotFound
	}

	// Save thread and user in the same transaction.
	err := sectionDB.contents.Update(func(tx *bolt.Tx) error {
		// Get author data.
		pbUser, err := h.User(userId)
		if err != nil {
			return err
		}

		// The last time this user created a thread must be before the last clean
		// up.
		if pbUser.LastTimeCreated != nil {
			if !(pbUser.LastTimeCreated.Seconds < h.lastQA) {
				return dbmodel.ErrUserNotAllowed
			}
		}
		activeContents := tx.Bucket([]byte(activeContentsB))
		if activeContents == nil {
			log.Printf("Bucket %s not found\n", activeContentsB)
			return dbmodel.ErrBucketNotFound
		}
		seq, _ := activeContents.NextSequence()
		seqB := itob(seq)
		hash := sha1.New()
		hash.Write(seqB)

		hashSum := hash.Sum(nil)
		// Keep just the first 6 bytes of the hashed sequence.
		hashSeq := fmt.Sprintf("%x", hashSum[:6])

		// Build thread Id by replacing spaces with dashes, converting it to
		// lowercase and appending the hashed sequence to it.
		newId := strings.ToLower(strings.Replace(content.Title, " ", "-", -1))
		newId += fmt.Sprintf("-%s", hashSeq)
		// Build permalink: /{section-id}/{thread-id}.
		permalink = fmt.Sprintf("/%s/%s", sectionId, newId)

		pbContent := &pbDataFormat.Content{
			Title:       content.Title,
			Content:     content.Content,
			FtFile:      content.FtFile,
			PublishDate: content.PublishDate,
			AuthorId:    userId,
			Id:          newId,
			SectionName: sectionDB.name,
			SectionId:   sectionId,
			Permalink:   permalink,
			Metadata: &pbMetadata.Content{
				LastUpdated: content.PublishDate,
				DataKey:     newId,
			},
		}
		pbContentBytes, err := proto.Marshal(pbContent)
		if err != nil {
			log.Printf("Could not marshal content: %v\n", err)
			return err
		}
		err = h.UpdateUser(userId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
			if pbUser.RecentActivity == nil {
				pbUser.RecentActivity = new(pbDataFormat.Activity)
			}
			// Append new thread context to users' recent activity.
			threadCtx := &pbContext.Thread{
				Id:         newId,
				SectionCtx: section,
			}
			pbUser.RecentActivity.ThreadsCreated = append(pbUser.RecentActivity.ThreadsCreated, threadCtx)
			// Update last time created field.
			pbUser.LastTimeCreated = content.PublishDate
			return pbUser
		})
		if err != nil {
			return err
		}
		return activeContents.Put([]byte(newId), pbContentBytes)
	})
	if err != nil {
		return "", err
	}
	return permalink, nil
}

// SetThreadContent encodes the given content in protobuf bytes, then updates
// the value of the given thread with the resulting []byte.
//
// It may return an ErrSectionNotFound error in case of being called with an
// invalid section context, ErrThreadNotFound if the thread does not exist, or
// a proto marshalling error.
func (h *handler) SetThreadContent(thread *pbContext.Thread, content *pbDataFormat.Content) error {
	var (
		id        = thread.Id
		sectionId = thread.SectionCtx.Id
	)

	// check whether the section exists
	sectionDB, ok := h.sections[sectionId]
	if !ok {
		return dbmodel.ErrSectionNotFound
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

// SetCommentContent encodes the given content in protobuf bytes, then updates
// the value of the given comment with the resulting []byte.
//
// It may return an ErrSectionNotFound error in case of being called with an
// invalid section context, ErrCommentNotFound if either the comment or the
// thread it belongs to does not exist, or a proto marshalling error.
func (h *handler) SetCommentContent(comment *pbContext.Comment, content *pbDataFormat.Content) error {
	var (
		id        = comment.Id
		threadId  = comment.ThreadCtx.Id
		sectionId = comment.ThreadCtx.SectionCtx.Id
	)

	// check whether the section exists
	sectionDB, ok := h.sections[sectionId]
	if !ok {
		return dbmodel.ErrSectionNotFound
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

// SetSubcommentContent encodes the given content in protobuf bytes, then
// updates the value of the given subcomment with the resulting []byte.
//
// It may return an ErrSectionNotFound error in case of being called with an
// invalid section context, ErrSubcommentNotFound if either the subcomment, the
// comment it belongs to or the thread it belongs to does not exist, or a proto
// marshalling error.
func (h *handler) SetSubcommentContent(subcomment *pbContext.Subcomment, content *pbDataFormat.Content) error {
	var (
		id        = subcomment.Id
		commentId = subcomment.CommentCtx.Id
		threadId  = subcomment.CommentCtx.ThreadCtx.Id
		sectionId = subcomment.CommentCtx.ThreadCtx.SectionCtx.Id
	)

	// check whether the section exists
	sectionDB, ok := h.sections[sectionId]
	if !ok {
		return dbmodel.ErrSectionNotFound
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

func (h *handler) AppendUserWhoSaved(thread *pbContext.Thread, userId string) error {
	var (
		id = thread.Id
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
			log.Printf("Could not find thread (id: %s) [root]->[%s]: %v", id, sectionId, err)
			return err
		}
		pbContent := new(pbDataFormat.Content)
		if err = proto.Unmarshal(threadBytes, pbContent); err != nil {
			log.Printf("Could not unmarshal content: %v\n", err)
			return err
		}
		pbContent.UsersWhoSaved = append(pbContent.UsersWhoSaved, userId)
		threadBytes, err = proto.Marshal(pbContent)
		if err != nil {
			log.Printf("Could not marshal content: %v\n", err)
			return err
		}
		return setThreadBytes(tx, id, threadBytes)
	})
}

func (h *handler) RemoveUserWhoSaved(thread *pbContext.Thread, userId string) error {
	var (
		id = thread.Id
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
			log.Printf("Could not find thread (id: %s) [root]->[%s]: %v", id, sectionId, err)
			return err
		}
		pbContent := new(pbDataFormat.Content)
		if err = proto.Unmarshal(threadBytes, pbContent); err != nil {
			log.Printf("Could not unmarshal content: %v\n", err)
			return err
		}
		var found bool
		for i, userWhoSaved := range pbContent.UsersWhoSaved {
			if userWhoSaved == userId {
				found = true
				last := len(pbContent.UsersWhoSaved) - 1
				pbContent.UsersWhoSaved[i] = pbContent.UsersWhoSaved[last]
				pbContent.UsersWhoSaved = pbContent.UsersWhoSaved[:last]
				break
			}
		}
		if !found {
			return nil
		}
		threadBytes, err = proto.Marshal(pbContent)
		if err != nil {
			log.Printf("Could not marshal content: %v\n", err)
			return err
		}
		return setThreadBytes(tx, id, threadBytes)
	})
}
