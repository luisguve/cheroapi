package bolt

import (
	"fmt"
	"log"
	"time"

	pbTime "github.com/golang/protobuf/ptypes/timestamp"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
)

// notifyInteraction formats the notification, calls SaveNotif to save it to
// the user and returns a *pbApi.NotifyUser.
func (h *handler) notifyInteraction(userId, toNotify, msg, subject string,
	notifType pbDataFormat.Notif_NotifType, pbContent *pbDataFormat.Content) *pbApi.NotifyUser {
	now := &pbTime.Timestamp{
		Seconds: time.Now().Unix(),
	}
	notifPermalink := pbContent.Permalink
	notifDetails := &pbDataFormat.Notif_NotifDetails{
		LastUserIdInvolved: userId,
		Type:               notifType,
	}
	notifId := fmt.Sprintf("%s#%v", notifPermalink, notifDetails.Type)

	notif := &pbDataFormat.Notif{
		Message:   msg,
		Subject:   subject,
		Id:        notifId,
		Permalink: notifPermalink,
		Details:   notifDetails,
		Timestamp: now,
	}
	go h.SaveNotif(toNotify, notif)

	return &pbApi.NotifyUser{
		UserId:       toNotify,
		Notification: notif,
	}
}

// SaveNotif updates the given notification if it was already there, or appends
// it to the list of unread notifications of the given user.
// If the notification was in the list of read notifications, it removes it.
func (h *handler) SaveNotif(toNotify string, notif *pbDataFormat.Notif) {
	pbUser, err := h.User(toNotify)
	if err != nil {
		log.Printf("SaveNotif get user: %v\n", err)
		return
	}
	// If a notification with the same Id was there before, the old
	// notification will be overriden with the new one; it's an update.
	var set bool
	for idx, unreadNotif := range pbUser.UnreadNotifs {
		if unreadNotif.Id == notif.Id {
			pbUser.UnreadNotifs[idx] = notif
			set = true
			break
		}
	}
	// Otherwise, the new notification will be appended.
	if !set {
		pbUser.UnreadNotifs = append(pbUser.UnreadNotifs, notif)
	}
	// Find and remove the notification from the list of read notifications.
	for idx, readNotif := range pbUser.ReadNotifs {
		if readNotif.Id == notif.Id {
			last := len(pbUser.ReadNotifs) - 1
			pbUser.ReadNotifs[idx] = pbUser.ReadNotifs[last]
			pbUser.ReadNotifs = pbUser.ReadNotifs[:last]
			break
		}
	}
	if err = h.UpdateUser(pbUser, toNotify); err != nil {
		log.Printf("SaveNotif update user: %v\n", err)
	}
}
