package patillator

import (
	"time"

	pbContext "github.com/luisguve/cheroproto-go/context"
	pbMetadata "github.com/luisguve/cheroproto-go/metadata"
)

// Content holds only the thread Id. The caller should know the section Id it
// belongs to.
type Content pbMetadata.Content

// ToDiscard iterates over the received list of ids, compares the ids to the
// field DataKey of the underlying *pbMetadata.Content and returns true if it
// finds a coincidence and the list without the found id, or false and the
// unchanged list.
//
// The caller should be aware that removing the element found means that it
// was replaced by the last element and the slice was re-sliced leaving out the
// last element.
func (c Content) ToDiscard(ids []string) (bool, []string) {
	// type cast for ease of use
	metadata := pbMetadata.Content(c)
	discard := false

	for i := 0; i < len(ids); i++ {
		if metadata.DataKey == ids[i] {
			// this content and the corresponding id are to be discarded
			discard = true
			// Remove the id by copying the last id in the position i and
			// then reslicing the ids, leaving the last element out.
			last := len(ids) - 1
			ids[i] = ids[last]
			ids = ids[:last]
			break
		}
	}
	return discard, ids
}

// IsRelevant returns true if the number of interactions is greater than 10 and
// the average update time difference is less than 10 minutes. It does the
// comparison on the Interactions and AvgUpdateTime fields, respectively.
func (c Content) IsRelevant() bool {
	// type cast for ease of use
	metadata := pbMetadata.Content(c)
	min := 10 * time.Minute
	return (metadata.Interactions > 10) && (metadata.AvgUpdateTime <= min.Seconds())
}

// IsLessRelevantThan compares the field Interactions and the field AvgUpdateTime
// of c and the underlying Content of the argument.
//
// It returns true if c -the local- has less interactions AND an equal or
// greater average update time difference than other -the argument-, or false if
// either some of the conditions are false or the underlying type of other is not
// a Content.
func (c Content) IsLessRelevantThan(other interface{}) bool {
	// type assert to get access to fields Interactions and AvgUpdateTime
	otherC, ok := other.(Content)
	if !ok {
		return false
	}
	// type cast for ease of use
	metadata := pbMetadata.Content(c)
	otherMetadata := pbMetadata.Content(otherC)
	return (metadata.Interactions < otherMetadata.Interactions) &&
		(metadata.AvgUpdateTime >= otherMetadata.AvgUpdateTime)
}

// Key returns a string containing the field DataKey of c, which represents
// a thread Id. The caller should know the section it belongs to.
func (c Content) Key() interface{} {
	// type cast for ease of use
	metadata := pbMetadata.Content(c)
	return metadata.DataKey
}

// GeneralContent holds the thread Id as well as the section Id it belongs to.
type GeneralContent pbMetadata.GeneralContent

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
	metadata := pbMetadata.GeneralContent(gc)
	discard := false

	for i := 0; i < len(ids); i++ {
		if metadata.Content.DataKey == ids[i] {
			// this content and the corresponding id are to be discarded
			discard = true
			// Remove the id by copying the last id in the position i and
			// then reslicing the ids, leaving the last element out.
			last := len(ids) - 1
			ids[i] = ids[last]
			ids = ids[:last]
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
	metadata := pbMetadata.GeneralContent(gc)
	min := 10 * time.Minute
	return (metadata.Content.Interactions > 10) &&
		(metadata.Content.AvgUpdateTime <= min.Seconds())
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
	otherGC, ok := other.(GeneralContent)
	if !ok {
		return false
	}
	// type cast for ease of use
	metadata := pbMetadata.GeneralContent(gc)
	otherMetadata := pbMetadata.GeneralContent(otherGC)
	return (metadata.Content.Interactions < otherMetadata.Content.Interactions) &&
		(metadata.Content.AvgUpdateTime >= otherMetadata.Content.AvgUpdateTime)
}

// Key returns a GeneralId, which holds a thread id and the section it belongs
// to.
func (gc GeneralContent) Key() interface{} {
	// type cast for ease of use
	metadata := pbMetadata.GeneralContent(gc)
	return GeneralId{
		Id:        metadata.Content.DataKey,
		SectionId: metadata.SectionId,
	}
}

// Id holds information to get a thread from the database: its Id. The caller
// should know the section it belongs to.
type Id struct {
	Id     string
	Status string
}

// GeneralId holds information to get a thread from the database: its id and
// the section it belongs to.
type GeneralId struct {
	Id        string
	SectionId string
	Status    string
}

// OrderBySection returns thread ids grouped by section Id.
func OrderBySection(threads []*pbContext.Thread) map[string][]string {
	result := make(map[string][]string)
	for _, t := range threads {
		id := t.SectionCtx.Id
		section := result[id]
		section = append(section, t.Id)
		result[id] = section
	}
	return result
}
