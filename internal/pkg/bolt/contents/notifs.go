package contents

import (
	"context"
	"fmt"
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
	req := &pbApi.NotifyUser{
		UserId:       toNotify,
		Notification: notif,
	}
	go h.users.SaveNotif(context.Background(), req)

	return req
}
