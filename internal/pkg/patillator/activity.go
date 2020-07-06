package patillator

import(
	"time"

	pbMetadata "github.com/luisguve/cheroproto-go/metadata"
	pbContext "github.com/luisguve/cheroproto-go/context"
)

// ActivityMetadata holds the metadata of a content.
type ActivityMetadata pbMetadata.Content

// IsRelevant returns true if the number of interactions is greater than 10 and
// the average update time difference is less than 10 minutes. It does the
// comparison on the Interactions and AvgUpdateTime fields, respectively.
func (am ActivityMetadata) IsRelevant() bool {
	// type cast for ease of use
	metadata := pbMetadata.Content(am)
	min := 10 * time.Minute
	return (metadata.Interactions > 10) && (metadata.AvgUpdateTime <= min.Minutes())
}

// IsLessRelevantThan compares the field Interactions and the field AvgUpdateTime
// of am and the underlying ActivityMetadata of the argument.
// 
// It returns true if gc -the local- has less interactions AND an equal or
// greater average update time difference than other -the argument-, or false
// if either some of the conditions are false or the underlying type of other
// is not an ActivityMetadata.
func (am ActivityMetadata) IsLessRelevantThan(other interface{}) bool {
	// type assert to get the underlying ActivityMetadata
	otherAM, ok := other.(ActivityMetadata)
	if !ok {
		return false
	}
	// type cast for ease of use
	metadata := pbMetadata.Content(am)
	otherMetadata := pbMetadata.Content(otherAM)
	return (metadata.Interactions < otherMetadata.Interactions) &&
		(metadata.AvgUpdateTime >= otherMetadata.AvgUpdateTime)
}

// ThreadActivity holds the metadata of a thread as well as its context.
type ThreadActivity struct {
	Thread *pbContext.Thread
	ActivityMetadata
}

// Key returns a Context containing a thread context.
func (ta ThreadActivity) Key() interface{} {
	return &pbContext.Context{
		Ctx: &pbContext.Context_ThreadCtx{
			ThreadCtx: ta.Thread,
		},
	}
}

// toDiscard compares the section ids, then thread ids of ta and each comment
// context in ids. If it finds a coincidence, it returns true and the slice of
// ids without the comment context found.
func (ta ThreadActivity) toDiscard(ids []*pbContext.Thread) (bool, []*pbContext.Thread) {
	toDiscard := false
	t := ta.Thread

	id1 := t.Id
	sectionCtx1 := t.SectionCtx

	for i, t := range ids {
		// compare sections
		section1 := sectionCtx1.Id
		section2 := t.SectionCtx.Id

		if section1 == section2 {
			// compare ids
			id2 := t.Id

			if id1 == id2 {
				toDiscard = true
				// remove id
				last := len(ids) - 1
				ids[i] = ids[last]
				ids = ids[:last]
				break
			}
		}
	}
	return toDiscard, ids
}

// CommentActivity holds the metadata of a comment as well as its context.
type CommentActivity struct {
	Comment *pbContext.Comment
	ActivityMetadata
}

// Key returns a Context containing a comment context.
func (ca CommentActivity) Key() interface{} {
	return &pbContext.Context{
		Ctx: &pbContext.Context_CommentCtx{
			CommentCtx: ca.Comment,
		},
	}
}

// toDiscard compares the section ids, then thread ids, then comment ids of
// ca and each comment context in ids. If it finds a coincidence, it returns
// true and the slice of ids without the comment context found.
func (ca CommentActivity) toDiscard(ids []*pbContext.Comment) (bool, []*pbContext.Comment) {
	c := ca.Comment

	id1 := c.Id
	threadCtx1 := c.ThreadCtx
	sectionCtx1 := c.ThreadCtx.SectionCtx

	toDiscard := false

	// Iterate over ids. Compare sections then threads then ids.
	for i, c := range ids {
		sectionCtx2 := c.ThreadCtx.SectionCtx
		
		// compare sections
		section1 := sectionCtx1.Id
		section2 := sectionCtx2.Id

		if section1 == section2 {
			threadCtx2 := c.ThreadCtx

			// compare threads
			thread1 := threadCtx1.Id
			thread2 := threadCtx2.Id

			if thread1 == thread2 {
				// compare ids
				id2 := c.Id

				if id1 == id2 {
					toDiscard = true
					// remove id
					last := len(ids) - 1
					ids[i] = ids[last]
					ids = ids[:last]
					break					
				}
			}
		}
	}
	return toDiscard, ids
}

// SubcommentActivity holds the metadata of a subcomment as well as its context.
type SubcommentActivity struct {
	Subcomment *pbContext.Subcomment
	ActivityMetadata
}

// Key returns a Context containing a subcomment context.
func (sca SubcommentActivity) Key() interface{} {
	return &pbContext.Context{
		Ctx: &pbContext.Context_SubcommentCtx{
			SubcommentCtx: sca.Subcomment,
		},
	}
}

// toDiscard compares the section ids, then thread ids, then comment ids, then
// subcomment ids of sca and each subcomment context in ids. If it finds a
// coincidence, it returns true and the slice of ids without the subcomment
// context found.
func (sca SubcommentActivity) toDiscard(ids []*pbContext.Subcomment) (bool, []*pbContext.Subcomment) {
	sc := sca.Subcomment

	id1 := sc.Id
	commentCtx1 := sc.CommentCtx
	threadCtx1 := sc.CommentCtx.ThreadCtx
	sectionCtx1 := sc.CommentCtx.ThreadCtx.SectionCtx

	toDiscard := false
	
	// Iterate over ids. Compare sections then threads then comments then ids.
	for i, sc := range ids {
		sectionCtx2 := sc.CommentCtx.ThreadCtx.SectionCtx

		// compare sections
		section1 := sectionCtx1.Id
		section2 := sectionCtx2.Id

		if section1 == section2 {
			threadCtx2 := sc.CommentCtx.ThreadCtx

			// compare threads
			thread1 := threadCtx1.Id
			thread2 := threadCtx2.Id

			if thread1 == thread2 {
				commentCtx2 := sc.CommentCtx

				//compare comments
				comment1 := commentCtx1.Id
				comment2 := commentCtx2.Id

				if comment1 == comment2 {
					// compare ids
					id2 := sc.Id

					if id1 == id2 {
						toDiscard = true
						// remove id
						last := len(ids) - 1
						ids[i] = ids[last]
						ids = ids[:last]
						break
					}
				}
			}
		}
	}
	return toDiscard, ids
}

type UserActivity struct {
	ThreadsCreated []SegregateFinder
	Comments []SegregateFinder
	Subcomments []SegregateFinder
}
