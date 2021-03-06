package cheroapi

import (
	"errors"

	pbTime "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/luisguve/cheroapi/internal/pkg/patillator"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	pbContext "github.com/luisguve/cheroproto-go/context"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
)

type UpdateUserFunc func(*pbDataFormat.User) *pbDataFormat.User

// Handler defines the set of available CRUD operations to perform on the
// database.
type Handler interface {
	// Get metadata of all the active threads in a section.
	GetActiveThreadsOverview(...patillator.SetSDF) ([]patillator.SegregateDiscarderFinder, error)
	// Get metadata of the given thread ids in a section.
	GetThreadsOverview([]string, ...patillator.SetSDF) ([]patillator.SegregateDiscarderFinder, error)
	// Get content of the given thread ids in a section.
	GetThreads([]patillator.Id) ([]*pbApi.ContentRule, error)
	// Get metadata of comments in a thread.
	GetCommentsOverview(*pbContext.Thread) ([]patillator.SegregateDiscarderFinder, error)
	// Get content of the given comment ids in a thread.
	GetComments(*pbContext.Thread, []patillator.Id) ([]*pbApi.ContentRule, error)
	// Get a single ContentRule containing a thread.
	GetThread(*pbContext.Thread) (*pbApi.ContentRule, error)
	// Get a single ContentRule containing a comment.
	GetComment(*pbContext.Comment) (*pbApi.ContentRule, error)
	// Get a single ContentRule containing a subcomment.
	GetSubcomment(*pbContext.Subcomment) (*pbApi.ContentRule, error)
	// Get a single thread content.
	GetThreadContent(*pbContext.Thread) (*pbDataFormat.Content, error)
	// Get a single comment content.
	GetCommentContent(*pbContext.Comment) (*pbDataFormat.Content, error)
	// Get a single subcomment content.
	GetSubcommentContent(*pbContext.Subcomment) (*pbDataFormat.Content, error)
	// Get content of 10 subcomments, skip first n comments.
	GetSubcomments(comment *pbContext.Comment, n int) ([]*pbApi.ContentRule, error)
	// Append user id to list of users who saved.
	AppendUserWhoSaved(thread *pbContext.Thread, userId string) error
	// Remove user id from list of users who saved.
	RemoveUserWhoSaved(thread *pbContext.Thread, userId string) error
	// Submit upvote on a thread from the given user id and return a list of users
	// and the notifications for them and an error.
	UpvoteThread(userId string, thread *pbContext.Thread) (*pbApi.NotifyUser, error)
	// Submit upvote on a comment from the given user id and return a list of users
	// and the notifications for them and an error.
	UpvoteComment(userId string, comment *pbContext.Comment) ([]*pbApi.NotifyUser, error)
	// Submit upvote on a subcomment from the given user id and return a list of
	// users and the notifications for them and an error.
	UpvoteSubcomment(userId string, subcomment *pbContext.Subcomment) ([]*pbApi.NotifyUser, error)
	// Undo upvote on a thread from the given user id.
	UndoUpvoteThread(userId string, thread *pbContext.Thread) error
	// Undo upvote on a comment from the given user id.
	UndoUpvoteComment(userId string, comment *pbContext.Comment) error
	// Undo upvote on a subcomment from the given user id.
	UndoUpvoteSubcomment(userId string, subcomment *pbContext.Subcomment) error
	// Post a comment on a thread.
	ReplyThread(thread *pbContext.Thread, r Reply) (*pbApi.NotifyUser, error)
	// Post a comment on a comment.
	ReplyComment(comment *pbContext.Comment, r Reply) ([]*pbApi.NotifyUser, error)
	// Create a new thread, save it and return its permalink.
	CreateThread(content *pbApi.Content, author string) (string, error)
	// Delete the given thread and the contents associated to it.
	DeleteThread(thread *pbContext.Thread, userId string) error
	// Delete the given comment and the contents associated to it.
	DeleteComment(thread *pbContext.Comment, userId string) error
	// Delete the given subcomment and the contents associated to it.
	DeleteSubcomment(thread *pbContext.Subcomment, userId string) error
	// Return the last time a clean up was done.
	LastQA() int64
	// Clean up every section database.
	QA() (string, error)
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
)
