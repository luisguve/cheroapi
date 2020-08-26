package contents

import (
	"log"
	"errors"

	dbmodel "github.com/luisguve/cheroapi/internal/app/cheroapi"
	"github.com/luisguve/cheroapi/internal/pkg/patillator"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
	pbMetadata "github.com/luisguve/cheroproto-go/metadata"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Get new feed of either threads in a section or comments in a thread.
//
// In case of being called in the context of a section, it returns only
// threads that are currently active, as opposed to comments in a thread,
// in which case the type of content is considered always active, despite
// the current status of the thread they belong to.
//
// It iterates over the section specified in req.Context looking for the active
// top-level contents (threads). Once it has all the active contents of the
// section, it discards those already seen by the client, if any, specified in the
// field DiscardIds of req. Then it picks threads in a random fashion, following
// the specified Pattern of status (or expected content quality) 80% of the
// times, queries the content of these threads and finally sends them to the
// client in the stream, one by one.
//
// It may return a smaller number of contents than the specified by the client,
// depending upon the availability of contents, and it will try as much as
// possible to fulfill the Pattern of quality specified by the client, which
// also depends upon the availability of contents.
//
// It may return a codes.InvalidArgument error in case of being passed a
// request with a nil ContentContext or a codes.Internal error in case of
// a database querying or network issue.
func (s *Server) RecycleContent(req *pbApi.ContentPattern, stream pbApi.CrudCheropatilla_RecycleContentServer) error {
	if s.dbHandler == nil {
		return status.Error(codes.Internal, "No database connection")
	}
	var (
		metadata     []patillator.SegregateDiscarderFinder // active contents.
		cleanedUp    []patillator.SegregateFinder // metadata after discarding ids.
		contentIds   []patillator.Id // cleanedUp after filling the pattern.
		contentRules []*pbApi.ContentRule
		getErr1      error // get contents metadata
		getErr2      error // get contents data
		sendErr      error // stream send
	)

	// call a different getter depending upon the content context:
	// - GetActiveThreadsOverview and GetThreads in case of a section
	// - GetCommentsOverview and GetComments in case of a thread
	switch ctx := req.ContentContext.(type) {
	case *pbApi.ContentPattern_SectionCtx:
		// get threads in a section
		metadata, getErr1 = s.dbHandler.GetActiveThreadsOverview()
		// return an error only if no content could be gotten
		if (getErr1 != nil) && (len(metadata) == 0) {
			if errors.Is(getErr1, dbmodel.ErrSectionNotFound) {
				return status.Error(codes.NotFound, getErr1.Error())
			}
			return status.Error(codes.Internal, getErr1.Error())
		}
		// get rid of contents already seen by the user
		cleanedUp = patillator.DiscardContents(metadata, req.DiscardIds)
		contentIds = patillator.FillPattern(cleanedUp, req.Pattern)
		contentRules, getErr2 = s.dbHandler.GetThreads(contentIds)
	case *pbApi.ContentPattern_ThreadCtx:
		// get comments in a thread
		metadata, getErr1 = s.dbHandler.GetCommentsOverview(ctx.ThreadCtx)
		// return an error only if no content could be gotten
		if (getErr1 != nil) && (len(metadata) == 0) {
			if errors.Is(getErr1, dbmodel.ErrSectionNotFound) ||
			errors.Is(getErr1, dbmodel.ErrNoComments) {
				return status.Error(codes.NotFound, getErr1.Error())
			}
			return status.Error(codes.Internal, getErr1.Error())
		}
		// get rid of contents already seen by the user
		cleanedUp = patillator.DiscardContents(metadata, req.DiscardIds)
		contentIds = patillator.FillPattern(cleanedUp, req.Pattern)
		contentRules, getErr2 = s.dbHandler.GetComments(ctx.ThreadCtx, contentIds)
	default:
		return status.Error(
			codes.InvalidArgument,
			"A Context is required",
		)
	}
	// return an error only if no content rules could be gotten
	if (getErr2 != nil) && (len(contentRules) == 0) {
		return status.Error(codes.Internal, getErr2.Error())
	}

	for _, contentRule := range contentRules {
		if sendErr = stream.Send(contentRule); sendErr != nil {
			log.Printf("Could not send Content Rule: %v\n", sendErr)
			return status.Error(codes.Internal, sendErr.Error())
		}
	}
	// While getting the content, if an error was encountered, the content
	// returned also had to be empty for the handler to return an error through
	// status.Error.
	// A final error checking will return a well-formatted error (i.e. by
	// calling status.Error) if an error was returned by the content getters
	// but the content returned was not empty.
	switch {
	case getErr1 != nil:
		return status.Error(codes.Internal, getErr1.Error())
	case getErr2 != nil:
		return status.Error(codes.Internal, getErr2.Error())
	}
	return nil
}

func (s *Server) GetActiveThreadsOverview(req *pbApi.GetActiveThreadsOverviewRequest, stream pbApi.CrudCheropatilla_GetActiveThreadsOverviewServer) error {
	if s.dbHandler == nil {
		return status.Error(codes.Internal, "No database connection")
	}
	setContent := func(c *pbDataFormat.Content) patillator.SegregateDiscarderFinder {
		gc := pbMetadata.GeneralContent{
			SectionId: c.SectionId,
			Content:   c.Metadata,
		}
		return patillator.GeneralContent(gc)
	}
	// Get threads in the section.
	contents, err := s.dbHandler.GetActiveThreadsOverview(setContent)
	// Return an error only if no contents could be gotten.
	if (err != nil) && (len(contents) == 0) {
		return status.Error(codes.Internal, err.Error())
	}
	// Send metadata one by one.
	for _, content := range contents {
		gc, ok := content.(patillator.GeneralContent)
		if !ok {
			log.Println("GetActiveThreadsOverview: failed type assertion to patillator.GeneralContent.")
			continue
		}
		metadata := pbMetadata.GeneralContent(gc)
		if err = stream.Send(&metadata); err != nil {
			log.Println("Could not send GeneralContent:", err)
			return status.Error(codes.Internal, err.Error())
		}
	}

	return nil
}

func (s *Server) GetThreadsOverview(req *pbApi.GetThreadsOverviewRequest, stream pbApi.CrudCheropatilla_GetThreadsOverviewServer) error {
	if s.dbHandler == nil {
		return status.Error(codes.Internal, "No database connection")
	}
	setContent := func(c *pbDataFormat.Content) patillator.SegregateDiscarderFinder {
		gc := pbMetadata.GeneralContent{
			SectionId: c.SectionId,
			Content:   c.Metadata,
		}
		return patillator.GeneralContent(gc)
	}
	// Get threads in the section.
	contents, err := s.dbHandler.GetThreadsOverview(req.Ids, setContent)
	// Return an error only if no contents could be gotten.
	if (err != nil) && (len(contents) == 0) {
		return status.Error(codes.Internal, err.Error())
	}
	// Send metadata one by one.
	for _, content := range contents {
		gc, ok := content.(patillator.GeneralContent)
		if !ok {
			log.Println("GetThreadsOverview: failed type assertion to patillator.GeneralContent.")
			continue
		}
		metadata := pbMetadata.GeneralContent(gc)
		if err = stream.Send(&metadata); err != nil {
			log.Println("Could not send GeneralContent:", err)
			return status.Error(codes.Internal, err.Error())
		}
	}

	return nil
}
