package contents

// inSlice returns whether user is in users and an integer indicating the index
// where the user id is in the slice. Returns false an 0 if the user is not in
// users slice.
func inSlice(users []string, user string) (bool, int) {
	for idx, u := range users {
		if u == user {
			return true, idx
		}
	}
	return false, 0
}
