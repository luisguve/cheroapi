package patillator

import (
	"time"

	pbContext "github.com/luisguve/cheroproto-go/context"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
	pbMetadata "github.com/luisguve/cheroproto-go/metadata"
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
	return (metadata.Interactions > 10) && (metadata.AvgUpdateTime <= min.Seconds())
}

// IsLessRelevantThan compares the field Interactions and the field AvgUpdateTime
// of am and the underlying ActivityMetadata of the argument.
//
// It returns true if gc -the local- has less interactions AND an equal or
// greater average update time difference than other -the argument-, or false
// if either some of the conditions are false or the underlying type of other
// is not an ActivityMetadata.
func (am ActivityMetadata) IsLessRelevantThan(other interface{}) bool {
	var otherAM ActivityMetadata
	// type switch to get the underlying ActivityMetadata
	switch act := other.(type) {
	case ThreadActivity:
		otherAM = act.ActivityMetadata
	case CommentActivity:
		otherAM = act.ActivityMetadata
	case SubcommentActivity:
		otherAM = act.ActivityMetadata
	default:
		return false
	}
	return (am.Interactions < otherAM.Interactions) &&
		(am.AvgUpdateTime >= otherAM.AvgUpdateTime)
}

// ThreadActivity holds the metadata of a thread as well as its context.
type ThreadActivity struct {
	SectionId string
	Thread    *pbContext.Thread
	ActivityMetadata
}

// Key returns a Context containing a thread context.
func (ta ThreadActivity) Key() interface{} {
	return &pbContext.Context{
		Ctx: &pbContext.Context_ThreadCtx{
			ThreadCtx: ta.Thread,
		},
		SectionId: ta.SectionId,
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
	SectionId string
	Comment   *pbContext.Comment
	ActivityMetadata
}

// Key returns a Context containing a comment context.
func (ca CommentActivity) Key() interface{} {
	return &pbContext.Context{
		Ctx: &pbContext.Context_CommentCtx{
			CommentCtx: ca.Comment,
		},
		SectionId: ca.SectionId,
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
	SectionId  string
	Subcomment *pbContext.Subcomment
	ActivityMetadata
}

// Key returns a Context containing a subcomment context.
func (sca SubcommentActivity) Key() interface{} {
	return &pbContext.Context{
		Ctx: &pbContext.Context_SubcommentCtx{
			SubcommentCtx: sca.Subcomment,
		},
		SectionId: sca.SectionId,
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

// OrderActivityBySection returns the given activity sorted by section id.
func OrderActivityBySection(activity *pbDataFormat.Activity) map[string]*pbDataFormat.Activity {
	if activity == nil {
		return nil
	}

	var result map[string]*pbDataFormat.Activity

	for _, t := range activity.ThreadsCreated {
		section := t.SectionCtx.Id
		if result == nil {
			result = make(map[string]*pbDataFormat.Activity)
		}
		sectionActivity := result[section]
		if sectionActivity == nil {
			sectionActivity = &pbDataFormat.Activity{}
		}
		sectionActivity.ThreadsCreated = append(sectionActivity.ThreadsCreated, t)
		result[section] = sectionActivity
	}
	for _, c := range activity.Comments {
		section := c.ThreadCtx.SectionCtx.Id
		if result == nil {
			result = make(map[string]*pbDataFormat.Activity)
		}
		sectionActivity := result[section]
		if sectionActivity == nil {
			sectionActivity = &pbDataFormat.Activity{}
		}
		sectionActivity.Comments = append(sectionActivity.Comments, c)
		result[section] = sectionActivity
	}
	for _, sc := range activity.Subcomments {
		section := sc.CommentCtx.ThreadCtx.SectionCtx.Id
		if result == nil {
			result = make(map[string]*pbDataFormat.Activity)
		}
		sectionActivity := result[section]
		if sectionActivity == nil {
			sectionActivity = &pbDataFormat.Activity{}
		}
		sectionActivity.Subcomments = append(sectionActivity.Subcomments, sc)
		result[section] = sectionActivity
	}
	return result
}

type UserActivity struct {
	ThreadsCreated []SegregateFinder
	Comments       []SegregateFinder
	Subcomments    []SegregateFinder
}
