// package dbmodel provides the Handler interface defining a set of CRUD
// operations on any sort of DBMS, specifically designed for the cheroapi
// service.

package dbmodel

import(
	"github.com/luisguve/cheroapi/internal/pkg/patillator"
	pbApi "github.com/luisguve/cheroapi/internal/protogen/cheropatillapb"
	pbMetadata "github.com/luisguve/cheroapi/internal/protogen/metadata"
	pbContext "github.com/luisguve/cheroapi/internal/protogen/context"
	pbDataFormat "github.com/luisguve/cheroapi/internal/protogen/dataformat"
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
}
