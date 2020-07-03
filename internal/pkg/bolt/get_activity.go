package bolt

import(
	"log"
	"sync"

	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
	pbContext "github.com/luisguve/cheroproto-go/context"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	"github.com/luisguve/cheroapi/internal/pkg/patillator"
)

// Get activity of users
func (h *handler) GetActivity(users ...string) (map[string]patillator.UserActivity, []error) {
	var (
		activity map[string]patillator.UserActivity
		m sync.Mutex
		once sync.Once
		wg sync.WaitGroup
		errs []error
	)

	setTA := func(pbContent *pbDataFormat.Content, ctx *pbContext.Thread) patillator.SegregateFinder {
		return patillator.ThreadActivity{
			Thread:           ctx,
			ActivityMetadata: patillator.ActivityMetadata(pbContent.Metadata),
		}
	}
	setCA := func(pbContent *pbDataFormat.Content, ctx *pbContext.Comment) patillator.SegregateFinder {
		return patillator.CommentActivity{
			Comment:          ctx,
			ActivityMetadata: patillator.ActivityMetadata(pbContent.Metadata),
		}
	}
	setSCA := func(pbContent *pbDataFormat.Content, ctx *pbContext.Subcomment) patillator.SegregateFinder {
		return patillator.SubcommentActivity{
			Subcomment:       ctx,
			ActivityMetadata: patillator.ActivityMetadata(pbContent.Metadata),
		}
	}

	err := h.users.View(func(tx *bolt.Tx) error {
		for _, user := range users {
			wg.Add(1)
			go func(user string) {
				defer wg.Done()
				
				pbUser, err := h.User(user)
				if err != nil {
					log.Println(err)
					return
				}
				// check whether the user has recent activity
				recent := pbUser.RecentActivity
				if recent == nil {
					// no recent activity; do nothing and finish this go-routine
					return
				}
				// set threads from the recent activity of this user
				for _, ctx := range recent.ThreadsCreated {
					wg.Add(1)
					go func(ctx *pbContext.Thread, user string) {
						defer wg.Done()
						pbContent, err := h.GetThreadContent(ctx)
						m.Lock()
						defer m.Unlock()
						if err != nil {
							// failed to get thread metadata; append err to errs
							// and return
							errs = append(errs, err)
							return
						}
						once.Do(func() {
							activity = make(map[string]patillator.UserActivity)
						})
						ta := setTA(pbContent, ctx)
						activity[user].ThreadsCreated = append(activity[user].ThreadsCreated, ta)
					}(ctx, user)
				}
				// set comments from the recent activity of this user
				for _, ctx := range recent.Comments {
					wg.Add(1)
					go func(ctx *pbContext.Comment, user string) {
						defer wg.Done()
						pbContent, err := h.GetCommentContent(ctx)
						m.Lock()
						defer m.Unlock()
						if err != nil {
							// failed to get comment metadata; append err to errs
							// and return
							errs = append(errs, err)
							return
						}
						once.Do(func() {
							activity = make(map[string]patillator.UserActivity)
						})
						ca := setCA(pbContent, ctx)
						activity[user].Comments = append(activity[user].Comments, ca)
					}(ctx, user)
				}
				// set subcomments from the recent activity of this user
				for _, ctx := range recent.ThreadsCreated {
					wg.Add(1)
					go func(ctx *pbContext.Subcomment, user string) {
						defer wg.Done()
						pbContent, err := h.GetSubcommentContent(ctx)
						m.Lock()
						defer m.Unlock()
						if err != nil {
							// failed to get subcomment metadata; append err to
							// errs and return
							errs = append(errs, err)
							return
						}
						once.Do(func() {
							activity = make(map[string]patillator.UserActivity)
						})
						sca := setSCA(pbContent, ctx)
						activity[user].Subcomments = append(activity[user].Subcomments, sca)
					}(ctx, user)
				}
			}(user)
		}
		wg.Wait()
		return nil
	})
	if err != nil {
		errs = append(errs, err)
	}
	return activity, errs
}

// GetContentsByContext gets and formats the contents with the given contexts
// into a []*pbApi.ContentRule.
func (h *handler) GetContentsByContext(contexts []*pbContext.Context) ([]*pbApi.ContentRule, []error) {
	var (
		contentRules = make([]*pbApi.ContentRule, len(contexts))
		errs []error
		m sync.Mutex
		wg sync.WaitGroup
	)

	for idx, context := range contexts {
		wg.Add(1)
		go func(idx int, context *pbContext.Context) {
			defer wg.Done()
			var (
				contentRule *pbApi.ContentRule
				err error
			)
			switch ctx := context.(type) {
			case *pbContext.Thread:
				contentRule, err = h.GetThread(ctx)
				if err != nil {
					contentRule = &pbApi.ContentRule{}
					log.Printf("Could not get thread: %v\n", err)
					m.Lock()
					errs = append(errs, err)
					m.Unlock()
				}
			case *pbContext.Comment:
				contentRule, err = h.GetComment(ctx)
				if err != nil {
					contentRule = &pbApi.ContentRule{}
					log.Printf("Could not get comment: %v\n", err)
					m.Lock()
					errs = append(errs, err)
					m.Unlock()
				}
			case *pbContext.Subcomment:
				contentRule, err = h.GetSubcomment(ctx)
				if err != nil {
					contentRule = &pbApi.ContentRule{}
					log.Printf("Could not get subcomment: %v\n", err)
					m.Lock()
					errs = append(errs, err)
					m.Unlock()
				}
			}
			contentRules[idx] = contentRule
		}(idx, context)
	}
	wg.Wait()
	return contentRules, errs
}
