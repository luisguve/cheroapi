package cheroapi

import (
	"errors"

	pbTime "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/luisguve/cheroapi/internal/pkg/patillator"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	pbContext "github.com/luisguve/cheroproto-go/context"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
	"google.golang.org/grpc/status"
)

// Handler defines the set of available CRUD operations to perform on the
// database.
type Handler interface {
	// Get metadata of threads in a section
	GetThreadsOverview(*pbContext.Section, ...patillator.SetSDF) ([]patillator.SegregateDiscarderFinder, error)
	// Get content of the given thread ids in a section
	GetThreads(*pbContext.Section, []patillator.Id) ([]*pbApi.ContentRule, error)
	// Get metadata of comments in a thread
	GetCommentsOverview(*pbContext.Thread) ([]patillator.SegregateDiscarderFinder, error)
	// Get content of the given comment ids in a thread
	GetComments(*pbContext.Thread, []patillator.Id) ([]*pbApi.ContentRule, error)
	// Get metadata of threads in every section
	GetGeneralThreadsOverview() (map[string][]patillator.SegregateDiscarderFinder, []error)
	// Get content of the given thread ids in the given sections
	GetGeneralThreads([]patillator.GeneralId) ([]*pbApi.ContentRule, []error)
	// Get a single ContentRule containing a thread.
	GetThread(*pbContext.Thread) (*pbApi.ContentRule, error)
	// Get a single thread content.
	GetThreadContent(thread *pbContext.Thread) (*pbDataFormat.Content, error)
	// Get 10 content of subcomments, skip first n comments.
	GetSubcomments(comment *pbContext.Comment, n int) ([]*pbApi.ContentRule, error)
	// Get activity of users
	GetActivity(users ...string) (map[string]patillator.UserActivity, []error)
	// Get contents by context
	GetContentsByContext([]patillator.Context) ([]*pbApi.ContentRule, []error)
	// Get metadata of saved threads of a given user
	GetSavedThreadsOverview(user string) (map[string][]patillator.SegregateDiscarderFinder, []error)
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
	// Post a comment on a thread
	ReplyThread(thread *pbContext.Thread, r Reply) (*pbApi.NotifyUser, error)
	// Post a comment on a comment
	ReplyComment(comment *pbContext.Comment, r Reply) ([]*pbApi.NotifyUser, error)
	// Create a new thread, save it and return its permalink.
	CreateThread(content *pbApi.Content, section *pbContext.Section, author string) (string, error)
	// Delete the given thread and the contents associated to it.
	DeleteThread(thread *pbContext.Thread, userId string) error
	// Delete the given comment and the contents associated to it.
	DeleteComment(thread *pbContext.Comment, userId string) error
	// Delete the given subcomment and the contents associated to it.
	DeleteSubcomment(thread *pbContext.Subcomment, userId string) error
	// Get data of a user.
	User(userId string) (*pbDataFormat.User, error)
	// Associate username to user id.
	MapUsername(username, userId string) error
	// Update data of user.
	UpdateUser(pbUser *pbDataFormat.User, userId string) error
	// Get user id with the given username.
	FindUserIdByUsername(username string) ([]byte, error)
	// Get user id with the given email.
	FindUserIdByEmail(email string) ([]byte, error)
	// Return the last time a clean up was done.
	LastQA() int64
	// Clean up every section database.
	QA()
	// Release all database resources.
	Close() error
}

// Reply holds the data of a reply
type Reply struct {
	Content     string
	FtFile      string
	Submitter   string
	PublishDate *pbTime.Timestamp
}

// These errors are returned when contents are not found.
var (
	ErrUserNotFound              = errors.New("User not found")
	ErrUsernameNotFound          = errors.New("Username not found")
	ErrEmailNotFound             = errors.New("Email not found")
	ErrSectionNotFound           = errors.New("Section not found")
	ErrThreadNotFound            = errors.New("Thread not found")
	ErrCommentNotFound           = errors.New("Comment not found")
	ErrSubcommentNotFound        = errors.New("Subcomment not found")
	ErrBucketNotFound            = errors.New("Bucket not found")
	ErrCommentsBucketNotFound    = errors.New("Comments bucket not found")
	ErrSubcommentsBucketNotFound = errors.New("Subcomments bucket not found")
)

// These errors can be returned when accessing contents.
var (
	// There are no comments in the thread.
	ErrNoComments = errors.New("No comments available")
	// This user has not saved any thread yet
	ErrNoSavedThreads = errors.New("This user has not saved any thread yet")
	// Trying to index a list beyond its size.
	ErrOffsetOutOfRange = errors.New("Offset out of range")
)

// These errors can be returned when submitting actions.
var (
	// A user is trying to undo an upvote on a content he's not upvoted.
	ErrNotUpvoted = errors.New("This user has not upvoted this content")
	// A user has not the permission to do something.
	ErrUserNotAllowed = errors.New("User not allowed")
	// A user wants to change or set its username, but it's not available.
	ErrUsernameAlreadyExists = errors.New("Username already exists")
)

var SectionIds = map[string]string{
	"My Life": "mylife", /*
		"Food": "food",
		"Technology": "tech",
		"Art": "art",
		"Music": "music",
		"Do it yourself": "diy",
		"Questions": "questions",
		"Literature": "literature",*/
}
