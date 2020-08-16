package users

import (
	"context"
	"errors"

	dbmodel "github.com/luisguve/cheroapi/internal/app/userapi"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
	pbApi "github.com/luisguve/cheroproto-go/userapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SaveNotif updates the given notification if it was already there, or appends
// it to the list of unread notifications of the given user.
// If the notification was in the list of read notifications, it removes it from
// there.
func (s *Server) SaveNotif(ctx context.Context, req *pbApi.NotifyUser) (*pbApi.SaveNotifResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	var (
		userId = req.UserId
		notif  = req.Notification
	)
	err := s.dbHandler.UpdateUser(userId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
		// If an unread notification with the same Id was there before, the old
		// notification will be overriden with the new one; it's just an update.
		var found bool
		for idx, unreadNotif := range pbUser.UnreadNotifs {
			if unreadNotif.Id == notif.Id {
				found = true
				pbUser.UnreadNotifs[idx] = notif
				break
			}
		}
		// Otherwise, the new notification will be appended.
		if !found {
			pbUser.UnreadNotifs = append(pbUser.UnreadNotifs, notif)
		}
		// Find and remove the notification from the list of read notifications
		// if it was there before.
		for idx, readNotif := range pbUser.ReadNotifs {
			if readNotif.Id == notif.Id {
				last := len(pbUser.ReadNotifs) - 1
				pbUser.ReadNotifs[idx] = pbUser.ReadNotifs[last]
				pbUser.ReadNotifs = pbUser.ReadNotifs[:last]
				break
			}
		}
		return pbUser
	})
	if err != nil {
		if errors.Is(err, dbmodel.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.SaveNotifResponse{}, nil
}

// Mark unread notifications as read
func (s *Server) MarkAllAsRead(ctx context.Context, req *pbApi.ReadNotifsRequest) (*pbApi.ReadNotifsResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	err := s.dbHandler.UpdateUser(req.UserId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
		// Append read notifs to list of unread notifs to keep unread as the
		// first notifs, then set the resulting list of notifs to read notifs
		// and nil to unread notifs.
		pbUser.UnreadNotifs = append(pbUser.UnreadNotifs, pbUser.ReadNotifs...)
		pbUser.ReadNotifs = pbUser.UnreadNotifs
		pbUser.UnreadNotifs = nil
		return pbUser
	})
	if err != nil {
		if errors.Is(err, dbmodel.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.ReadNotifsResponse{}, nil
}

// Clear all the notifications
func (s *Server) ClearNotifs(ctx context.Context, req *pbApi.ClearNotifsRequest) (*pbApi.ClearNotifsResponse, error) {
	if s.dbHandler == nil {
		return nil, status.Error(codes.Internal, "No database connection")
	}
	err := s.dbHandler.UpdateUser(req.UserId, func(pbUser *pbDataFormat.User) *pbDataFormat.User {
		pbUser.ReadNotifs = nil
		pbUser.UnreadNotifs = nil
		return pbUser
	})
	if err != nil {
		if errors.Is(err, dbmodel.ErrUserNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &pbApi.ClearNotifsResponse{}, nil
}
