package contents

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"log"
	"strings"

	"github.com/golang/protobuf/proto"
	dbmodel "github.com/luisguve/cheroapi/internal/app/cheroapi"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	pbUsers "github.com/luisguve/cheroproto-go/userapi"
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
func (h *handler) CreateThread(content *pbApi.Content, userId string) (string, error) {
	var (
		permalink string
		section = &pbContext.Section{
			Id: h.section.id,
		}
	)

	// Save thread and user in the same transaction.
	err := h.section.contents.Update(func(tx *bolt.Tx) error {
		// Get author data.
		req := &pbUsers.GetBasicUserDataRequest{UserId: userId}
		pbUser, err := h.users.GetUserHeaderData(context.Background(), req)
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
		permalink = fmt.Sprintf("/%s/%s", h.section.id, newId)

		pbContent := &pbDataFormat.Content{
			Title:       content.Title,
			Content:     content.Content,
			FtFile:      content.FtFile,
			PublishDate: content.PublishDate,
			AuthorId:    userId,
			Id:          newId,
			SectionName: h.section.name,
			SectionId:   h.section.id,
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
		err = activeContents.Put([]byte(newId), pbContentBytes)
		if err != nil {
			return err
		}
		threadCtx := &pbContext.Thread{
			Id:         newId,
			SectionCtx: section,
		}
		reqUpdateUser := &pbUsers.CreateThreadRequest{
			UserId:      userId,
			Ctx:         threadCtx,
			PublishDate: content.PublishDate,
		}
		// Update users' recent activity by appending a thread.
		_, err = h.users.CreateThread(context.Background(), reqUpdateUser)
		return err
	})
	if err != nil {
		return "", err
	}
	return permalink, nil
}

func (h *handler) AppendUserWhoSaved(thread *pbContext.Thread, userId string) error {
	var (
		id = thread.Id
	)

	return h.section.contents.Update(func(tx *bolt.Tx) error {
		threadBytes, err := getThreadBytes(tx, id)
		if err != nil {
			log.Printf("Could not find thread %s: %v", id, err)
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
		err = setThreadBytes(tx, id, threadBytes)
		if err != nil {
			return err
		}
		reqUpdateUser := &pbUsers.SaveThreadRequest{
			UserId: userId,
			Thread: thread,
		}
		_, err = h.users.SaveThread(context.Background(), reqUpdateUser)
		return err
	})
}

func (h *handler) RemoveUserWhoSaved(thread *pbContext.Thread, userId string) error {
	var (
		id = thread.Id
	)

	return h.section.contents.Update(func(tx *bolt.Tx) error {
		threadBytes, err := getThreadBytes(tx, id)
		if err != nil {
			log.Printf("Could not find thread %s: %v", id, err)
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
		err = setThreadBytes(tx, id, threadBytes)
		if err != nil {
			return err
		}
		reqUpdateUser := &pbUsers.RemoveSavedRequest{
			UserId: userId,
			Ctx:    thread,
		}
		_, err = h.users.RemoveSaved(context.Background(), reqUpdateUser)
		return err
	})
}
