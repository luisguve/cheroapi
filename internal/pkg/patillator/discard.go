package patillator

import(
	"log"
	
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
)

// DiscardActivities removes the threads, comments and subcomments of the activity
// set whose toDiscard method returns true and returns the resulting activity.
// It does nothing when the fields ThreadsCreated, Comments or Subcomments from
// either the given Activity or ids are empty.
func DiscardActivities(a UserActivity, ids *pbDataFormat.Activity) UserActivity {
	// initially, no activity has been discarded and if there are no ids
	// to compare, the exact same list of activity will be returned back.
	discardedActivity := a

	// check whether there is not anything to discard
	if ids == nil {
		// return the same Activity
		return a
	}

	var wg sync.WaitGroup

	// workflow: remove threads then comments then subcomments

	// discard threads only if there are threads to discard
	if (len(a.ThreadsCreated) > 0) && (len(ids.ThreadsCreated) > 0) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			total := len(a.ThreadsCreated)
			removed := 0
			for idx := 0; idx < len(a.ThreadsCreated); idx++ {
				// Discard the thread at position idx and the threads that
				// replace it if it fulfills the requirement toDiscard.
				for {
					ta, ok := a.ThreadsCreated[idx].(ThreadActivity)
					if !ok {
						log.Printf("Failed type assertion to ThreadActivity.\n")
						break
					}
					var discard bool
					discard, ids.ThreadsCreated = ta.toDiscard(ids.ThreadsCreated)
					if discard {
						removed++
						// copy last valid element in position idx
						a.ThreadsCreated[idx] = a.ThreadsCreated[total - removed]
						// re-slice a.ThreadsCreated, leaving out the last element
						a.ThreadsCreated = a.ThreadsCreated[:total - removed]
						// If there are still elements in a.ThreadsCreated,
						// keep checking the element at position idx, which was
						// replaced by the last thread in a.ThreadsCreated.
						if len(a.ThreadsCreated) > 0 {
							continue
						}
					}
					break
				}
				if len(ids.ThreadsCreated) == 0 {
					// no more thread ids to compare; break loop
					break
				}
			}
			// free memory used by removed elements by allocating a new slice
			// and copying the resulting elements.
			discardedActivity.ThreadsCreated = make([]SegregateFinder, total - removed)
			copy(discardedActivity.ThreadsCreated, a.ThreadsCreated)
		}()
	}
	// discard comments only if there are comments to discard
	if (len(a.Comments) > 0) && (len(ids.Comments) > 0) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			total := len(a.Comments)
			removed := 0
			for idx := 0; idx < len(a.Comments); idx++ {
				// Discard the comment at position idx and the comments that
				// replace it if it fulfills the requirement toDiscard.
				for {
					ca, ok := a.Comments[idx].(CommentActivity)
					if !ok {
						log.Printf("Failed type assertion to CommentActivity.\n")
						break
					}
					var discard bool
					discard, ids.Comments = ca.toDiscard(ids.Comments)
					if discard {
						removed++
						// copy last element in position idx
						a.Comments[idx] = a.Comments[total - removed]
						// re-slice a.Comments, leaving out the last element
						a.Comments = a.Comments[:total - removed]
						// If there are still elements in a.Comments, keep
						// checking the element at position idx, which was
						// replaced by the last comment in a.Comments.
						if len(a.Comments) > 0 {
							continue
						}
					}
					break
				}
				if len(ids.Comments) == 0 {
					// no more comment ids to compare; break loop
					break
				}
			}
			// free memory used by removed elements by allocating a new slice
			// and copying the resulting elements
			discardedActivity.Comments = make([]SegregateFinder, total - removed)
			copy(discardedActivity.Comments, a.Comments)
		}()
	}
	// discard subcomments only if there are subcomments to discard
	if (len(a.Subcomments) > 0) && (len(ids.Subcomments) > 0) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			total := len(a.Subcomments)
			removed := 0
			for idx := 0; idx < len(a.Subcomments); idx++ {
				// Discard the subcomment at position idx and the subcomments
				// that replace it if it fulfills the requirement toDiscard.
				for {
					sca, ok := a.Subcomments[idx].(SubcommentActivity)
					if !ok {
						log.Printf("Failed type assertion to SubcommentActivity\n")
						break
					}
					var discard bool
					discard, ids.Subcomments = sca.toDiscard(ids.Subcomments)
					if discard {
						removed++
						// copy last element in position idx
						a.Subcomments[idx] = a.Subcomments[total - removed]
						// re-slice a.Subcomments, leaving out the last element
						a.Subcomments = a.Subcomments[:total - removed]
						// If there are still elements in a.Subcomments, keep
						// checking the element at position idx, which was
						// replaced by the last comment in a.Subcomments.
						if len(a.Subcomments) > 0 {
							continue
						}
					}
					break
				}
				if len(ids.Subcomments) == 0 {
					// no more subcomment ids to compare; break loop
					break
				}
			}
			// free memory used by removed elements by allocating a new slice
			// and copying the resulting elements
			discardedActivity.Subcomments = make([]SegregateFinder, total - removed)
			copy(discardedActivity.Subcomments, a.Subcomments)
		}()
	}
	wg.Wait()
	return discardedActivity
}

// DiscardContents removes the contents whose ToDiscard method returns true and
// returns the resulting contents. It does nothing when either contents, ids or
// both are empty.
func DiscardContents(contents []SegregateDiscarderFinder, ids []string) []SegregateFinder {
	// initially, no contents have been discarded and if there are no ids
	// to compare, the exact same list of contents will be returned back.

	if (len(ids) > 0) && (len(contents) > 0) {
		total := len(contents)
		removed := 0
		for idx := 0; idx < len(contents); idx++ {
			// Discard the content at position idx and the contents that
			// replace it if it fulfills the requirement ToDiscard.
			for {
				var discard bool
				discard, ids = contents[idx].ToDiscard(ids)
				if discard {
					removed++
					// copy last valid element in position idx
					contents[idx] = contents[total - removed]
					// re-slice contents, leaving out the last element
					contents = contents[:total - removed]
					// If there are still elements in contents, keep checking
					// the element at position idx, which was replaced by the
					// last content.
					if len(contents) > 0 {
						continue
					}
				}
				break
			}
			if len(ids) == 0 {
				// no more ids to compare; break loop
				break
			}
		}
	}
	result := make([]SegregateFinder, len(contents))
	for i, c := range contents {
		result[i] = SegregateFinder(c)
	}
	return result
}
