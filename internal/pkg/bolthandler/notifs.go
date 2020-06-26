package bolthandler

import (
	"log"

	"github.com/luisguve/cheroapi/internal/pkg/dbmodel"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
)

// SaveNotif updates the given notification if it was already there, or appends
// it to the list of unread notifications of the given user.
func (h *handler) SaveNotif(userToNotif string, notif *pbDataFormat.Notif) {
	pbUser, err := h.User(userToNotif)
	if err != nil {
		log.Printf("SaveNotif get user: %v\n", err)
		return
	}
	// if a notification with the same Id was there before, the old
	// notification will be overriden with the new one; it's an update.
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
	if err = h.UpdateUser(pbUser, userToNotif); err != nil {
		log.Printf("SaveNotif update user: %v\n", err)
	}
}
