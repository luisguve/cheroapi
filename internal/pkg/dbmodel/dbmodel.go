// package dbmodel provides the Handler interface defining a set of CRUD
// operations on any sort of DBMS, specifically designed for the cheroapi
// service.

package dbmodel

import(
	"google.golang.org/grpc/status"
	"github.com/luisguve/cheroapi/internal/pkg/patillator"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	pbMetadata "github.com/luisguve/cheroproto-go/metadata"
	pbContext "github.com/luisguve/cheroproto-go/context"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
)

// Handler defines the set of available CRUD operations to perform on the
// database.
type Handler interface {
	// Get metadata of threads in a section
	GetThreadsOverview(*pbContext.Section, func(*pbDataFormat.Content) patillator.SegregateDiscarderFinder) ([]patillator.SegregateDiscarderFinder, error)
	// Get content of the given thread ids in a section
	GetThreads(*pbContext.Section, []string) ([]*pbApi.ContentRule, error)
	// Get metadata of comments in a thread
	GetCommentsOverview(*pbContext.Thread, func(*pbDataFormat.Content) patillator.SegregateDiscarderFinder) ([]patillator.SegregateDiscarderFinder, error)
	// Get content of the given comment ids in a thread
	GetComments(*pbContext.Thread, []string) ([]*pbApi.ContentRule, error)
	// Get metadata of threads in every section
	GetGeneralThreadsOverview(func(*pbDataFormat.Content) patillator.SegregateDiscarderFinder) (map[string][]patillator.SegregateDiscarderFinder, []error)
	// Get content of the given thread ids in the given sections
	GetGeneralThreads([]patillator.GeneralId) ([]*pbApi.ContentRule, []error)
	// Get activity of users
	GetActivity(users ...string,
		func(metadata *pbMetadata.Content, ctx *pbContext.Thread) patillator.SegregateFinder,
		func(metadata *pbMetadata.Content, ctx *pbContext.Comment) patillator.SegregateFinder,
		func(metadata *pbMetadata.Content, ctx *pbContext.Subcomment) patillator.SegregateFinder)
		(map[string]patillator.UserActivity, []error)
	// Get contents by context
	GetContentsByContext([]*pbContext.Context) ([]*pbApi.ContentRule, []error)
	// Get metadata of saved threads of a given user
	GetSavedThreadsOverview(user string,
		func(metadata *pbDataFormat.Content) patillator.SegregateDiscarderFinder) (map[string][]patillator.SegregateDiscarderFinder, []error)
	// Validate user credentials (login); returns the user id and true if the user
	// credentials are valid, empty string and false otherwise.
	CheckUser(username, password string) (string, bool)
	// Add user credentials and data (sign in); returns the user id and a nil
	// *status.Status on successful registering or an empty string an a *status.Status
	// indicating what went wrong (email or username already in use) otherwise.
	RegisterUser(email, name, patillavatar, username, alias, about, password string) (string, *status.Status)
	// Submit upvote on a thread from the given user id and return a list of users
	// and the notifications for them and an error
	UpvoteThread(userId string, thread *pbContext.Thread) (*pbApi.NotifyUser, error)
	// Submit upvote on a comment from the given user id and return a list of users
	// and the notifications for them and an error
	UpvoteComment(userId string, comment *pbContext.Comment) ([]*pbApi.NotifyUser, error)
	// Submit upvote on a subcomment from the given user id and return a list of
	// users and the notifications for them and an error
	UpvoteSubcomment(userId string, subcomment *pbContext.Subcomment) ([]*pbApi.NotifyUser, error)
	// Set notif into user's list of unread notifications
	SaveNotif(userToNotif string, notif *pbDataFormat.Notif)
	// Undo upvote on a thread from the given user id
	UndoUpvoteThread(userId string, thread *pbContext.Thread) error
	// Undo upvote on a comment from the given user id
	UndoUpvoteComment(userId string, comment *pbContext.Comment) error
	// Undo upvote on a subcomment from the given user id
	UndoUpvoteSubcomment(userId string, subcomment *pbContext.Subcomment) error
}
