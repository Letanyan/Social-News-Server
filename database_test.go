package main

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
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
		if a.id != b.id {
			t.Errorf("Mismatched id %d != %d", a.id, b.id)
		}
		if a.name != b.name {
			t.Errorf("Mismatched name %s != %s", a.name, b.name)
		}
		if a.email != b.email {
			t.Errorf("Mismatched email %s != %s", a.email, b.email)
		}
		if a.password != b.password {
			t.Errorf("Mismatched password %s != %s", a.password, b.password)
		}
		if a.registerDate != b.registerDate {
			t.Errorf("Mismatched registerDate %v != %v", a.registerDate, b.registerDate)
		}
	}

	rand.Seed(63487)
	users := []User{}
	for _, tc := range usersTC {
		user := DBCreateUser(db, tc.name, tc.email, tc.password)
		users = append(users, user)
	}
	for _, source := range users {
		t.Run(source.name, func(t *testing.T) {
			// t.Logf("%v\n", source)
			user1 := DBGetUser(db, source.id, "")
			matchUsers("id matched user", source, user1)
			user2 := DBGetUser(db, 0, source.email)
			matchUsers("email matched user", source, user2)

			newPassword := users[rand.Intn(len(users))].password
			DBUpdatePasswordForUser(db, source.id, source.password, newPassword)
			user3 := DBGetUser(db, source.id, "")
			if user3.password != newPassword {
				t.Errorf("not matching password (%s, %s) after update", user3.password, newPassword)
			}

			otherUser := users[rand.Intn(len(users))]
			user4 := DBGetUser(db, 0, otherUser.email)
			amount := int64(rand.Intn(50)) * sign(rand.Intn(2) == 0)
			srcUser, srcPref := DBVoteForUser(db, source.id, user4.id, amount)

			user5 := DBGetUser(db, user4.id, "")
			if amount < 0 && user5.downvotes < 0 {
				t.Errorf("Failed to update user total downvotes")
			} else if amount > 0 && user5.upvotes < 0 {
				t.Errorf("Failed to update user total upvotes")
			}
			matchUsers("match vote and get user", srcUser, user5)

			userPrefs := DBGetUserPref(db, source.id, soScore, upUser, user5.id, 0, 10, 0)
			if len(userPrefs) != 1 {
				t.Errorf("failed to get user prefs for user %d", source.id)
			} else {
				p := userPrefs[0]
				if amount < 0 && p.downvotes < float64(amount) {
					t.Errorf("failed to update user pref downvotes for other user")
				} else if amount > 0 && p.upvotes < float64(amount) {
					t.Errorf("failed to update user pref upvotes for other user")
				}
				if p.downvotes != srcPref.downvotes || p.upvotes != srcPref.upvotes ||
					p.kind != srcPref.kind || p.pid != srcPref.pid || p.sid != srcPref.sid {
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

	matchPost := func(a Post, b Post) {
		if a.id != b.id {
			t.Errorf("Mismatch id %d != %d", a.id, b.id)
		}
		if a.content != b.content {
			t.Errorf("Mismatch content %s != %s", a.content, b.content)
		}
		if a.userId != b.userId {
			t.Errorf("Mismatch userId %d != %d", a.userId, b.userId)
		}
		// if a.tags != b.tags {
		// 	t.Errorf("Mismatch content %v != %v", a.content, b.content)
		// }
		// if a.location != b.location {
		// 	t.Errorf("Mismatch id %d != %d", a.id, b.id)
		// }
		if a.createdAt != b.createdAt {
			t.Errorf("Mismatch createdAt %v != %v", a.createdAt, b.createdAt)
		}
		if a.updatedAt != b.updatedAt {
			t.Errorf("Mismatch updatedAt %v != %v", a.updatedAt, b.updatedAt)
		}
		if a.upvotes != b.upvotes {
			t.Errorf("Mismatch upvotes %f != %f", a.upvotes, b.upvotes)
		}
		if a.downvotes != b.downvotes {
			t.Errorf("Mismatch downvotes %f != %f", a.downvotes, b.downvotes)
		}
	}

	startOfYear := time.Date(utc().Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
	endOfYear := time.Date(utc().Year(), time.December, 31, 23, 59, 59, 999999, time.UTC)
	for _, tc := range posts {
		t.Run(tc.content, func(t *testing.T) {
			source := DBCreatePost(db, tc.userId, tc.content, tc.tags, tc.location)
			post1 := DBGetPosts(db, 0, source.id, []string{}, []string{}, soScore, 1, 0, formatTime(startOfYear), formatTime(endOfYear))
			matchPost(source, post1[0])

			odx := rand.Intn((len(posts)))
			otherUser := users[odx]
			other := DBGetUser(db, 0, otherUser.email)
			loc := posts[odx].location
			amount := int64(rand.Intn(50)) * sign(rand.Intn(2) == 0)
			post2, userPoster, tags, prefs := DBVotePost(db, other.id, source.id, amount, loc)

			if amount < 0 && post2.downvotes < float64(amount) {
				t.Errorf("Post downvotes not updated")
			} else if amount > 0 && post2.upvotes < float64(amount) {
				t.Errorf("Post upvotes not updated")
			}
			if amount < 0 && userPoster.downvotes < float64(amount) {
				t.Errorf("Post downvotes for poster not updated")
			} else if amount > 0 && userPoster.upvotes < float64(amount) {
				t.Errorf("Post upvotes for poster not updated")
			}
			uPref := prefs[0]
			if uPref.kind != upUser && uPref.pid != other.id {
				t.Errorf("User pref not update user voted for")
			}
			if amount < 0 && uPref.downvotes < float64(amount) {
				t.Errorf("User pref downvotes for poster not updated")
			} else if amount > 0 && uPref.upvotes < float64(amount) {
				t.Errorf("User pref upvotes for poster not updated")
			}
			pPref := prefs[1]
			if pPref.kind != upPost && pPref.pid != source.id {
				t.Errorf("User pref not update user voted for")
			}
			if amount < 0 && pPref.downvotes < float64(amount) {
				t.Errorf("User pref downvotes for post not updated")
			} else if amount > 0 && pPref.upvotes < float64(amount) {
				t.Errorf("User pref upvotes for post not updated")
			}

			for i, tag := range tags {
				if tc.tags[i] != tag.name {
					t.Errorf("Mismatch of tag names")
				}
			}
			for i := 2; i < len(prefs); i += 1 {
				fmt.Printf("%d : %v\n", i-2, tags[i-2])
				if tags[i-2].id != prefs[i].pid && prefs[i].kind != upTag {
					t.Errorf("Mismatch of tag id and user pref id")
				}
			}
		})
	}
}
