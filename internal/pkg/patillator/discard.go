package patillator

import (
	"sync"

	pbContext "github.com/luisguve/cheroproto-go/context"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
)

// DiscardActivities removes the threads, comments and subcomments of the activity
// set whose toDiscard method returns true and returns the resulting activity.
// It does nothing when the fields ThreadsCreated, Comments or Subcomments from
// either the given Activity or ids are empty.
func DiscardActivities(a *pbDataFormat.Activity, ids *pbDataFormat.Activity) *pbDataFormat.Activity {
	// Initially, no activity has been discarded and if there are no ids
	// to compare, the exact same list of activity will be returned back.
	discardedActivity := a

	// Check whether there is not anything to discard.
	if (ids == nil) || (a == nil) {
		// Return the same Activity.
		return a
	}

	var wg sync.WaitGroup

	// Workflow: discard threads, comments and subcomments concurrently.

	// Check whether there are threads to discard.
	if (len(a.ThreadsCreated) > 0) && (len(ids.ThreadsCreated) > 0) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			total := len(a.ThreadsCreated)
			removed := 0
			for idx := 0; idx < len(a.ThreadsCreated); idx++ {
				// Discard the thread at position idx and the threads that
				// replace it if it fulfills the requirement to be discarded.
				for {
					var (
						discard     bool
						id1         = a.ThreadsCreated[idx].Id
						sectionCtx1 = a.ThreadsCreated[idx].SectionCtx
					)
					for i, t := range ids.ThreadsCreated {
						// Compare sections.
						section1 := sectionCtx1.Id
						section2 := t.SectionCtx.Id
						if section1 == section2 {
							// Compare ids.
							id2 := t.Id
							if id1 == id2 {
								// Found coincidence.
								discard = true
								// Remove thread reference from ids.
								last := len(ids.ThreadsCreated) - 1
								ids.ThreadsCreated[i] = ids.ThreadsCreated[last]
								ids.ThreadsCreated = ids.ThreadsCreated[:last]
								break
							}
						}
					}
					if discard {
						removed++
						// Copy the last valid thread in position idx.
						a.ThreadsCreated[idx] = a.ThreadsCreated[total-removed]
						// Re-slice a.ThreadsCreated, leaving out the last thread.
						a.ThreadsCreated = a.ThreadsCreated[:total-removed]
						// If there are still elements in a.ThreadsCreated,
						// keep checking the thread at position idx, which was
						// replaced by the last thread in a.ThreadsCreated.
						if idx < len(a.ThreadsCreated) {
							continue
						}
					}
					break
				}
				if len(ids.ThreadsCreated) == 0 {
					// No more thread ids to compare; break loop.
					break
				}
			}
			// Free memory used by removed threads by allocating a new slice
			// and copying the resulting threads.
			cleanThreadsCreated := make([]*pbContext.Thread, total-removed)
			copy(cleanThreadsCreated, a.ThreadsCreated)
			discardedActivity.ThreadsCreated = cleanThreadsCreated
		}()
	}
	// Check whether there are comments to discard.
	if (len(a.Comments) > 0) && (len(ids.Comments) > 0) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			total := len(a.Comments)
			removed := 0
			for idx := 0; idx < len(a.Comments); idx++ {
				// Discard the comment at position idx and the comments that
				// replace it if it fulfills the requirement to be discarded.
				for {
					var (
						id1         = a.Comments[idx].Id
						threadCtx1  = a.Comments[idx].ThreadCtx
						sectionCtx1 = a.Comments[idx].ThreadCtx.SectionCtx
						discard     bool
					)
					for i, c := range ids.Comments {
						// Compare sections.
						sectionCtx2 := c.ThreadCtx.SectionCtx
						section1 := sectionCtx1.Id
						section2 := sectionCtx2.Id
						if section1 == section2 {
							// Compare threads.
							threadCtx2 := c.ThreadCtx
							thread1 := threadCtx1.Id
							thread2 := threadCtx2.Id
							if thread1 == thread2 {
								// Compare ids.
								id2 := c.Id
								if id1 == id2 {
									// Found coincidence.
									discard = true
									// Remove comment reference from ids.
									last := len(ids.Comments) - 1
									ids.Comments[i] = ids.Comments[last]
									ids.Comments = ids.Comments[:last]
									break
								}
							}
						}
					}
					if discard {
						removed++
						// Copy the last comment in position idx.
						a.Comments[idx] = a.Comments[total-removed]
						// Re-slice a.Comments, leaving out the last comment.
						a.Comments = a.Comments[:total-removed]
						// If there are still elements in a.Comments, keep
						// checking the comment at position idx, which was
						// replaced by the last comment in a.Comments.
						if idx < len(a.Comments) {
							continue
						}
					}
					break
				}
				if len(ids.Comments) == 0 {
					// No more comment ids to compare; break loop.
					break
				}
			}
			// Free memory used by removed comments by allocating a new slice
			// and copying the resulting comments.
			cleanComments := make([]*pbContext.Comment, total-removed)
			copy(cleanComments, a.Comments)
			discardedActivity.Comments = cleanComments
		}()
	}
	// Check whether there are subcomments to discard.
	if (len(a.Subcomments) > 0) && (len(ids.Subcomments) > 0) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			total := len(a.Subcomments)
			removed := 0
			for idx := 0; idx < len(a.Subcomments); idx++ {
				// Discard the subcomment at position idx and the subcomments
				// that replace it if it fulfills the requirement to be discarded.
				for {
					var (
						discard bool
						id1 = a.Subcomments[idx].Id
						commentCtx1 = a.Subcomments[idx].CommentCtx
						threadCtx1 = a.Subcomments[idx].CommentCtx.ThreadCtx
						sectionCtx1 = a.Subcomments[idx].CommentCtx.ThreadCtx.SectionCtx
					)
					for i, sc := range ids.Subcomments {
						// Compare sections.
						sectionCtx2 := sc.CommentCtx.ThreadCtx.SectionCtx
						section1 := sectionCtx1.Id
						section2 := sectionCtx2.Id
						if section1 == section2 {
							// Compare threads.
							threadCtx2 := sc.CommentCtx.ThreadCtx
							thread1 := threadCtx1.Id
							thread2 := threadCtx2.Id
							if thread1 == thread2 {
								// Compare comments.
								commentCtx2 := sc.CommentCtx
								comment1 := commentCtx1.Id
								comment2 := commentCtx2.Id
								if comment1 == comment2 {
									// Compare ids.
									id2 := sc.Id
									if id1 == id2 {
										// Found coincidence
										discard = true
										// Remove subcomment reference from ids.
										last := len(ids.Subcomments) - 1
										ids.Subcomments[i] = ids.Subcomments[last]
										ids.Subcomments = ids.Subcomments[:last]
										break
									}
								}
							}
						}
					}
					if discard {
						removed++
						// Copy the last subcomment in position idx.
						a.Subcomments[idx] = a.Subcomments[total-removed]
						// Re-slice a.Subcomments, leaving out the last subcomment.
						a.Subcomments = a.Subcomments[:total-removed]
						// If there are still elements in a.Subcomments, keep
						// checking the subcomment at position idx, which was
						// replaced by the last comment in a.Subcomments.
						if idx < len(a.Subcomments) {
							continue
						}
					}
					break
				}
				if len(ids.Subcomments) == 0 {
					// No more subcomment ids to compare; break loop.
					break
				}
			}
			// Free memory used by removed subcomments by allocating a new slice
			// and copying the resulting subcomments.
			cleanSubcomments := make([]*pbContext.Subcomment, total-removed)
			copy(cleanSubcomments, a.Subcomments)
			discardedActivity.Subcomments = cleanSubcomments
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
					contents[idx] = contents[total-removed]
					// re-slice contents, leaving out the last element
					contents = contents[:total-removed]
					// If there are still elements in contents, keep checking
					// the element at position idx, which was replaced by the
					// last content.
					if idx < len(contents) {
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

// DiscardIds returns contents without those whose id is in the given list of ids.
func DiscardIds(contents []string, ids []string) []string {
	// Initially, no contents have been discarded and if there are no ids
	// to compare, the exact same list of contents will be returned back.
	if (len(ids) == 0) || (len(contents) == 0) {
		return contents
	}

	total := len(contents)
	removed := 0
	for idx := 0; idx < len(contents); idx++ {
		// Discard the content at position idx and the contents that
		// replace it if it fulfills the requirement to be discarded.
		for {
			var (
				threadId = contents[idx]
				discard bool
			)
			for idx, id := range ids {
				if threadId == id {
					// Remove element from list of ids to be discarded.
					discard = true
					last := len(ids) - 1
					ids[idx] = ids[last]
					ids = ids[:last]
					break
				}
			}
			if discard {
				// Remove thread from resulting slice.
				removed++
				last := len(contents) - 1
				// Copy last valid element in position idx.
				contents[idx] = contents[last]
				// Re-slice contents, leaving out the last element.
				contents = contents[:last]
				// If there are still elements in contents, check again the
				// element at position idx, which was replaced by the last
				// content.
				if idx < len(contents) {
					continue
				}
			}
			break
		}
	}
	if removed > 0 {
		remaining := total - removed
		result := make([]string, remaining)
		copy(result, contents)
		contents = result
	}
	return contents
}
