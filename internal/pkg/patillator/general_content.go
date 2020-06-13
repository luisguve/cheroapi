package patillator

import(
	
)

type GeneralContent *pbMetadata.GeneralContent

// ToDiscard iterates over the received list of ids, compares the ids to the
// field DataKey of the Content of the underlying *pbMetadata.GeneralContent
// and returns true if it finds a coincidence and the list without the found
// id, or false and the unchanged list.
//
// The caller should be aware that removing the element found means that it was
// replaced by the last element and the slice was re-sliced leaving out the
// last element.
func (gc GeneralContent) ToDiscard(ids []string) (bool, []string) {
	// type cast for ease of use
	metadata := *pbMetadata.GeneralContent(gc)
	discard := false

	for i := 0; i < len(ids); i++ {
		if metadata.Content.DataKey == ids[i] {
			// this content and the corresponding id are to be discarded
			discard = true
			// Remove the id by copying the last id in the position i and
			// then reslicing the ids, leaving the last element out.
			last := len(ids) - 1
			ids[i] = ids[last]
			ids = ids[: last]
			break
		}
	}
	return discard, ids
}

// IsRelevant returns true if the number of interactions is greater than 10 and
// the average update time difference is less than 10 minutes. It does the
// comparison on the Interactions and AvgUpdateTime fields, respectively.
func (gc GeneralContent) IsRelevant() bool {
	// type cast for ease of use
	metadata := *pbMetadata.GeneralContent(gc)
	return (metadata.Content.Interactions > 10) && 
		(metadata.Content.AvgUpdateTime <= 10 * time.Minute)
}

// IsLessRelevantThan compares the field Interactions and the field AvgUpdateTime
// of gc and the underlying GeneralContent of the argument.
// 
// It returns true if gc -the local- has less interactions AND an equal or
// greater average update time difference than other -the argument-, or false if
// either some of the conditions are false or the underlying type of other is not
// a GeneralContent.
func (gc GeneralContent) IsLessRelevantThan(other interface{}) bool {
	// type assert to get access to fields Contents.Interactions and Contents.AvgUpdateTime
	if otherGC, ok := other.(GeneralContent); !ok {
		return false
	}
	// type cast for ease of use
	metadata := *pbMetadata.GeneralContent(gc)
	otherMetadata := *pbMetadata.GeneralContent(otherGC)
	return (metadata.Content.Interactions < otherMetadata.Content.Interactions) &&
		(metadata.Content.AvgUpdateTime >= otherMetadata.Content.AvgUpdateTime)
}

// DataKey returns a GeneralId, which holds a thread id and the section it
// belongs to.
func (gc GeneralContent) DataKey() interface{} {
	// type cast for ease of use
	metadata := *pbMetadata.GeneralContent(gc)
	return GeneralId{
		Id:        metadata.Content.DataKey,
		SectionId: metadata.SectionId,
	}
}

// GeneralId holds information to get a thread from the database: its id and
// the section it belongs to.
type GeneralId struct {
	Id string
	SectionId string
}
