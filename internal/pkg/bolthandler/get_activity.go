package bolthandler

import(

)

// Get activity of users
func (h *handler) GetActivity(users ...string,
	setTA func(pbContent *pbDataFormat.Content, ctx *pbContext.Thread) patillator.SegregateFinder,
	setCA func(pbContent *pbDataFormat.Content, ctx *pbContext.Comment) patillator.SegregateFinder,
	setSCA func(pbContent *pbDataFormat.Content, ctx *pbContext.Subcomment) patillator.SegregateFinder)
		(map[string]patillator.UserActivity, []error) {
	var (
		activity map[string]patillator.UserActivity
		m sync.Mutex
		once sync.Once
		wg sync.WaitGroup
		errs []error
	)
	
	err := h.users.View(func(tx *bolt.Tx) error {
		usersBucket := tx.Bucket([]byte(usersB))
		if usersBucket == nil {
			return fmt.Errorf("Bucket %s of users not found\n", usersB)
		}

		for _, user := range users {
			wg.Add(1)
			go func(user string) {
				defer wg.Done()

				userBytes := usersBucket.Get([]byte(user))
				if userBytes == nil {
					log.Printf("Could not find user (id %s)\n", user)
					return
				}

				pbUser := new(pbDataFormat.User)

				if err := proto.Unmarshal(userBytes, pbUser); err != nil {
					log.Printf("Could not unmarshal user: %v\n", err)
					m.Lock()
					defer m.Unlock()
					errs = append(errs, err)
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
		contentRules []*pbApi.ContentRule
		errs []error
		m sync.Mutex
		wg sync.WaitGroup
	)

	for _, context := range contexts {
		wg.Add(1)
		go func(context *pbContext.Context) {
			defer wg.Done()

			switch ctx := context.(type) {
			case *pbContext.Thread:
				contentRule, err := h.GetThread(ctx)

				m.Lock()
				defer m.Unlock()
				if err != nil {
					log.Printf("Could not get thread: %v\n", err)
					errs = append(errs, err)
					return
				}
				contentRules = append(contentRules, contentRule)
			case *pbContext.Comment:
				contentRule, err := h.GetComment(ctx)

				m.Lock()
				defer m.Unlock()
				if err != nil {
					log.Printf("Could not get comment: %v\n", err)
					errs = append(errs, err)
					return
				}
				contentRules = append(contentRules, contentRule)
			case *pbContext.Subcomment:
				contentRule, err := h.GetSubcomment(ctx)

				m.Lock()
				defer m.Unlock()
				if err != nil {
					log.Printf("Could not get subcomment: %v\n", err)
					errs = append(errs, err)
					return
				}
				contentRules = append(contentRules, contentRule)
			}
		}(context)
	}
	wg.Wait()
	return contentRules, errs
}
