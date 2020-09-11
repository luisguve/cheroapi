package general

import (
	"fmt"
	"log"

	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"
	dbmodel "github.com/luisguve/cheroapi/internal/app/cheroapi"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	"github.com/luisguve/cheroapi/internal/pkg/patillator"
)

// Get new feed of threads in general (from multiple sections).
//
// It iterates over all the sections looking for the active threads. Once it
// has all the active contents of every section, it discards those already seen
// by the client, if any, specified in the field DiscardIds of req. Then it
// picks threads in a random fashion, following the specified Pattern of status
// (or expected content quality) in req 80% of the times, queries the content
// of these threads and finally sends them to the client in the stream, one by
// one.
//
// It may return a smaller number of contents than the specified by the client,
// depending upon the availability of contents, and it will try as much as
// possible to fulfill the Pattern of quality specified by the client, which
// also depends upon the availability of contents.
//
// It may return a codes.Internal error in case of a database querying or
// network issue.
func (s *server) RecycleGeneral(req *pbApi.GeneralPattern, stream pbApi.CrudGeneral_RecycleGeneralServer) error {
	var (
		generalMetadata map[string][]patillator.SegregateDiscarderFinder
		cleanedUp       = make(map[string][]patillator.SegregateFinder)
		contentRules    []*pbApi.ContentRule
		getErrs1        []error // get general metadata
		getErrs2        []error // get contents data
		sendErr         error   // stream send
	)

	// Get threads overview from every section.
	generalMetadata, getErrs1 = s.getGeneralThreadsOverview()
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

	contextList := patillator.FillGeneralPattern(cleanedUp, req.Pattern)
	contentRules, getErrs2 = s.getContentsByContext(contextList)
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
// It requests the overview of the activities through the users service,
// discarding those already seen by the client, specified in the field
// DiscardIds of req. Then it picks activities in a random fashion, following
// the specified Pattern of status (or expected content quality) in req 80% of
// the times, queries the resulting content of these activities and finally
// sends them to the client in the stream, one by one.
//
// It may return a smaller number of contents than the specified by the client,
// depending upon the availability of contents, and it will try as much as
// possible to fulfill the Pattern of quality specified by the client, which
// also depends upon the availability of contents.
//
// It may return a codes.Internal error in case of a database querying or
// network issue.
func (s *server) RecycleActivity(req *pbApi.ActivityPattern, stream pbApi.CrudGeneral_RecycleActivityServer) error {
	var (
		// Activity of users classified by section id.
		activityOverview map[string]patillator.UserActivity
		contentRules     []*pbApi.ContentRule
		getErrs1         []error // get contents overview
		getErrs2         []error // get contents data
		sendErr          error   // stream send
	)

	activityOverview, getErrs1 = s.getActivity(req.Users, req.DiscardIds)

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

	contextList := patillator.FillActivityPattern(activityOverview, req.Pattern)
	// Get content of activity from each section.
	contentRules, getErrs2 = s.getContentsByContext(contextList)
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
// This handler is pretty similar to the handler for recycling threads from
// every section (RecycleGeneral). The only difference is the call to get the
// threads overview: in this case it calls GetSavedThreadsOverview, which takes
// in a user id to fetch saved threads metadata from and a list of ids to be
// discarded. It also does not discard the contents, that's done by the call
// to query the user's saved threads.
// Everything else keeps exactly the same.
//
// It may return a smaller number of contents than the specified by the client,
// depending upon the availability of contents, and it will try as much as
// possible to fulfill the Pattern of quality specified by the client, which
// also depends upon the availability of contents.
//
// It may return a codes.Internal error in case of a database querying or
// network issue.
func (s *server) RecycleSaved(req *pbApi.SavedPattern, stream pbApi.CrudGeneral_RecycleSavedServer) error {
	var (
		generalMetadata map[string][]patillator.SegregateDiscarderFinder
		cleanedUp       = make(map[string][]patillator.SegregateFinder)
		contentRules    []*pbApi.ContentRule
		getErrs1        []error // get saved threads metadata
		getErrs2        []error // get contents data
		sendErr         error   // stream send
	)

	// Get all the saved threads but those already seen by the user from the
	// users service.
	generalMetadata, getErrs1 = s.getSavedThreadsOverview(req.UserId, req.DiscardIds)
	// Return an error only if no content could be gotten.
	if (len(getErrs1) > 0) && (generalMetadata == nil) {
		// Check whether the given user has not saved threads.
		if getErrs1[0] == dbmodel.ErrNoSavedThreads {
			return status.Error(codes.InvalidArgument, getErrs1[0].Error())
		}
		// Set the first error.
		errs := fmt.Sprintf("Error 1: %v\n", getErrs1[0].Error())
		// Set the rest of errors.
		for i := 1; i < len(getErrs1); i++ {
			errs += fmt.Sprintf("Error %d: %v\n", i+1, getErrs1[i].Error())
		}
		return status.Error(codes.Internal, errs)
	}

	// Type cast every content (type patillator.SegregateDiscarderFinder)
	// into a patillator.SegregateFinder.
	for section, contents := range generalMetadata {
		cleanedUp[section] = make([]patillator.SegregateFinder, len(contents))
		for i, c := range contents {
			cleanedUp[section][i] = patillator.SegregateFinder(c)
		}
	}

	contextList := patillator.FillGeneralPattern(cleanedUp, req.Pattern)
	contentRules, getErrs2 = s.getContentsByContext(contextList)
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
