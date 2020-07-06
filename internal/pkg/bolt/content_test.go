package bolt_test

import (
	"time"
	"testing"
	"strconv"
	"math/rand"

	dbmodel "github.com/luisguve/cheroapi/internal/app/cheroapi"
	pbTime "github.com/golang/protobuf/ptypes/timestamp"
	pbContext "github.com/luisguve/cheroproto-go/context"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
	pbApi "github.com/luisguve/cheroproto-go/cheroapi"
	"github.com/luisguve/cheroapi/internal/pkg/bolt"
)

type post struct {
	content *pbApi.Content
	section *pbContext.Section
	// Expected permalink to get back.
	expId   string
}

func TestCreateThread(t *testing.T) {
	db, err := bolt.New("db")
	if err != nil {
		t.Errorf("DB open error: %v\n", err)
	}
	userKeys := make(map[string]user)
	var ids []string
	// Register users.
	for _, u := range users {
		userId, st := db.RegisterUser(u.email, u.name, u.patillavatar, u.username, u.alias, u.about, u.password)
		if st != nil {
			t.Errorf("Got status %v: %v\n", st.Code(). st.Message())
		}
		ids = append(ids, userId)
		userKeys[userId] = u
	}

	for _, t := range threads {
		i := rand.NextInt(0, len(ids))
		user := ids[i]
		permalink, err := db.CreateThread(t.content, t.section, user)
		if err != nil {
			t.Errorf("Got err: %v\n", err)
		}
		if !strings.Contains(permalink, t.expId) {
			t.Errorf("Expected permalink: %v\nGot: %v\n", t.expId, permalink)
		}
	}
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

var threads := []post{
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 1",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expId: "/mylife/awesome-blog-post-1",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 2",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(5 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expId: "/mylife/awesome-blog-post-2",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 3",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(6 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expId: "/mylife/awesome-blog-post-3",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 4",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(7 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expId: "/mylife/awesome-blog-post-4",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 5",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(8 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expId: "/mylife/awesome-blog-post-5",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 6",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(9 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expId: "/mylife/awesome-blog-post-6",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 7",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(10 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expId: "/mylife/awesome-blog-post-7",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 8",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(11 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expId: "/mylife/awesome-blog-post-8",
	},
	post{
		content: &pbApi.Content{
			Title:       "Awesome blog post 9",
			Content:     "Lorem ipsum dolor sit amet... Lest assume this is a long post",
			FtFile:      "fresh-watermelon.jpg",
			PublishDate: &pbTime.Timestamp{
				Seconds: time.Now().Add(12 * time.Minute).Unix(),
			},
		},
		section: &pbContext.Section{
			Id: "mylife",
		},
		expId: "/mylife/awesome-blog-post-9",
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
		expId: "/mylife/awesome-blog-post-10",
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
		expId: "/mylife/awesome-blog-post-11",
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
		expId: "/mylife/awesome-blog-post-12",
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
		expId: "/mylife/awesome-blog-post-13",
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
		expId: "/mylife/awesome-blog-post-14",
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
		expId: "/mylife/awesome-blog-post-15",
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
		expId: "/mylife/awesome-blog-post-16",
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
		expId: "/mylife/awesome-blog-post-17",
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
		expId: "/mylife/awesome-blog-post-18",
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
		expId: "/mylife/awesome-blog-post-19",
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
		expId: "/mylife/awesome-blog-post-20",
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
		expId: "/mylife/awesome-blog-post-21",
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
		expId: "/mylife/awesome-blog-post-22",
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
		expId: "/mylife/awesome-blog-post-23",
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
		expId: "/mylife/awesome-blog-post-24",
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
		expId: "/mylife/awesome-blog-post-25",
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
		expId: "/mylife/awesome-blog-post-26",
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
		expId: "/mylife/awesome-blog-post-27",
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
		expId: "/mylife/awesome-blog-post-28",
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
		expId: "/mylife/awesome-blog-post-29",
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
		expId: "/mylife/awesome-blog-post-30",
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
		expId: "/mylife/awesome-blog-post-31",
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
		expId: "/mylife/awesome-blog-post-32",
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
		expId: "/mylife/awesome-blog-post-33",
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
		expId: "/mylife/awesome-blog-post-34",
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
		expId: "/mylife/awesome-blog-post-35",
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
		expId: "/mylife/awesome-blog-post-36",
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
		expId: "/mylife/awesome-blog-post-37",
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
		expId: "/mylife/awesome-blog-post-38",
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
		expId: "/mylife/awesome-blog-post-39",
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
		expId: "/mylife/awesome-blog-post-40",
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
		expId: "/mylife/awesome-blog-post-41",
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
		expId: "/mylife/awesome-blog-post-42",
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
		expId: "/mylife/awesome-blog-post-43",
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
		expId: "/mylife/awesome-blog-post-44",
	},
}
