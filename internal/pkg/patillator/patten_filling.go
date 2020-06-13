package patillator

import(
	
)

// FillActivityPattern merges out the fields ThreadsCreated, Comments and
// Subcomments (type []SegregateFinder) from the given map[string]UserActivity
// into a single []SegregateFinder, and then fetches out SegregateFinder
// instances in a random fashion, with a probability of 80% of following
// the given pattern, and returns a []*pbContext.Context containing the required
// information to retrieve the contents from the databases.
// 
// It may return a smaller list of content contexts than the provided pattern
// requires, depending upon the availability of contents.
func FillActivityPattern(activity map[string]UserActivity, pattern []pbMetadata.ContentStatus) []*pbContext.Context {
	var activities []SegregateFinder
	for _, a := range activity {
		activities = append(activities, a.ThreadsCreated...)
		activities = append(activities, a.Comments...)
		activities = append(activities, a.Subcomments...)
	}
	segActivities := segregate(activities)

	var result []*pbContext.Context

	var content ContentFinder
	// empty is a flag that indicates whether both newContents and relContents
	// have no more contents to fetch from.
	var empty bool
	FOR:
		for _, status := range pattern {
			switch pbMetadata.ContentStatus_name[status] {
			case "NEW":
				content, segActivities.newContents, segActivities.relContents, empty = fetch(segActivities.newContents,
					segActivities.relContents)
				if !empty {
					var ctx *pbContext.Context
					// check type assertion to ensure there will not be a panic
					if ctx, ok :=  content.DataKey().(*pbContext.Context); ok {
						result = append(result, ctx)
					}
					continue
				}
				fallthrough
			case "REL":
				content, segActivities.relContents, segActivities.newContents, empty = fetch(segActivities.relContents,
					segActivities.newContents)
				if !empty {
					var ctx *pbContext.Context
					// check type assertion to ensure there will not be a panic
					if ctx, ok :=  content.DataKey().(*pbContext.Context); ok {
						result = append(result, ctx)
					}
					continue
				}
				fallthrough
			case "TOP":
				if segActivities.topContent != nil {
					var ctx *pbContext.Context
					// check type assertion to ensure there will not be a panic
					if ctx, ok :=  segActivities.topContent.DataKey().(*pbContext.Context); ok {
						result = append(result, ctx)
					}
					// set topContent to nil to avoid reaching this point again.
					segActivities.topContent = nil
				}
				if empty {
					break FOR
				}
			}
		}
	return result
}

// FillGeneralPattern merges out every []SegregateFinder in generalContents into
// a single []SegregateFinder, then fetches out contents from it in a random
// fashion, with a probability of 80% of following the given pattern, and returns
// a []GeneralId containing the ids of the contents along with the section they
// they belong to, to be retrieved from the database.
//
// It may return a smaller list of content contexts than the provided pattern
// requires, depending upon the availability of contents.
func FillGeneralPattern(generalContents map[string][]SegregateFinder, pattern []pbMetadata.ContentStatus) []GeneralId {
	var contents []SegregateFinder
	for _, c := range generalContents {
		contents = append(contents, c...)
	}
	// segregated contents
	segContents := segregate(contents)

	var result []GeneralId

	var content ContentFinder
	// empty is a flag that indicates whether both newContents and relContents
	// have no more contents to fetch from.
	var empty bool
	FOR:
		for _, status := range pattern {
			switch pbMetadata.ContentStatus_name[status] {
			case "NEW":
				content, segContents.newContents, segContents.relContents, empty = fetch(segContents.newContents,
					segContents.relContents)
				if !empty {
					var id GeneralId
					// check type assertion to ensure there will not be a panic.
					if id, ok := content.DataKey().(GeneralId); ok {
						result = append(result, id)
					}
					continue
				}
				fallthrough
			case "REL":
				content, segContents.relContents, segContents.newContents, empty = fetch(segContents.relContents,
					segContents.newContents)
				if !empty {
					var id GeneralId
					// check type assertion to ensure there will not be a panic.
					if id, ok := content.DataKey().(GeneralId); ok {
						result = append(result, id)
					}
					continue
				}
				fallthrough
			case "TOP":
				if segContents.topContent != nil {
					var id GeneralId
					// check type assertion to ensure there will not be a panic.
					if id, ok := segContents.topContent.DataKey().(GeneralId); ok {
						result = append(result, id)
					}
					// set topContent to nil to avoid reaching this point again.
					segContents.topContent = nil
				}
				if empty {
					break FOR
				}
			}
		}
	return result
}

// FillPattern fetches out contents from the given []SegregateFinder in a
// random fashion, with a probability of 80% of following the given pattern, and
// returns a []string containing the ids of the contents to be retrieved from
// the database. The caller must know the context of the contents being fetched
// out, as only the list of raw content ids will be returned back.
//
// It may return a smaller list of content contexts than the provided pattern
// requires, depending upon the availability of contents.
func FillPattern(contents []SegregateFinder, pattern []pbMetadata.ContentStatus) []string {
	// segregated contents
	segContents := segregate(contents)

	var result []string

	var content ContentFinder
	// empty is a flag that indicates whether both newContents and relContents
	// have no more contents to fetch from.
	var empty bool
	FOR:
		for _, status := range pattern {
			switch pbMetadata.ContentStatus_name[status] {
			case "NEW":
				content, segContents.newContents, segContents.relContents, empty = fetch(segContents.newContents,
					segContents.relContents)
				if !empty {
					var id string
					// check type assertion to ensure there will not be a panic
					if id, ok :=  content.DataKey().(string); ok {
						result = append(result, id)
					}
					continue
				}
				fallthrough
			case "REL":
				content, segContents.relContents, segContents.newContents, empty = fetch(segContents.relContents,
					segContents.newContents)
				if !empty {
					var id string
					// check type assertion to ensure there will not be a panic
					if id, ok :=  content.DataKey().(string); ok {
						result = append(result, id)
					}
					continue
				}
				fallthrough
			case "TOP":
				if segContents.topContent != nil {
					var id string
					// check type assertion to ensure there will not be a panic
					if id, ok :=  segContents.topContent.DataKey().(string); ok {
						result = append(result, id)
					}
					// set topContent to nil to avoid reaching this point again.
					segContents.topContent = nil
				}
				if empty {
					break FOR
				}
			}
		}
	return result
}
