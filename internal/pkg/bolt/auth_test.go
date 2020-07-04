package bolt_test

import (
	"testing"
	"strconv"

	dbmodel "github.com/luisguve/cheroapi/internal/app/cheroapi"
	pbDataFormat "github.com/luisguve/cheroproto-go/dataformat"
	"github.com/luisguve/cheroapi/internal/pkg/bolt"
)

type user struct {
	email, name, patillavatar, username, alias, about, password string
}

// Register users, get them, get their ids through they usernames and emails,
// update their data, and update their usernames.
func TestAuthUser(t *testing.T) {
	db, err := bolt.New("db")
	if err != nil {
		t.Errorf("DB open error: %v\n", err)
	}
	users := map[string]user{
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
	userKeys := make(map[string]user)
	pbUsers := make(map[string]*pbDataFormat.User)
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
	// Get users.
	for _, id := range ids {
		pbUser, err := db.User(id)
		if err != nil {
			t.Errorf("Got err: %v\n", err)
		}
		user := userKeys[id]
		equals := (user.email == pbUser.PrivateData.Email) &&
		(user.name == pbUser.BasicUserData.Name) &&
		(user.patillavatar == pbUser.BasicUserData.PicUrl) &&
		(user.username == pbUser.BasicUserData.Username) &&
		(user.alias == pbUser.BasicUserData.Alias) &&
		(user.about == pbUser.BasicUserData.About)
		if !equals {
			t.Errorf("Expected: %v\nGot: %v\n", printUser(user), printPbBasicUserData(pbUser))
		}
		pbUsers[id] = pbUser
	}
	// Update users.
	for _, id := range ids {
		// append _UPDATED to about.
		pbUsers[id].BasicUserData.About += "_UPDATED"
		err = db.UpdateUser(pbUsers[id], id)
		if err != nil {
			t.Errorf("Got err: %v\n", err)
		}
	}
	// Get users again and compare updated about.
	for _, id := range ids {
		pbUser, err := db.User(id)
		if err != nil {
			t.Errorf("Got err: %v\n", err)
		}
		user := userKeys[id]
		user.about += "_UPDATED"
		equals := user.about == pbUser.BasicUserData.About
		if !equals {
			t.Errorf("Expected: %v\nGot: %v\n", printUser(user), printPbBasicUserData(pbUser))
		}
	}
	// Get user id by username.
	for _, id := range ids {
		username := userKeys[id].username
		idBytes, err := db.FindUserIdByUsername(username)
		if err != nil {
			t.Errorf("Got err: %v\n", err)
		}
		if !(id == string(idBytes)) {
			t.Errorf("Expected %v\nGot: %v\n", id, string(idBytes))
		}
	}
	// Get user id by email.
	for _, id := range ids {
		email := userKeys[id].email
		idBytes, err := db.FindUserIdByEmail(email)
		if err != nil {
			t.Errorf("Got err: %v\n", err)
		}
		if !(id == string(idBytes)) {
			t.Errorf("Expected %v\nGot: %v\n", id, string(idBytes))
		}
	}
	// Change username of users, append a number to it.
	num := 1
	for _, id := range ids {
		sNum := strconv.Itoa(num)
		// Build new username.
		username := pbUsers[id].BasicUserData.Username + sNum
		err := db.MapUsername(username, id)
		if err != nil {
			t.Errorf("Got err: %v\n", err)
		}
		num++
		// Set new username and save user.
		userKeys[id].username = username
		pbUsers[id].BasicUserData.Username = username
		err = db.UpdateUser(pbUser, id)
		if err != nil {
			t.Errorf("Got err: %v\n", err)
		}
	}
	// Get user id by username again.
	for _, id := range ids {
		username := pbUser[id].BasicUserData.Username
		idBytes, err := db.FindUserIdByUsername(username)
		if err != nil {
			t.Errorf("Got err: %v\n", err)
		}
		if !(id == string(idBytes)) {
			t.Errorf("Expected %v\nGot: %v\n", id, string(idBytes))
		}
	}
}

func printPbBasicUserData(pbUser *pbDataFormat.User) string {
	private := pbUser.PrivateData
	basic := pbUser.BasicUserData
	return fmt.Sprintf("email: %v, name: %v, picurl: %v, username: %v, alias: %v, about: %v\n", 
		private.Email, basic.Name, basic.PicUrl, basic.Username, basic.Alias, basic.About)
}

func printUser(u user) string {
	return fmt.Sprintf("email: %v, name: %v, picurl: %v, username: %v, alias: %v, about: %v\n", 
		u.email, u.name, u.patillavatar, u.username, u.alias, u.about)
}
