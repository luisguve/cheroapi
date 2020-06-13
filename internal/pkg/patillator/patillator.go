// package patillator defines the set of functionalities, data types and data
// manipulation functions that are the core of the CheroPatilla system.

package patillator

import(
	"log"
	"time"

	pbContext "github.com/luisguve/cheroapi/internal/protogen/context"
	pbMetadata "github.com/luisguve/cheroapi/internal/protogen/metadata"
	pbDataformat "github.com/luisguve/cheroapi/internal/protogen/dataformat"
)

// ContentFinder is the set of methods that provide the required information
// to find the underlying content among the different databases.
type ContentFinder interface {
	// DataKey returns the required information for finding the content among
	// the databases. The caller must know the type of the returned value to
	// perform a type assertion.
	DataKey() interface{}
}

// Segregator is the set of methods to classify the underlying content and
// place it in the right category.
type Segregator interface {
	// IsRelevant returns whether or not the underlying content fulfills the 
	// requirements to be relevant.
	IsRelevant() bool
	// IsLessRelevantThan returns whether or not the underlying content is less
	// relevant than the underlying content of the interface{} being passed as
	// an argument.
	// It's useful for determining the most relevant content of a given list.
	//
	// True: the argument is more relevant.
	// False: the argument is not more relevant.
	IsLessRelevantThan(interface{}) bool
}

// Discarder defines the required method for contents to evaluate whether they
// should be discarded from the list of contents they belong to.
type Discarder interface {
	// ToDiscard returns whether or not the underlying content fulfills the
	// requirement to be discarded, i.e. that the content's id is in the
	// list of ids being passed as an argument, and the list of ids without
	// the id on which a coincidence was found.
	ToDiscard(ids []string) (bool, []string)
}

// SegregateFinder defines the set of methods for contents to be properly
// classified and to be found among the different databases. See Segregator and
// ContentFinder.
type SegregateFinder interface {
	Segregator
	ContentFinder
}

// SegregateDiscarderFinder defines the set of methods for contents to evaluate
// whether they should be discarded from the list of contents they belong to, to
// be properly classified and to be found among the different databases. See
// Discarder, Segregator and ContentFinder.
type SegregateDiscarderFinder interface {
	Discarder
	Segregator
	ContentFinder
}

// segregatedContents holds the contents classified into three categories,
// according to the ones available in pbMetadata; they refer to the content
// status, which is a way of describing the quality of the content at a given
// time.
type segregatedContents struct {
	newContents []ContentFinder
	relContents []ContentFinder
	topContent  ContentFinder
}

// segregate classifies the contents into three categories: new, relevant and
// top and returns the result into a *segregatedContents instance.
func segregate(contents []SegregateFinder) *segregatedContents {
	// segregated contents
	segContents := new(segregatedContents)

	// segregate contents only if there are contents
	if len(contents) > 0 {
		// set the first content as the top content.
		// it will probably change.
		topContent := contents[0]
		// set relevant and new contents
		for _, content := range contents {
			if content.IsRelevant() {
				// add to list of relevant contents
				segContents.relContents = append(segContents.relContents, ContentFinder(content))
			} else {
				// add to list of new contents
				segContents.newContents = append(segContents.newContents, ContentFinder(content))
			}
		}
		// fetch top thread from the list of relevant contents only if there
		// were found relevant contents.
		if len(segContents.relContents) > 0 {
			// Top Content Index
			var TCI int
			// search for the most top content
			for idx, content := range segContents.relContents {
				if topContent.IsLessRelevantThan(content) {
					// new topContent found
					topContent = SegregateFinder(content)
					TCI = idx
				}
			}

			// copy the last element of relContents into the position at
			// which the topContent was found.
			last := len(segContents.relContents) - 1
			segContents.relContents[TCI] = segContents.relContents[last]

			// remove the last element from the list of relevant contents
			// by reslicing relContents and leaving out the last element.
			segContents.relContents = segContents.relContents[:last]
		} else {
			// at the beginning, the first content was set as the top content.
			//
			// copy the last element from newContents into the position at
			// which the topContent was retrieved from (first content).
			last := len(segContents.newContents) - 1
			segContents.newContents[0] = segContents.newContents[last]

			// remove the last element from the list of new contents by
			// reslicing newContents and leaving the last element out.
			segContents.newContents = segContents.newContents[:last]
		}
		// set topContent
		segContents.topContent = ContentFinder(topContent)
	}
	return segContents
}

// fetch takes two slices of ContentFinder, one with contents of an expected
// status and the other with contents of optional status.
//
// It returns a content fetched from either the main contents or the optional
// contents in a random fashion, both the main contents and the optional contents
// without the element containing the content just fetched, and a boolean
// indicating whether or not both the main contents list and the optional contents
// list have no elements.
func fetch(mainContents, optContents []ContentFinder) (ContentFinder,
	[]ContentFinder, []ContentFinder, bool) {
	var content ContentFinder
	var empty bool
	// check whether there are contents with the expected status
	if len(mainContents) > 0 {
		// check whether to follow the pattern and fetch a random content of the
		// expected status (i.e. from mainContents)
		if fetchExpectedType() {
			content, mainContents = fetchRandomContent(mainContents)
		} else {
			// check whether there are contents with the optional status
			if len(optContents) > 0 {
				content, optContents = fetchRandomContent(optContents)
			} else {
				content, mainContents = fetchRandomContent(mainContents)
			}
		}
	} else if len(optContents) > 0 {
		content, optContents = fetchRandomContent(optContents)
	} else {
		// both mainContents and optContents are empty.
		empty = true
	}
	return content, mainContents, optContents, empty
}

// fetchRandomContent fetches out one content from the list of contents in a
// random fashion and returns the content and the list of contents without the
// element just fetched out.
func fetchRandomContent(contents []ContentFinder) (ContentFinder, []ContentFinder) {
	idx := rand.NextInt(0, len(contents))
	// copy content at position idx
	content := contents[idx]
	// copy the last element from contents into the position at which the
	// content was fetched out.
	last := len(contents) - 1
	contents[idx] = contents[last]
	// remove the last element from the original list of contents by reslicing
	// it and leaving the last element out.
	contents = contents[:last]
	// allocate a new slice without the last element and copy contents to it.
	reducedContents := make([]ContentFinder, len(contents))
	copy(reducedContents, contents)
	return content, reducedContents
}

// fetchExpectedType returns true on a probability of 80% and false on a
// probability of 20%.
func fetchExpectedType() bool {
	values := [10]bool{true,true,true,true,true,true,true,true,true,true}
	i := rand.NextInt(0, 10)
	values[i] = false
	i = rand.NextInt(0, 10)
	values[i] = false
	i = rand.NextInt(0, 10)
	return values[i]
}
