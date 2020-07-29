package bolt_test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	pbTime "github.com/golang/protobuf/ptypes/timestamp"
	dbmodel "github.com/luisguve/cheroapi/internal/app/cheroapi"
	"github.com/luisguve/cheroapi/internal/pkg/bolt"
	"github.com/luisguve/cheroapi/internal/pkg/patillator"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	pbContext "github.com/luisguve/cheroproto-go/context"
)

type user struct {
	email, name, patillavatar, username, alias, about, password string
}

type post struct {
	content *pbApi.Content
	section *pbContext.Section
	// Expected permalink to get back.
	expLink string
}

type comment struct {
	// thread *pbContext.Thread
	content dbmodel.Reply
	// expNotif *pbApi.NotifyUser
}

// Map section ids to post permalink (includes section id).
var sectionPosts = make(map[string][]string)

// Map post id to user id.
var postAuthor = make(map[string]string)

// Map post id to post.
var idPost = make(map[string]post)

var sections = map[string]string{"mylife": "My life"}

// Register users, then create threads, then leave replies on those threads.
func TestQA(t *testing.T) {
	dir, err := ioutil.TempDir("db", "storage")
	if err != nil {
		t.Fatalf("Error in test: %v\n", err)
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("RemoveAll Error: %v\n", err)
		}
	}()
	db, err := bolt.New(dir, sections)
	if err != nil {
		t.Errorf("DB open error: %v\n", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Errorf("DB Close error: %v\n", err)
		}
	}()
	userKeys := make(map[string]user)
	var ids []string
	// Register users.
	t.Log("Register users")
	for _, u := range users {
		userId, st := db.RegisterUser(u.email, u.name, u.patillavatar, u.username, u.alias, u.about, u.password)
		if st != nil {
			t.Errorf("Got status %v: %v\n", st.Code(), st.Message())
		}
		ids = append(ids, userId)
		userKeys[userId] = u
	}
	t.Log("Finished register users")
	// Create 44 threads.
	var wg sync.WaitGroup
	var m sync.Mutex
	t.Log("Create threads")
	for _, p := range posts {
		wg.Add(1)
		go func(p post) {
			defer wg.Done()
			i := rand.Intn(len(ids))
			user := ids[i]
			permalink, err := db.CreateThread(p.content, p.section, user)
			if err != nil {
				t.Errorf("Got err: %v\n", err)
			}
			if !strings.Contains(permalink, p.expLink) {
				t.Errorf("Expected permalink: %v\nGot: %v\n", p.expLink, permalink)
			}
			// The permalink comes in the format "/{section-id}/{thread-id}",
			// guaranteeing that there will not be collisions (or value overrides)
			// in between threads from different sections that turned out to have
			// the same id.
			postIds := sectionPosts[p.section.Id]
			postIds = append(postIds, permalink)
			m.Lock()
			defer m.Unlock()
			sectionPosts[p.section.Id] = postIds
			// Associate permalink to user id of author.
			postAuthor[permalink] = user
			// Associate permalink to post.
			idPost[permalink] = p
		}(p)
	}
	wg.Wait()
	t.Log("Finished creating threads.")
	// Post a comment on each thread.
	t.Log("Reply threads.")
	for _, c := range comments {
		wg.Add(1)
		go func(c comment) {
			defer wg.Done()
			for section, postPermalinks := range sectionPosts {
				wg.Add(1)
				go func(section string, postPermalinks []string) {
					defer wg.Done()
					for _, postPermalink := range postPermalinks {
						wg.Add(1)
						prefix := fmt.Sprintf("/%s/", section)
						postId := strings.TrimPrefix(postPermalink, prefix)
						go func(section, permalink, postId string, r dbmodel.Reply) {
							defer wg.Done()
							ctx := &pbContext.Thread{
								Id: postId,
								SectionCtx: &pbContext.Section{
									Id: section,
								},
							}
							// The submitter may be replying it's own thread, in
							// which case the returned notification should be nil.
							i := rand.Intn(len(ids))
							r.Submitter = ids[i]
							notifyUser, err := db.ReplyThread(ctx, r)
							if err != nil {
								t.Fatalf("Got error while posting reply: %v\n", err)
							}
							// t.Log("Just replied a thread.")
							if r.Submitter == postAuthor[permalink] {
								// The submitter is the thread author; there must
								// not be any notification.
								if notifyUser != nil {
									t.Errorf("Got notification, but the replier is the author.\n")
								}
								return
							}
							// The submitter is not the thread author; the notification
							// must be for the thread author.
							equals := postAuthor[permalink] == notifyUser.UserId
							if !equals {
								t.Errorf("Post author (%s) != notifyUser Id (%s)\n", postAuthor[permalink], notifyUser.UserId)
							}
							expSubject := fmt.Sprintf("On your thread %s", idPost[permalink].content.Title)
							equals = expSubject == notifyUser.Notification.Subject
							if !equals {
								t.Errorf("received subject (%s) != expected subject (%s)\n", expSubject, notifyUser.Notification.Subject)
							}
						}(section, postPermalink, postId, c.content)
					}
				}(section, postPermalinks)
			}
		}(c)
	}
	wg.Wait()
	t.Log("Finished replying threads.")
	// Get all the threads in "mylife" section. There should be 44.
	section := &pbContext.Section{
		Id: "mylife",
	}
	t.Log("GetThreadsOverview")
	threads, err := db.GetThreadsOverview(section)
	if err != nil {
		t.Fatalf("Got err: %v\n", err)
	}
	t.Log("Finished GetThreadsOverview")
	// Copy every id from the section into a new variable. Each time a thread
	// id is received, it will be removed from the slice of ids. At the end,
	// the slice of copies should be empty.
	var idCopies = make([]string, len(sectionPosts["mylife"]))
	copy(idCopies, sectionPosts["mylife"])
	var threadIds []patillator.Id
	for _, thread := range threads {
		threadId, ok := thread.Key().(string)
		if !ok {
			t.Errorf("Expected key to be string, but got: %v\n", thread.Key())
			continue
		}
		var found bool
		for idx, idCopy := range idCopies {
			// idCopy holds a permalink, which includes the section.
			idCopy = strings.TrimPrefix(idCopy, "/mylife/")
			if idCopy == threadId {
				found = true
				last := len(idCopies) - 1
				idCopies[idx] = idCopies[last]
				idCopies = idCopies[:last]
				patillaId := patillator.Id{Id: threadId, Status: "NEW"}
				threadIds = append(threadIds, patillaId)
				break
			}
		}
		if !found {
			t.Errorf("id %v not found in copies!\n", threadId)
		}
	}
	if len(idCopies) != 0 {
		t.Errorf("idCopies should be empty. These were left: %v\n", idCopies)
	}
	// Upvoting threads. Thread "Awesome blog post 03" is going to receive
	// len(users) upvotes.
	// Then, it will be gotten and compare the number of upvotes, which
	// should match.
	// Get Id of thread "Awesome blog post 03"
	var (
		post03Id string
		post03Permalink string
		found bool
	)
	for _, id := range sectionPosts["mylife"] {
		if strings.Contains(id, "post-03") {
			found = true
			post03Id = strings.TrimPrefix(id, "/mylife/")
			post03Permalink = id
			break
		}
	}
	if !found {
		t.Fatalf("Could not find id of \"Awesome blog post 03\"")
	}
	t.Log("Upvoting thread \"Awesome blog post 03\".")
	threadCtx := &pbContext.Thread{
		Id: post03Id,
		SectionCtx: &pbContext.Section{
			Id: "mylife",
		},
	}
	var upvotesWG sync.WaitGroup
	n := len(users)
	for i := 0; i < n; i++ {
		upvotesWG.Add(1)
		go func(userId string) {
			defer upvotesWG.Done()
			notifyUser, err := db.UpvoteThread(userId, threadCtx)
			if err != nil {
				t.Errorf("Got err: %v\n", err)
				return
			}
			if userId == postAuthor[post03Permalink] {
				// The upvoter is the thread author; there must not be
				// any notification.
				if notifyUser != nil {
					t.Errorf("Got notification, but the upvoter is the author.\n")
				}
				return
			}
			// The submitter is not the thread author; the notification
			// must be for the thread author.
			equals := postAuthor[post03Permalink] == notifyUser.UserId
			if !equals {
				t.Errorf("Post author (%s) != notifyUser Id (%s)\n", postAuthor[post03Permalink], notifyUser.UserId)
			}
			expSubject := fmt.Sprintf("On your thread %s", idPost[post03Permalink].content.Title)
			equals = expSubject == notifyUser.Notification.Subject
			if !equals {
				t.Errorf("received subject (%s) != expected subject (%s)\n", expSubject, notifyUser.Notification.Subject)
			}
		}(ids[i])
	}
	upvotesWG.Wait()
	contentRule, err := db.GetThread(threadCtx)
	if err != nil {
		t.Errorf("Got err: %v\n", err)
		return
	}
	// awesome-blog-post-xx-hashseq
	logTitle := strings.TrimPrefix(post03Id, "awesome-blog-")[:7]
	upvotes := int(contentRule.Data.Metadata.Upvotes)
	if n != upvotes {
		t.Errorf("%s should have %d upvotes, but have %d\n", logTitle, n, upvotes)
	}
	t.Log("Finished upvoting \"Awesome blog post 03\".")
	// Post 50 comments on "Awesome blog post 03".
	t.Log("Posting 50 comments on \"Awesome blog post 03\".")
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(r dbmodel.Reply) {
			defer wg.Done()
			ctx := &pbContext.Thread{
				Id: post03Id,
				SectionCtx: &pbContext.Section{
					Id: "mylife",
				},
			}
			// The submitter may be replying it's own thread, in
			// which case the returned notification should be nil.
			i := rand.Intn(len(ids))
			r.Submitter = ids[i]
			notifyUser, err := db.ReplyThread(ctx, r)
			if err != nil {
				t.Fatalf("Got error while posting reply: %v\n", err)
			}
			// t.Log("Just replied a thread.")
			if r.Submitter == postAuthor[post03Permalink] {
				// The submitter is the thread author; there must
				// not be any notification.
				if notifyUser != nil {
					t.Errorf("Got notification, but the replier is the author.\n")
				}
				return
			}
			// The submitter is not the thread author; the notification
			// must be for the thread author.
			equals := postAuthor[post03Permalink] == notifyUser.UserId
			if !equals {
				t.Errorf("Post author (%s) != notifyUser Id (%s)\n", postAuthor[post03Permalink], notifyUser.UserId)
			}
			expSubject := fmt.Sprintf("On your thread %s", idPost[post03Permalink].content.Title)
			equals = expSubject == notifyUser.Notification.Subject
			if !equals {
				t.Errorf("received subject (%s) != expected subject (%s)\n", expSubject, notifyUser.Notification.Subject)
			}
		}(comments[0].content)
	}
	wg.Wait()
	t.Log("Finished replying \"Awesome blog post 03\" 50 times.")
	// Get Id of thread "Awesome blog post 11"
	var (
		post11Id string
		post11Permalink string
	)
	found = false
	for _, id := range sectionPosts["mylife"] {
		if strings.Contains(id, "post-11") {
			found = true
			post11Id = strings.TrimPrefix(id, "/mylife/")
			post11Permalink = id
			break
		}
	}
	if !found {
		t.Fatalf("Could not find id of \"Awesome blog post 11\"")
	}
	// Post 40 comments on "Awesome blog post 11".
	// It is not enough to keep active after QA.
	t.Log("Posting 40 comments on \"Awesome blog post 11\".")
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(r dbmodel.Reply) {
			defer wg.Done()
			ctx := &pbContext.Thread{
				Id: post11Id,
				SectionCtx: &pbContext.Section{
					Id: "mylife",
				},
			}
			// The submitter may be replying it's own thread, in
			// which case the returned notification should be nil.
			i := rand.Intn(len(ids))
			r.Submitter = ids[i]
			notifyUser, err := db.ReplyThread(ctx, r)
			if err != nil {
				t.Fatalf("Got error while posting reply: %v\n", err)
			}
			// t.Log("Just replied a thread.")
			if r.Submitter == postAuthor[post11Permalink] {
				// The submitter is the thread author; there must
				// not be any notification.
				if notifyUser != nil {
					t.Errorf("Got notification, but the replier is the author.\n")
				}
				return
			}
			// The submitter is not the thread author; the notification
			// must be for the thread author.
			equals := postAuthor[post11Permalink] == notifyUser.UserId
			if !equals {
				t.Errorf("Post author (%s) != notifyUser Id (%s)\n", postAuthor[post11Permalink], notifyUser.UserId)
			}
			expSubject := fmt.Sprintf("On your thread %s", idPost[post11Permalink].content.Title)
			equals = expSubject == notifyUser.Notification.Subject
			if !equals {
				t.Errorf("received subject (%s) != expected subject (%s)\n", expSubject, notifyUser.Notification.Subject)
			}
		}(comments[0].content)
	}
	wg.Wait()
	t.Log("Finished replying \"Awesome blog post 11\" 40 times.")
	// All the contents and comments should be moved to archived contents.
	result, err := db.QA()
	if err != nil {
		t.Errorf("Got err: %v.\n", err)
	}
	if result != "" {
		t.Log("Result:", result)
	}
	t.Log("GetThreadsOverview again.")
	threads, err = db.GetThreadsOverview(section)
	if err != nil {
		t.Fatalf("Got err: %v\n", err)
	}
	t.Log("Finished GetThreadsOverview")
	if len(threads) != 2 {
		t.Errorf("Expected 2 thread still active, got %v.\n", len(threads))
	}
}

var users = map[string]user{
	"usr1": user{
		email:        "luisguveal@gmail.com",
		name:         "Luis Villegas",
		patillavatar: "pic.jpg",
		username:     "luisguve",
		alias:        "Luis",
		about:        "Some description about myself",
		password:     "1747018Lv/",
	},
	"usr2": user{
		email:        "otheruser@other.com",
		name:         "Other User",
		patillavatar: "otherpic.jpg",
		username:     "other",
		alias:        "Other",
		about:        "Some other description",
		password:     "digital-dissent",
	},
	"usr3": user{
		email:        "cheesetris21@gmail.com",
		name:         "Artur Car",
		patillavatar: "ctpic.png",
		username:     "cheesetris21",
		alias:        "Cheez",
		about:        "Cheese description",
		password:     "436173918//",
	},
}

var posts = []post{
	post{
		content: &pbApi.Content{
			Title:   "Awesome blog post 01",
			Content: "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:  "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(-24 * time.Hour).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-01",
	},
	post{
		content: &pbApi.Content{
			Title:   "Awesome blog post 02",
			Content: "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:  "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(-24 * time.Hour).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-02",
	},
	post{
		content: &pbApi.Content{
			Title:   "Awesome blog post 03",
			Content: "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:  "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(-24 * time.Hour).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-03",
	},
	post{
		content: &pbApi.Content{
			Title:   "Awesome blog post 04",
			Content: "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:  "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(-24 * time.Hour).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-04",
	},
	// This will NOT be moved to archived contents, since it's been around for
	// less than one day.
	post{
		content: &pbApi.Content{
			Title:   "Awesome blog post 05",
			Content: "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:  "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-05",
	},
	post{
		content: &pbApi.Content{
			Title:   "Awesome blog post 06",
			Content: "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:  "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(-24 * time.Hour).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-06",
	},
	post{
		content: &pbApi.Content{
			Title:   "Awesome blog post 07",
			Content: "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:  "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(-24 * time.Hour).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-07",
	},
	post{
		content: &pbApi.Content{
			Title:   "Awesome blog post 08",
			Content: "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:  "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(-24 * time.Hour).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-08",
	},
	post{
		content: &pbApi.Content{
			Title:   "Awesome blog post 09",
			Content: "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:  "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(-24 * time.Hour).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-09",
	},
	post{
		content: &pbApi.Content{
			Title:   "Awesome blog post 10",
			Content: "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:  "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(-24 * time.Hour).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-10",
	},
	post{
		content: &pbApi.Content{
			Title:   "Awesome blog post 11",
			Content: "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:  "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(-24 * time.Hour).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-11",
	},
	post{
		content: &pbApi.Content{
			Title:   "Awesome blog post 12",
			Content: "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:  "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(-24 * time.Hour).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-12",
	},
}

var comments = []comment{
	comment{
		content: dbmodel.Reply{
			Content: "(1) HEY yo! I'm leaving a comment on your amazing post.",
			FtFile:  "animated_pic.gif",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Unix(),
			},
		},
	}, /*
		comment{
			content: dbmodel.Reply{
				Content: "(2) HEY yo! I'm leaving a comment on your amazing post.",
				FtFile: "animated_pic.gif",
				PublishDate: &pbTime.Timestamp{
					Seconds: time.Now().Unix(),
				},
			},
		},
		comment{
			content: dbmodel.Reply{
				Content: "(3) HEY yo! I'm leaving a comment on your amazing post.",
				FtFile: "animated_pic.gif",
				PublishDate: &pbTime.Timestamp{
					Seconds: time.Now().Unix(),
				},
			},
		},
		comment{
			content: dbmodel.Reply{
				Content: "(4) HEY yo! I'm leaving a comment on your amazing post.",
				FtFile: "animated_pic.gif",
				PublishDate: &pbTime.Timestamp{
					Seconds: time.Now().Unix(),
				},
			},
		},
		comment{
			content: dbmodel.Reply{
				Content: "(5) HEY yo! I'm leaving a comment on your amazing post.",
				FtFile: "animated_pic.gif",
				PublishDate: &pbTime.Timestamp{
					Seconds: time.Now().Unix(),
				},
			},
		},*/
}
