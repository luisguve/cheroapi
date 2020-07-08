package bolt_test

import (
	"time"
	"os"
	"sync"
	"testing"
	"fmt"
	"math/rand"
	"strings"
	"io/ioutil"

	dbmodel "github.com/luisguve/cheroapi/internal/app/cheroapi"
	pbTime "github.com/golang/protobuf/ptypes/timestamp"
	pbContext "github.com/luisguve/cheroproto-go/context"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	"github.com/luisguve/cheroapi/internal/pkg/bolt"
)

type user struct {
	email, name, patillavatar, username, alias, about, password string
}

type post struct {
	content *pbApi.Content
	section *pbContext.Section
	// Expected permalink to get back.
	expLink   string
}

// Map section ids to post permalink (includes section id).
var sectionPosts = make(map[string][]string)
// Map post id to user id.
var postAuthor = make(map[string]string)
// Map post id to post.
var idPost = make(map[string]post)

type comment struct {
	// thread *pbContext.Thread
	content  dbmodel.Reply
	// expNotif *pbApi.NotifyUser
}

// Register users, then create threads, then leave replies on those threads.
func TestCreateThread(t *testing.T) {
	dir, err := ioutil.TempDir("db", "storage")
	if err != nil {
		t.Fatalf("Error in test: %v\n", err)
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Errorf("RemoveAll Error: %v\n", err)
		}
	}()
	db, err := bolt.New(dir)
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
	for _, u := range users {
		userId, st := db.RegisterUser(u.email, u.name, u.patillavatar, u.username, u.alias, u.about, u.password)
		if st != nil {
			t.Errorf("Got status %v: %v\n", st.Code(), st.Message())
		}
		ids = append(ids, userId)
		userKeys[userId] = u
	}
	// Create 44 threads.
	var wg sync.WaitGroup
	var m sync.Mutex
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
	// Post a comment on each thread.
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
								Id:         postId,
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
}

var users = map[string]user{
	"usr1": user{
		email: "luisguveal@gmail.com",
		name: "Luis Villegas",
		patillavatar: "pic.jpg",
		username: "luisguve",
		alias: "Luis",
		about: "Some description about myself",
		password: "1747018Lv/",
	},
	"usr2": user{
		email: "otheruser@other.com",
		name: "Other User",
		patillavatar: "otherpic.jpg",
		username: "other",
		alias: "Other",
		about: "Some other description",
		password: "digital-dissent",
	},
	"usr3": user{
		email: "cheesetris21@gmail.com",
		name: "Artur Car",
		patillavatar: "ctpic.png",
		username: "cheesetris21",
		alias: "Cheez",
		about: "Cheese description",
		password: "436173918//",
	},
}

var posts = []post{
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 01",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-01",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 02",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(5 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-02",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 03",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(6 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-03",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 04",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(7 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-04",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 05",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(8 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-05",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 06",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(9 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-06",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 07",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(10 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-07",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 08",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(11 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-08",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 09",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(12 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-09",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 10",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(13 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-10",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 11",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(14 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-11",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 12",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(15 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-12",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 13",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(16 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-13",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 14",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(17 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-14",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 15",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(18 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-15",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 16",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(19 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-16",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 17",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(20 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-17",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 18",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(21 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-18",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 19",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(22 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-19",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 20",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(23 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-20",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 21",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(24 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-21",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 22",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(25 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-22",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 23",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(26 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-23",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 24",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(27 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-24",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 25",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(28 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-25",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 26",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(29 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-26",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 27",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(30 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-27",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 28",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(31 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-28",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 29",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(32 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-29",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 30",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(33 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-30",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 31",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(34 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-31",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 32",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(35 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-32",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 33",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(36 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-33",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 34",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(37 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-34",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 35",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(38 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-35",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 36",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(39 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-36",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 37",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(40 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-37",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 38",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(41 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-38",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 39",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(42 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-39",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 40",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(43 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-40",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 41",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(44 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-41",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 42",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(45 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-42",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 43",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(46 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-43",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 44",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(47 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expLink: "/mylife/awesome-blog-post-44",
	},
}

var comments = []comment{
	comment{
		content: dbmodel.Reply{
			Content: "(1) HEY yo! I'm leaving a comment on your amazing post.",
			FtFile: "animated_pic.gif",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Unix(),
			},
		},
	},
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
	},
}
