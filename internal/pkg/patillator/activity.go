package patillator

import(
	pbMetadata ""
)

type ActivityMetadata *pbMetadata.Content

// IsRelevant returns true if the number of interactions is greater than 10 and
// the average update time difference is less than 10 minutes. It does the
// comparison on the Interactions and AvgUpdateTime fields, respectively.
func (am ActivityMetadata) IsRelevant() bool {
	// type cast for ease of use
	metadata := *pbMetadata.Content(am)
	return (metadata.Interactions > 10) && (metadata.AvgUpdateTime <= 10 * time.Minute)
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
	if otherAM, ok := other.(ActivityMetadata); !ok {
		return false
	}
	// type cast for ease of use
	metadata := *pbMetadata.Content(am)
	otherMetadata := *pbMetadata.Content(otherAM)
	return (metadata.Interactions < otherMetadata.Interactions) &&
		(metadata.AvgUpdateTime >= otherMetadata.AvgUpdateTime)
}

type ThreadActivity struct {
	Thread *pbContext.Thread
	ActivityMetadata
}

func (ta ThreadActivity) DataKey() interface{} {
	return &pbContext.Context{
		Ctx: &pbContext.Context_ThreadCtx{
			ThreadCtx: *pbContext.Thread(ta.Thread),
		},
	}
}

func (ta ThreadActivity) toDiscard(ids []*pbContext.Thread) (bool, []*pbContext.Thread) {
	toDiscard := false
	t := ta.Thread
	for i := 0; i < len(ids); i++ {
		// compare sections
		section1 := t.SectionCtx.Id
		section2 := ids[i].SectionCtx.Id
		if section1 == section2 {
			id1 := t.Id
			id2 := ids[i].Id
			// compare ids
			if id1 == id2 {
				// remove id
				last := len(ids) - 1
				ids[i] = ids[last]
				ids = ids[:last]
				toDiscard = true
				break
			}
		}
	}
	return toDiscard, ids
}

type CommentActivity struct {
	Comment *pbContext.Comment
	ActivityMetadata
}

func (ca CommentActivity) DataKey() interface{} {
	return &pbContext.Context{
		Ctx: &pbContext.Context_CommentCtx{
			CommentCtx: *pbContext.Comment(ca.Comment),
		}
	}
}

func (ca CommentActivity) toDiscard(ids []*pbContext.Comment) (bool, []*pbContext.Comment) {
	c := ca.Comment

	id1 := c.Id
	threadCtx1 := c.ThreadCtx
	sectionCtx1 := c.ThreadCtx.SectionCtx

	toDiscard := false

	// Iterate over ids. Compare sections then threads then ids.
	for i := 0; i < len(ids); i++ {
		sectionCtx2 := ids[i].ThreadCtx.SectionCtx

		// compare sections
		section1 := sectionCtx1.Id
		section2 := sectionCtx2.Id
		if section1 == section2 {
			threadCtx2 := ids[i].ThreadCtx

			// compare threads
			thread1 := threadCtx1.Id
			thread2 := threadCtx2.Id
			if thread1 == thread2 {
				// compare ids
				id2 := ids[i].Id
				if id1 == id2 {
					// remove id
					last := len(ids) - 1
					ids[i] = ids[last]
					ids = ids[:last]
					toDiscard = true
					break					
				}
			}
		}
	}
	return toDiscard, ids
}

type SubcommentActivity struct {
	Subcomment *pbContext.Subcomment
	ActivityMetadata
}

func (sca SubcommentActivity) DataKey() interface{} {
	return &pbContext.Context{
		Ctx: &pbContext.Context_SubcommentCtx{
			SubcommentCtx: *pbContext.Subcomment(sca.Subcomment)
		}
	}
}

func (sca SubcommentActivity) toDiscard(ids []*pbContext.Subcomment) (bool, []*pbContext.Subcomment) {
	sc := sca.Subcomment

	id1 := sc.Id
	commentCtx1 := sc.CommentCtx
	threadCtx1 := sc.CommentCtx.ThreadCtx
	sectionCtx1 := sc.CommentCtx.ThreadCtx.SectionCtx

	toDiscard := false
	
	// Iterate over ids. Compare sections then threads then comments then ids.
	for i := 0; i < len(ids); i++ {
		sectionCtx2 := ids[i].CommentCtx.ThreadCtx.SectionCtx

		// compare sections
		section1 := sectionCtx1.Id
		section2 := sectionCtx2.Id
		if section1 == section2 {
			threadCtx2 := ids[i].CommentCtx.ThreadCtx

			// compare threads
			thread1 := threadCtx1.Id
			thread2 := threadCtx2.Id
			if thread1 == thread2 {
				commentCtx2 := ids[i].CommentCtx

				//compare comments
				comment1 := commentCtx1.Id
				comment2 := commentCtx2.Id
				if comment1 == comment2 {
					// compare ids
					id2 := ids[i].Id
					if id1 == id2 {
						// remove id
						last := len(ids) - 1
						ids[i] = ids[last]
						ids = ids[:last]
						toDiscard = true
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
