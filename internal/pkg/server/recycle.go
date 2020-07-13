package server

import (
	"fmt"
	"log"
	"sync"

	"github.com/luisguve/cheroapi/internal/pkg/patillator"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
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
		metadata     []patillator.SegregateDiscarderFinder
		cleanedUp    []patillator.SegregateFinder // metadata after discarding ids
		contentIds   []patillator.Id
		contentRules []*pbApi.ContentRule
		getErr1      error // get contents metadata
		getErr2      error // get contents data
		sendErr      error // stream send
	)

	// call a different getter depending upon the content context:
	// - GetThreadsOverview and GetThreads in case of a section
	// - GetCommentsOverview and GetComments in case of a thread
	switch ctx := req.ContentContext.(type) {
	case *pbApi.ContentPattern_SectionCtx:
		// get threads in a section
		metadata, getErr1 = s.dbHandler.GetThreadsOverview(ctx.SectionCtx)
		// return an error only if no content could be gotten
		if (getErr1 != nil) && (len(metadata) == 0) {
			return status.Error(codes.Internal, getErr1.Error())
		}
		// get rid of contents already seen by the user
		cleanedUp = patillator.DiscardContents(metadata, req.DiscardIds)
		contentIds = patillator.FillPattern(cleanedUp, req.Pattern)
		contentRules, getErr2 = s.dbHandler.GetThreads(ctx.SectionCtx, contentIds)
	case *pbApi.ContentPattern_ThreadCtx:
		// get comments in a thread
		metadata, getErr1 = s.dbHandler.GetCommentsOverview(ctx.ThreadCtx)
		// return an error only if no content could be gotten
		if (getErr1 != nil) && (len(metadata) == 0) {
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

// Get new feed of threads in general (from multiple sections).
//
// It iterates over all the sections looking for the active top-level contents
// (threads). Once it has all the active contents of every section, it discards
// those already seen by the client, if any, specified in the field DiscardIds
// of req. Then it picks threads in a random fashion, following the specified
// Pattern of status (or expected content quality) in req 80% of the times,
// queries the content of these threads and finally sends them to the client in
// the stream, one by one.
//
// It may return a smaller number of contents than the specified by the client,
// depending upon the availability of contents, and it will try as much as
// possible to fulfill the Pattern of quality specified by the client, which
// also depends upon the availability of contents.
//
// It may return a codes.Internal error in case of a database querying or
// network issue.
func (s *Server) RecycleGeneral(req *pbApi.GeneralPattern, stream pbApi.CrudCheropatilla_RecycleGeneralServer) error {
	if s.dbHandler == nil {
		return status.Error(codes.Internal, "No database connection")
	}
	var (
		generalMetadata map[string][]patillator.SegregateDiscarderFinder
		cleanedUp       = make(map[string][]patillator.SegregateFinder)
		contentRules    []*pbApi.ContentRule
		getErrs1        []error // get general metadata
		getErrs2        []error // get contents data
		sendErr         error   // stream send
	)

	// Get threads in every section.
	generalMetadata, getErrs1 = s.dbHandler.GetGeneralThreadsOverview()
	// Return an error only if no content could be gotten.
	if (getErrs1 != nil) && (generalMetadata == nil) {
		// Set the first error.
		errs := fmt.Sprintf("Error 1: %v\n", getErrs1[0].Error())
		// Set the rest of errors.
		for i := 1; i < len(getErrs1); i++ {
			errs += fmt.Sprintf("Error %d: %v\n", i+1, getErrs1[i].Error())
		}
		return status.Error(codes.Internal, errs)
	}

	// Get rid of contents already seen by the user and get back a
	// []patillator.SegregateFinder for every call to DiscardContents only if
	// there are ids to be discarded.
	if req.DiscardIds != nil {
		for section, contents := range generalMetadata {
			ids := req.DiscardIds[section].Ids
			cleanedUp[section] = patillator.DiscardContents(contents, ids)
		}
	} else {
		// Type cast every content (type patillator.SegregateDiscarderFinder)
		// into a patillator.SegregateFinder.
		for section, contents := range generalMetadata {
			cleanedUp[section] = make([]patillator.SegregateFinder, len(contents))
			for i, c := range contents {
				cleanedUp[section][i] = patillator.SegregateFinder(c)
			}
		}
	}

	contentIds := patillator.FillGeneralPattern(cleanedUp, req.Pattern)
	contentRules, getErrs2 = s.dbHandler.GetGeneralThreads(contentIds)
	// Return an error only if no content rules could be gotten.
	if (getErrs2 != nil) && (len(contentRules) == 0) {
		// Set the first error.
		errs := fmt.Sprintf("Error 1: %v\n", getErrs2[0].Error())
		// Set the rest of errors.
		for i := 1; i < len(getErrs2); i++ {
			errs += fmt.Sprintf("Error %d: %v\n", i+1, getErrs2[i].Error())
		}
		return status.Error(codes.Internal, errs)
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
	case getErrs1 != nil:
		// Set the first error.
		errs := fmt.Sprintf("Error 1: %v\n", getErrs1[0].Error())
		// Set the rest of errors.
		for i := 1; i < len(getErrs1); i++ {
			errs += fmt.Sprintf("Error %d: %v\n", i+1, getErrs1[i].Error())
		}
		return status.Error(codes.Internal, errs)
	case getErrs2 != nil:
		// Set the first error.
		errs := fmt.Sprintf("Error 1: %v\n", getErrs2[0].Error())
		// Set the rest of errors.
		for i := 1; i < len(getErrs2); i++ {
			errs += fmt.Sprintf("Error %d: %v\n", i+1, getErrs2[i].Error())
		}
		return status.Error(codes.Internal, errs)
	}
	return nil
}

// Get new activity from either multiple users or a single user.
//
// It iterates over the recent activity (every kind of content) of the given
// user/users. Once it has all the activities, it discards those already seen
// by the client, if any, specified in the field DiscardIds of req. Then it
// picks activities in a random fashion, following the specified Pattern of
// status (or expected content quality) in req 80% of the times, queries the
// resulting content of these activities and finally sends them to the client
// in the stream, one by one.
//
// It may return a smaller number of contents than the specified by the client,
// depending upon the availability of contents, and it will try as much as
// possible to fulfill the Pattern of quality specified by the client, which
// also depends upon the availability of contents.
//
// It may return a codes.Internal error in case of a database querying or
// network issue.
func (s *Server) RecycleActivity(req *pbApi.ActivityPattern, stream pbApi.CrudCheropatilla_RecycleActivityServer) error {
	if s.dbHandler == nil {
		return status.Error(codes.Internal, "No database connection")
	}
	var (
		activityOverview map[string]patillator.UserActivity
		contentRules     []*pbApi.ContentRule
		getErrs1         []error // get contents overview
		getErrs2         []error // get contents data
		sendErr          error   // stream send
	)

	// assign users to a new variable
	var users []string
	switch ctx := req.Context.(type) {
	case *pbApi.ActivityPattern_Users:
		users = ctx.Users.Ids
	case *pbApi.ActivityPattern_UserId:
		users = append(users, ctx.UserId)
	default:
		return status.Error(codes.InvalidArgument, "A Context is required")
	}

	activityOverview, getErrs1 = s.dbHandler.GetActivity(users...)

	// Return an error only if it couldn't get any activity.
	if (getErrs1 != nil) && (activityOverview == nil) {
		// Set the first error.
		errs := fmt.Sprintf("Error 1: %v\n", getErrs1[0].Error())
		// Set the rest of errors.
		for i := 1; i < len(getErrs1); i++ {
			errs += fmt.Sprintf("Error %d: %v\n", i+1, getErrs1[i].Error())
		}
		return status.Error(codes.Internal, errs)
	}

	// Discard activity of every user, only if there are ids to discard.
	if req.DiscardIds != nil {
		var wg sync.WaitGroup
		var m sync.Mutex
		for user, activity := range activityOverview {
			wg.Add(1)
			go func(user string, activity patillator.UserActivity, ids *pbDataFormat.Activity) {
				defer wg.Done()
				cleanedUp := patillator.DiscardActivities(activity, ids)
				m.Lock()
				activityOverview[user] = cleanedUp
				m.Unlock()
			}(user, activity, req.DiscardIds[user])
		}
		wg.Wait()
	}

	contexts := patillator.FillActivityPattern(activityOverview, req.Pattern)
	// Get content of activity.
	contentRules, getErrs2 = s.dbHandler.GetContentsByContext(contexts)
	// Return an error only if it couldn't get any content.
	if (getErrs2 != nil) && (len(contentRules) == 0) {
		// Set the first error.
		errs := fmt.Sprintf("Error 1: %v\n", getErrs2[0].Error())
		// Set the rest of errors.
		for i := 1; i < len(getErrs2); i++ {
			errs += fmt.Sprintf("Error %d: %v\n", i+1, getErrs2[i].Error())
		}
		return status.Error(codes.Internal, errs)
	}
	// Send stream of content rules.
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
	case getErrs1 != nil:
		// Set the first error.
		errs := fmt.Sprintf("Error 1: %v\n", getErrs1[0].Error())
		// Set the rest of errors.
		for i := 1; i < len(getErrs1); i++ {
			errs += fmt.Sprintf("Error %d: %v\n", i+1, getErrs1[i].Error())
		}
		return status.Error(codes.Internal, errs)
	case getErrs2 != nil:
		// Set the first error.
		errs := fmt.Sprintf("Error 1: %v\n", getErrs2[0].Error())
		// Set the rest of errors.
		for i := 1; i < len(getErrs2); i++ {
			errs += fmt.Sprintf("Error %d: %v\n", i+1, getErrs2[i].Error())
		}
		return status.Error(codes.Internal, errs)
	}
	return nil
}

// Get new feed of saved threads of a user.
//
// This handler is pretty similar to the above handler for recycling threads from
// every section (RecycleGeneral). The only difference is the call to get the
// threads overview: in this case it calls GetSavedThreadsOverview, which takes
// in a user id to fetch saved threads metadata from, and a callback to convert a
// *pbDataFormat.Content into a patillator.SegregateDiscarderFinder.
// Everything else keeps exactly the same.
//
// It may return a smaller number of contents than the specified by the client,
// depending upon the availability of contents, and it will try as much as
// possible to fulfill the Pattern of quality specified by the client, which
// also depends upon the availability of contents.
//
// It may return a codes.Internal error in case of a database querying or
// network issue.
func (s *Server) RecycleSaved(req *pbApi.SavedPattern, stream pbApi.CrudCheropatilla_RecycleSavedServer) error {
	if s.dbHandler == nil {
		return status.Error(codes.Internal, "No database connection")
	}
	var (
		generalMetadata map[string][]patillator.SegregateDiscarderFinder
		cleanedUp       = make(map[string][]patillator.SegregateFinder)
		contentRules    []*pbApi.ContentRule
		getErrs1        []error // get general metadata
		getErrs2        []error // get contents data
		sendErr         error   // stream send
	)

	// Get threads in every section.
	generalMetadata, getErrs1 = s.dbHandler.GetSavedThreadsOverview(req.UserId)
	// Return an error only if no content could be gotten.
	if (getErrs1 != nil) && (generalMetadata == nil) {
		// Set the first error.
		errs := fmt.Sprintf("Error 1: %v\n", getErrs1[0].Error())
		// Set the rest of errors.
		for i := 1; i < len(getErrs1); i++ {
			errs += fmt.Sprintf("Error %d: %v\n", i+1, getErrs1[i].Error())
		}
		return status.Error(codes.Internal, errs)
	}

	// Get rid of contents already seen by the user and get back a
	// []patillator.SegregateFinder for every call to DiscardContents only if
	// there are ids to be discarded.
	if req.DiscardIds != nil {
		for section, contents := range generalMetadata {
			ids := req.DiscardIds[section].Ids
			cleanedUp[section] = patillator.DiscardContents(contents, ids)
		}
	} else {
		// Type cast every content (type patillator.SegregateDiscarderFinder)
		// into a patillator.SegregateFinder.
		for section, contents := range generalMetadata {
			cleanedUp[section] = make([]patillator.SegregateFinder, len(contents))
			for i, c := range contents {
				cleanedUp[section][i] = patillator.SegregateFinder(c)
			}
		}
	}

	contentIds := patillator.FillGeneralPattern(cleanedUp, req.Pattern)
	contentRules, getErrs2 = s.dbHandler.GetGeneralThreads(contentIds)
	// Return an error only if no content rules could be gotten.
	if (getErrs2 != nil) && (len(contentRules) == 0) {
		// Set the first error.
		errs := fmt.Sprintf("Error 1: %v\n", getErrs2[0].Error())
		// Set the rest of errors.
		for i := 1; i < len(getErrs2); i++ {
			errs += fmt.Sprintf("Error %d: %v\n", i+1, getErrs2[i].Error())
		}
		return status.Error(codes.Internal, errs)
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
	case getErrs1 != nil:
		// Set the first error.
		errs := fmt.Sprintf("Error 1: %v\n", getErrs1[0].Error())
		// Set the rest of errors.
		for i := 1; i < len(getErrs1); i++ {
			errs += fmt.Sprintf("Error %d: %v\n", i+1, getErrs1[i].Error())
		}
		return status.Error(codes.Internal, errs)
	case getErrs2 != nil:
		// Set the first error.
		errs := fmt.Sprintf("Error 1: %v\n", getErrs2[0].Error())
		// Set the rest of errors.
		for i := 1; i < len(getErrs2); i++ {
			errs += fmt.Sprintf("Error %d: %v\n", i+1, getErrs2[i].Error())
		}
		return status.Error(codes.Internal, errs)
	}
	return nil
}
