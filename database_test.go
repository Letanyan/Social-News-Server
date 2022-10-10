package main

import (
	"math/rand"
	"testing"
)

func TestDatabase(t *testing.T) {
	db := getTestDatabase()
	defer db.Close()
	DBClearAllTables(db)
	DBSetup(db)

	usersTC := []struct {
		name     string
		email    string
		password string
	}{
		{"rb t", "ribet@yahoo.com", "123456"},
		{"ako", "akoblin@gmail.com", "password"},
		{"ps", "portscan@comcast.net", "1234"},
		{"noutlook", "mhanoh@outlook.com", "000"},
		{"monsolo", "solomon@hotmail.com", "00000"},
		{"com grady", "grady@comcast.net", "p15423"},
		{"mac wag", "wagnerch@mac.com", "fniweufjk"},
		{"jmail", "jandrese@gmail.com", "fcn5893gq8op%&"},
		{"barnot", "barnett@hotmail.com", "fjijfiejfiej"},
		{"yahear", "greear@yahoo.com", "geer"},
		{"toku", "tokuhirom@sbcglobal.net", "pass"},
		{"fat elk", "fatelk@gmail.com", "word"},
	}

	matchUsers := func(name string, a User, b User) {
		if a.ID != b.ID {
			t.Errorf("Mismatched id %d != %d", a.ID, b.ID)
		}
		if a.Name != b.Name {
			t.Errorf("Mismatched name %s != %s", a.Name, b.Name)
		}
		if a.Email != b.Email {
			t.Errorf("Mismatched email %s != %s", a.Email, b.Email)
		}
		if a.Password != b.Password {
			t.Errorf("Mismatched password %s != %s", a.Password, b.Password)
		}
		if a.RegisterDate != b.RegisterDate {
			t.Errorf("Mismatched registerDate %v != %v", a.RegisterDate, b.RegisterDate)
		}
	}
	matchUserProfile := func(name string, a UserProfile, b User) {
		if a.ID != b.ID {
			t.Errorf("Mismatched id %d != %d", a.ID, b.ID)
		}
		if a.Name != b.Name {
			t.Errorf("Mismatched name %s != %s", a.Name, b.Name)
		}
	}

	rand.Seed(63487)
	users := []User{}
	for _, tc := range usersTC {
		user := DBCreateUser(db, tc.name, tc.email, tc.password)
		DBValidateUser(db, user.ID, user.ValidationKey)
		if !DBEqualHashAndPassword(user.Password, tc.password) {
			t.Errorf("user password hash failed %s != %s", user.Password, tc.password)
		}
		users = append(users, user)
	}
	for i, source := range users {
		t.Run(source.Name, func(t *testing.T) {
			// t.Logf("%v\n", source)
			rand.Seed(int64(i))
			user1 := DBGetUser(db, source.ID, "")
			matchUsers("id matched user", source, user1)
			user2 := DBGetUser(db, 0, source.Email)
			matchUsers("email matched user", source, user2)

			if user1.ValidationKey != 0 {
				t.Errorf("user validation failed")
			}

			newPassword := usersTC[rand.Intn(len(usersTC))].password
			DBUpdatePasswordForUser(db, source.ID, usersTC[i].password, newPassword)
			user3 := DBGetUser(db, source.ID, "")
			if !DBEqualHashAndPassword(user3.Password, newPassword) {
				t.Errorf("not matching password (%s, %s) after update", user3.Password, newPassword)
			}

			otherUser := users[rand.Intn(len(users))]
			user4 := DBGetUser(db, 0, otherUser.Email)
			amount := int64(rand.Intn(50)) * sign(rand.Intn(2) == 0)
			date := utc().AddDate(0, 0, rand.Intn(25)*int(sign(rand.Intn(2) == 0))).Format("2006-01-02")
			srcUser, srcPref := DBVoteForUser(db, source.ID, user4.ID, amount, []string{}, date)

			user5 := DBGetUser(db, user4.ID, "")
			if amount < 0 && user5.Downvotes < 0 {
				t.Errorf("Failed to update user total downvotes")
			} else if amount > 0 && user5.Upvotes < 0 {
				t.Errorf("Failed to update user total upvotes")
			}
			matchUserProfile("match vote and get user", srcUser, user5)

			userPrefs := DBGetUserPref(db, source.ID, soScore, upUser, user5.ID, 0, 0, 0, 10, 0)
			if len(userPrefs) != 1 {
				t.Errorf("failed to get user prefs for user %d", source.ID)
			} else {
				p := userPrefs[0]
				if amount < 0 && p.Downvotes < float64(amount) {
					t.Errorf("failed to update user pref downvotes for other user")
				} else if amount > 0 && p.Upvotes < float64(amount) {
					t.Errorf("failed to update user pref upvotes for other user")
				}
				if p.Downvotes != srcPref.Downvotes || p.Upvotes != srcPref.Upvotes ||
					p.Kind != srcPref.Kind || p.PID != srcPref.PID || p.SID != srcPref.SID {
					t.Errorf("mismatch between get user pref and vote user pref")
				}
			}

		})
	}

	posts := []struct {
		content  string
		userId   int64
		tags     []string
		location []string
	}{
		{"After several other French cities", 1, []string{"france", "boycott", "qatar", "world cup"}, []string{"Europe", "Germany", "Hoffenheim", "1880"}},
		{"What Modric is doing, playing at this level at his age", 2, []string{"Modric", "football", "age"}, []string{"Europe", "Spain", "Madrid", "10"}},
		{"How the PL table shapes up after Matchweek 9", 1, []string{"PL", "table"}, []string{"Europe", "Germany", "Hoffenheim", "1880"}},
		{"Gareth Bale launches lager and ale", 3, []string{"Bale", "Lager"}, []string{"South America", "Brazil", "Sao Paolo", "1888"}},
		{"I know why the caged archon sings", 4, []string{"Genshin", "Nahida"}, []string{"Africa", "Egypt", "Giza", "3"}},
		{"central berg in spring is something else", 5, []string{"Drakensberg", "Spring"}, []string{"Africa", "South Africa", "Free State", "Drakensberg"}},
		{"Views from Lion's Head Cape Town. A moderate hike to the top", 6, []string{"Lions Head", "Hike"}, []string{"Africa", "South Africa", "West Cape", "Cape Town"}},
		{"Whats your go-to radio station to listen or stream", 5, []string{"Music", "Stream", "radio"}, []string{"Africa", "South Africa", "Free State", "Drakensberg"}},
	}

	matchPost := func(a Post, b PostResult) {
		if a.ID != b.ID {
			t.Errorf("Mismatch id %d != %d", a.ID, b.ID)
		}
		if a.Content != b.Content {
			t.Errorf("Mismatch content %s != %s", a.Content, b.Content)
		}
		if a.UserID != b.Author.ID {
			t.Errorf("Mismatch userId %d != %d", a.UserID, b.Author.ID)
		}
		// if a.tags != b.tags {
		// 	t.Errorf("Mismatch content %v != %v", a.content, b.content)
		// }
		// if a.location != b.location {
		// 	t.Errorf("Mismatch id %d != %d", a.id, b.id)
		// }
		if a.CreatedAt != b.CreatedAt {
			t.Errorf("Mismatch createdAt %v != %v", a.CreatedAt, b.CreatedAt)
		}
		if a.Upvotes != b.Upvotes {
			t.Errorf("Mismatch upvotes %f != %f", a.Upvotes, b.Upvotes)
		}
		if a.Downvotes != b.Downvotes {
			t.Errorf("Mismatch downvotes %f != %f", a.Downvotes, b.Downvotes)
		}
	}

	// startOfYear := time.Date(utc().Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
	// endOfYear := time.Date(utc().Year(), time.December, 31, 23, 59, 59, 999999, time.UTC)
	for i, tc := range posts {
		t.Run(tc.content, func(t *testing.T) {
			rand.Seed(int64(i))
			source := DBCreatePost(db, tc.userId, tc.content, tc.tags, tc.location)
			post1 := DBGetPost(db, source.ID)
			matchPost(source, post1)
			matchUserProfile("", post1.Author, users[tc.userId-1])

			for i := 0; i < rand.Intn(10); i += 1 {
				odx := rand.Intn(len(users))
				otherUser := users[odx]
				other := DBGetUser(db, 0, otherUser.Email)
				pidx := rand.Intn(len(posts))

				comt, cont := DBCreateComment(db, other.ID, posts[pidx].content, post1.ID, 0)

				if comt.Content != posts[pidx].content {
					t.Errorf("comment content not correct")
				}
				if comt.UserID != other.ID {
					t.Errorf("comment user id not correct")
				}
				if cont.CommentID != comt.ID {
					t.Errorf("comment id not correct on user cont")
				}
				if cont.PostID != comt.PostID {
					t.Errorf("comment post id not correct on user cont")
				}
			}

			for i := 0; i < rand.Intn(50); i += 1 {
				odx := rand.Intn(len(users))
				otherUser := users[odx]
				other := DBGetUser(db, 0, otherUser.Email)
				pidx := rand.Intn(len(posts))
				loc := posts[pidx].location
				amount := int64(rand.Intn(50)) * sign(rand.Intn(2) == 0)
				date := utc().AddDate(0, 0, rand.Intn(25)*int(sign(rand.Intn(2) == 0))).Format("2006-01-02")
				post2, userPoster, tags, prefs := DBVotePost(db, other.ID, source.ID, amount, loc, date)

				if amount < 0 && post2.Downvotes < float64(amount) {
					t.Errorf("Post downvotes not updated")
				} else if amount > 0 && post2.Upvotes < float64(amount) {
					t.Errorf("Post upvotes not updated")
				}
				if amount < 0 && userPoster.Downvotes < float64(amount) {
					t.Errorf("Post downvotes for poster not updated")
				} else if amount > 0 && userPoster.Upvotes < float64(amount) {
					t.Errorf("Post upvotes for poster not updated")
				}
				uPref := prefs[0]
				if uPref.Kind != upUser && uPref.PID != other.ID {
					t.Errorf("User pref not update user voted for")
				}
				if amount < 0 && uPref.Downvotes < float64(amount) {
					t.Errorf("User pref downvotes for poster not updated")
				} else if amount > 0 && uPref.Upvotes < float64(amount) {
					t.Errorf("User pref upvotes for poster not updated")
				}
				pPref := prefs[1]
				if pPref.Kind != upPost && pPref.PID != source.ID {
					t.Errorf("User pref not update user voted for")
				}
				if amount < 0 && pPref.Downvotes < float64(amount) {
					t.Errorf("User pref downvotes for post not updated")
				} else if amount > 0 && pPref.Upvotes < float64(amount) {
					t.Errorf("User pref upvotes for post not updated")
				}

				for i, tag := range tags {
					if tc.tags[i] != tag.Name {
						t.Errorf("Mismatch of tag names")
					}
				}
				for i := 2; i < len(prefs); i += 1 {
					if tags[i-2].ID != prefs[i].PID && prefs[i].Kind != upTag {
						t.Errorf("Mismatch of tag id and user pref id")
					}
				}
			}
		})
	}
}
