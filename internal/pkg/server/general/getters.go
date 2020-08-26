package general

import (
	"context"
	"io"
	"log"
	"sync"

	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
	pbUsers "github.com/luisguve/cheroproto-go/userapi"
	pbContext "github.com/luisguve/cheroproto-go/context"
	"github.com/luisguve/cheroapi/internal/pkg/patillator"
	dbmodel "github.com/luisguve/cheroapi/internal/app/cheroapi"
)

// Get metadata of all the active threads in every section. It calls
// GetActiveThreadsOverview for each section in a concurrent fashion and returns
// a map of section ids to []patillator.SegregateDiscarderFinder and a []error
// that returns each call to GetActiveThreadsOverview.
func (s *server) getGeneralThreadsOverview() (map[string][]patillator.SegregateDiscarderFinder, []error) {
	var (
		contents map[string][]patillator.SegregateDiscarderFinder
		errs     []error
		m        sync.Mutex
		wg       sync.WaitGroup
		once     sync.Once
	)

	for sectionId, section := range s.sections {
		// Query each section client concurrently; use a Mutex to synchronize
		// write access to contents and errs slice.
		wg.Add(1)
		go func(section string, client pbApi.CrudCheropatillaClient) {
			defer wg.Done()
			var threadsOverview []patillator.SegregateDiscarderFinder

			stream, err := client.GetActiveThreadsOverview(context.Background(), &pbApi.GetActiveThreadsOverviewRequest{})
			if err != nil {
				log.Printf("Could not get threads overview: %v\n", err)
				m.Lock()
				errs = append(errs, err)
				m.Unlock()
				return
			}
			for {
				metadata, err := stream.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					log.Printf("Error receiving stream: %v\n", err)
					m.Lock()
					errs = append(errs, err)
					m.Unlock()	
					return
				}
				gc := patillator.GeneralContent(*metadata)
				threadsOverview = append(threadsOverview, gc)
			}
			// Allocate map (only once) and assign the given metadata.
			once.Do(func() {
				contents = make(map[string][]patillator.SegregateDiscarderFinder)
			})
			m.Lock()
			defer m.Unlock()
			contents[section] = threadsOverview
		}(sectionId, section.Client)
	}
	wg.Wait()
	return contents, errs
}

// Get references to all the saved threads of the given user but those already
// seen by the user, specified by ids. Then, get the metadata of them.
func (s *server) getSavedThreadsOverview(userId string, ids map[string]*pbApi.IdList) (map[string][]patillator.SegregateDiscarderFinder, []error) {
	var (
		contents map[string][]patillator.SegregateDiscarderFinder
		errs     []error
	)

	req := &pbUsers.SavedThreadsRequest{
		UserId:     userId,
		DiscardIds: ids,
	}
	savedThreads, err := s.users.SavedThreads(context.Background(), req)

	if err != nil {
		log.Println(err)
		errs = append(errs, err)
		return nil, errs
	}
	if len(savedThreads.References) == 0 {
		log.Printf("GetSavedThreadsOverview: user %s has not saved any thread.\n", userId)
		errs = append(errs, dbmodel.ErrNoSavedThreads)
		return nil, errs
	}
	var (
		m    sync.Mutex
		once sync.Once
		wg   sync.WaitGroup
	)
	// Get and set threads metadata.
	for id, threads := range savedThreads.References {
		section, ok := s.sections[id]
		if !ok {
			log.Printf("Section %s is not in sections map.\n", id)
			continue
		}
		wg.Add(1)
		// Request threads from each section concurrently.
		// Should it get an error and it will append it to errs. Otherwise, it
		// will format the content and set it to the section they belong to.
		go func(sectionId string, client pbApi.CrudCheropatillaClient, ids []string) {
			defer wg.Done()

			var threadsOverview []patillator.SegregateDiscarderFinder
			req := &pbApi.GetThreadsOverviewRequest{
				Ids: ids,
			}
			stream, err := client.GetThreadsOverview(context.Background(), req)
			if err != nil {
				log.Printf("Could not get threads overview: %v\n", err)
				m.Lock()
				errs = append(errs, err)
				m.Unlock()
				return
			}
			for {
				metadata, err := stream.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					log.Printf("Error receiving stream: %v\n", err)
					m.Lock()
					errs = append(errs, err)
					m.Unlock()
					return
				}
				gc := patillator.GeneralContent(*metadata)
				threadsOverview = append(threadsOverview, gc)
			}

			once.Do(func() {
				contents = make(map[string][]patillator.SegregateDiscarderFinder)
			})
			m.Lock()
			defer m.Unlock()
			contents[sectionId] = threadsOverview
		}(section.Id, section.Client, threads.Ids)
	}
	wg.Wait()
	return contents, errs
}

// Get activity of users discarding those that are on ids. ids should have
// activity of users, classified by user id.
// It returns activity of users classified by section id.
func (s *server) getActivity(users []string, ids map[string]*pbDataFormat.Activity) (map[string]patillator.UserActivity, []error) {
	var (
		// activity holds user activity from different users mapped to their
		// section.
		activity map[string]patillator.UserActivity
		m        sync.Mutex
		once     sync.Once
		wg       sync.WaitGroup
		errs     []error
	)
	req := &pbUsers.RecentActivityRequest{
		Users:      users,
		DiscardIds: ids,
	}
	// Get recent activity of users classified by section id.
	recent, err := s.users.RecentActivity(context.Background(), req)
	if err != nil {
		errs = append(errs, err)
		if recent == nil {
			log.Printf("getActivity: Could not get recent activity: %v\n", err)
			return nil, errs
		}
	}
	// Get activity metadata from each section.
	for id, refs := range recent.References {
		// Check whether the section is available.
		section, ok := s.sections[id]
		if !ok {
			log.Printf("Section %s is not in sections map.\n", id)
			continue
		}
		// Request contents from each section concurrently.
		wg.Add(1)
		go func(sectionId string, client pbApi.CrudCheropatillaClient, refs *pbDataFormat.Activity) {
			defer wg.Done()

			var activityOverview patillator.UserActivity

			stream, err := client.GetActivityOverview(context.Background(), refs)
			if err != nil {
				log.Printf("Could not get activity overview: %v\n", err)
				m.Lock()
				errs = append(errs, err)
				m.Unlock()
				return
			}
			for {
				contentOverview, err := stream.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					log.Printf("Error receiving stream: %v\n", err)
					m.Lock()
					errs = append(errs, err)
					m.Unlock()
					return
				}
				switch ctx := contentOverview.Context.Ctx.(type) {
				case *pbContext.Context_ThreadCtx:
					ta := patillator.ThreadActivity{
						SectionId:        sectionId,
						Thread:           ctx.ThreadCtx,
						ActivityMetadata: patillator.ActivityMetadata(*contentOverview.Metadata),
					}
					activityOverview.ThreadsCreated = append(activityOverview.ThreadsCreated, ta)
				case *pbContext.Context_CommentCtx:
					ca := patillator.CommentActivity{
						SectionId:        sectionId,
						Comment:          ctx.CommentCtx,
						ActivityMetadata: patillator.ActivityMetadata(*contentOverview.Metadata),
					}
					activityOverview.Comments = append(activityOverview.Comments, ca)
				case *pbContext.Context_SubcommentCtx:
					sca := patillator.SubcommentActivity{
						SectionId:        sectionId,
						Subcomment:       ctx.SubcommentCtx,
						ActivityMetadata: patillator.ActivityMetadata(*contentOverview.Metadata),
					}
					activityOverview.Subcomments = append(activityOverview.Subcomments, sca)
				}
			}

			once.Do(func() {
				activity = make(map[string]patillator.UserActivity)
			})
			m.Lock()
			defer m.Unlock()
			activity[sectionId] = activityOverview
		}(section.Id, section.Client, refs)
	}

	return activity, errs
}

// GetContentsByContext gets and formats the contents with the given contexts
// into a []*pbApi.ContentRule.
func (s *server) getContentsByContext(contents map[string][]patillator.Context) ([]*pbApi.ContentRule, []error) {
	var (
		errs            []error
		m               sync.Mutex
		wg              sync.WaitGroup
		numContentRules = 0
	)

	// Set number of content rules.
	for _, contextList := range contents {
		numContentRules += len(contextList)
	}

	contentRules := make([]*pbApi.ContentRule, numContentRules)

	for id, contextList := range contents {
		// Check whether the section is available.
		section, ok := s.sections[id]
		if !ok {
			log.Printf("Section %s is not in sections map.\n", id)
			continue
		}
		wg.Add(1)
		go func(client pbApi.CrudCheropatillaClient, ids []patillator.Context) {
			defer wg.Done()
			contents := make([]*pbApi.ContentContext, len(ids))
			for idx, id := range ids {
				contents[idx] = &pbApi.ContentContext{
					Key:    id.Key,
					Status: id.Status,
				}
			}
			req := &pbApi.GetContentsByContextRequest{
				Contents: contents,
			}
			stream, err := client.GetContentsByContext(context.Background(), req)
			if err != nil {
				log.Printf("Could not get activity overview: %v\n", err)
				m.Lock()
				errs = append(errs, err)
				m.Unlock()
				return
			}
			for idx := 0; true; idx++ {
				contentRule, err := stream.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					log.Printf("Error receiving stream: %v\n", err)
					m.Lock()
					errs = append(errs, err)
					m.Unlock()
					return
				}
				last := len(ids) - 1
				if idx > last {
					log.Printf("Warning: idx %v is out of range of ids %v.\n", idx, last)
					return
				}
				// Order matters: get the position in the slice of content rules
				// where this content rule must be set.
				pos := ids[idx].Index
				contentRules[pos] = contentRule
			}
		}(section.Client, contextList)
	}
	wg.Wait()
	return contentRules, errs
}
