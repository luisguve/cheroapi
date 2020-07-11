package bolt

import (
	"log"
	"sync"

	"github.com/luisguve/cheroapi/internal/pkg/patillator"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	pbContext "github.com/luisguve/cheroproto-go/context"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
	bolt "go.etcd.io/bbolt"
)

// Get activity of users
func (h *handler) GetActivity(users ...string) (map[string]patillator.UserActivity, []error) {
	var (
		activity map[string]patillator.UserActivity
		m        sync.Mutex
		once     sync.Once
		wg       sync.WaitGroup
		errs     []error
	)

	setTA := func(pbContent *pbDataFormat.Content, ctx *pbContext.Thread) patillator.SegregateFinder {
		return patillator.ThreadActivity{
			Thread:           ctx,
			ActivityMetadata: patillator.ActivityMetadata(*pbContent.Metadata),
		}
	}
	setCA := func(pbContent *pbDataFormat.Content, ctx *pbContext.Comment) patillator.SegregateFinder {
		return patillator.CommentActivity{
			Comment:          ctx,
			ActivityMetadata: patillator.ActivityMetadata(*pbContent.Metadata),
		}
	}
	setSCA := func(pbContent *pbDataFormat.Content, ctx *pbContext.Subcomment) patillator.SegregateFinder {
		return patillator.SubcommentActivity{
			Subcomment:       ctx,
			ActivityMetadata: patillator.ActivityMetadata(*pbContent.Metadata),
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
						userAct := activity[user]
						userAct.ThreadsCreated = append(userAct.ThreadsCreated, ta)
						activity[user] = userAct
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
						userAct := activity[user]
						userAct.Comments = append(userAct.Comments, ca)
						activity[user] = userAct
					}(ctx, user)
				}
				// set subcomments from the recent activity of this user
				for _, ctx := range recent.Subcomments {
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
						userAct := activity[user]
						userAct.Subcomments = append(userAct.Subcomments, sca)
						activity[user] = userAct
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
		errs         []error
		m            sync.Mutex
		wg           sync.WaitGroup
	)

	for idx, context := range contexts {
		wg.Add(1)
		go func(idx int, context *pbContext.Context) {
			defer wg.Done()
			var (
				contentRule *pbApi.ContentRule
				err         error
			)
			switch ctx := context.Ctx.(type) {
			case *pbContext.Context_ThreadCtx:
				contentRule, err = h.GetThread(ctx.ThreadCtx)
				if err != nil {
					contentRule = &pbApi.ContentRule{}
					log.Printf("Could not get thread: %v\n", err)
					m.Lock()
					errs = append(errs, err)
					m.Unlock()
				}
			case *pbContext.Context_CommentCtx:
				contentRule, err = h.GetComment(ctx.CommentCtx)
				if err != nil {
					contentRule = &pbApi.ContentRule{}
					log.Printf("Could not get comment: %v\n", err)
					m.Lock()
					errs = append(errs, err)
					m.Unlock()
				}
			case *pbContext.Context_SubcommentCtx:
				contentRule, err = h.GetSubcomment(ctx.SubcommentCtx)
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
