package bolthandler

import (
	"log"

	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
)

// SaveNotif updates the given notification if it was already there, or appends
// it to the list of unread notifications of the given user.
func (h *handler) SaveNotif(userToNotif string, notif *pbDataFormat.Notif) {
	pbUser := new(pbDataFormat.User)

	err := h.users.View(func(tx *bolt.Tx) error {
		usersBucket := tx.Bucket([]byte(usersB))
		if usersBucket == nil {
			log.Printf("Bucket %s of users not found\n", usersB)
			return ErrBucketNotFound
		}
		userBytes := usersBucket.Get([]byte(userToNotif))
		if userBytes == nil {
			log.Printf("User \"%s\" not found\n", userToNotif)
			return ErrUserNotFound
		}
		if err := proto.Unmarshal(userBytes, pbUser); err != nil {
			log.Printf("Could not unmarshal user: %v\n", err)
			return err
		}
		return nil
	})
	if err != nil {
		log.Println("Could not save notification")
		return
	}
	// if a notification with the same Id was there before, the old notification
	// must be overriden with the new one; it's the last update.
	var set bool
	for idx, unreadNotif := range pbUser.UnreadNotifs {
		if unreadNotif.Id == notif.Id {
			pbUser.UnreadNotifs[idx] = notif
			set = true
			break
		}
	}
	// otherwise, the new notification will be appended
	if !set {
		pbUser.UnreadNotifs = append(pbUser.UnreadNotifs, notif)
	}
	userBytes, err := proto.Marshal(pbUser)
	if err != nil {
		log.Printf("Could not marhal user: %v\n", err)
		return
	}
	h.users.Update(func(tx *bolt.Tx) error{
		usersBucket := tx.Bucket([]byte(usersB))
		if usersBucket == nil {
			log.Printf("Bucket %s of users not found\n", usersB)
			return ErrBucketNotFound
		}
		if err := usersBucket.Put([]byte(userToNotif, userBytes)); err != nil {
			log.Printf("Could not put user: %v\n", err)
			return err
		}
		return nil
	})
}
