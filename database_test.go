package main

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	mainDB = getTestDatabase()
	defer mainDB.Close()
	DBClearAllTables(mainDB)
	DBSetup(mainDB)

	// agents = NAReadAllNewsAgents(mainDB)
	// agentsOnboarding = NAReadAllAgentsOnboarding()
	// tagsOnboarding = NAReadAllTagsOnboarding(mainDB)

	exitCode := m.Run()

	os.Exit(exitCode)
}

func TestDatabase(t *testing.T) {
	mainDB = getTestDatabase()
	defer mainDB.Close()
	DBClearAllTables(mainDB)
	DBSetup(mainDB)

	usersTC := []struct {
		name     string
		email    string
		password string
	}{
		{"rb t", "ribet@new-source.app", "123456"},
		{"ako", "akoblin@new-source.app", "password"},
		{"ps", "portscan@new-source.app", "1234"},
		{"noutlook", "mhanoh@new-source.app", "000"},
		{"monsolo", "solomon@new-source.app", "00000"},
		{"com grady", "grady@new-source.app", "p15423"},
		{"mac wag", "wagnerch@new-source.app", "fniweufjk"},
		{"jmail", "jandrese@new-source.app", "fcn5893gq8op%&"},
		{"barnot", "barnett@new-source.app", "fjijfiejfiej"},
		{"yahear", "greear@new-source.app", "geer"},
		{"toku", "tokuhirom@new-source.app", "pass"},
		{"fat elk", "fatelk@new-source.app", "word"},
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
		user := DBCreateUser(mainDB, tc.name, tc.email, tc.password)
		DBValidateUser(mainDB, user.ID, user.ValidationKey)
		if !DBEqualHashAndPassword(user.Password, tc.password) {
			t.Errorf("user password hash failed %s != %s", user.Password, tc.password)
		}
		users = append(users, user)
	}
	for i, source := range users {
		t.Run(source.Name, func(t *testing.T) {
			// t.Logf("%v\n", source)
			rand.Seed(int64(i))
			user1, _ := DBGetUser(mainDB, source.ID, "")
			matchUsers("id matched user", source, user1)
			user2, _ := DBGetUser(mainDB, 0, source.Email)
			matchUsers("email matched user", source, user2)

			if user1.ValidationKey != 0 {
				t.Errorf("user validation failed")
			}

			newPassword := usersTC[rand.Intn(len(usersTC))].password
			DBUpdatePasswordForUser(mainDB, source.ID, usersTC[i].password, newPassword)
			user3, _ := DBGetUser(mainDB, source.ID, "")
			if !DBEqualHashAndPassword(user3.Password, newPassword) {
				t.Errorf("not matching password (%s, %s) after update", user3.Password, newPassword)
			}

			otherUser := users[rand.Intn(len(users))]
			user4, _ := DBGetUser(mainDB, 0, otherUser.Email)
			amount := int64(rand.Intn(50)) * sign(rand.Intn(2) == 0)
			srcUser, srcPref := DBVoteForUser(mainDB, source.ID, user4.ID, amount, []string{})

			user5, _ := DBGetUser(mainDB, user4.ID, "")
			if amount < 0 && user5.Downvotes < 0 {
				t.Errorf("Failed to update user total downvotes")
			} else if amount > 0 && user5.Upvotes < 0 {
				t.Errorf("Failed to update user total upvotes")
			}
			if source.ID == user4.ID {
				if srcUser.ID != 0 {
					t.Errorf("User voted for self should not return")
				}
			} else {
				matchUserProfile("match vote and get user", srcUser, user5)
				userPrefs := DBGetUserPref(mainDB, true, source.ID, soScore, upUser, user5.ID, 0, 0, 0, 10, 0)
				if len(userPrefs) != 1 {
					t.Errorf("failed to get user prefs for user %d", source.ID)
				} else {
					p := userPrefs[0]
					if amount < 0 && p.Downvotes < amount {
						t.Errorf("failed to update user pref downvotes for other user")
					} else if amount > 0 && p.Upvotes < amount {
						t.Errorf("failed to update user pref upvotes for other user")
					}
					if p.Downvotes != srcPref.Downvotes || p.Upvotes != srcPref.Upvotes ||
						p.Kind != srcPref.Kind || p.PID != srcPref.PID || p.SID != srcPref.SID {
						t.Errorf("mismatch between get user pref and vote user pref")
					}
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

	matchPost := func(a PostResult, b PostResult) {
		if a.ID != b.ID {
			t.Errorf("Mismatch id %d != %d", a.ID, b.ID)
		}
		if a.Content != b.Content {
			t.Errorf("Mismatch content %s != %s", a.Content, b.Content)
		}
		if a.Author.ID != b.Author.ID {
			t.Errorf("Mismatch userId %d != %d", a.Author.ID, b.Author.ID)
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
			t.Errorf("Mismatch upvotes %d != %d", a.Upvotes, b.Upvotes)
		}
		if a.Downvotes != b.Downvotes {
			t.Errorf("Mismatch downvotes %d != %d", a.Downvotes, b.Downvotes)
		}
	}

	// startOfYear := time.Date(utc().Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
	// endOfYear := time.Date(utc().Year(), time.December, 31, 23, 59, 59, 999999, time.UTC)
	for i, tc := range posts {
		t.Run(tc.content, func(t *testing.T) {
			rand.Seed(int64(i))
			source := DBCreatePost(mainDB, tc.userId, tc.content, time.Time{}, tc.tags, tc.location, "en")
			post1 := DBGetPost(mainDB, source.ID)
			matchPost(source, post1)
			matchUserProfile("", post1.Author, users[tc.userId-1])

			for i := 0; i < rand.Intn(10); i += 1 {
				odx := rand.Intn(len(users))
				otherUser := users[odx]
				other, _ := DBGetUser(mainDB, 0, otherUser.Email)
				pidx := rand.Intn(len(posts))

				comt, cont := DBCreateComment(mainDB, other.ID, posts[pidx].content, post1.ID, 0, false, "")

				if comt.Content != posts[pidx].content {
					t.Errorf("comment content not correct")
				}
				if comt.UserID != other.ID {
					t.Errorf("comment user id not correct")
				}
				if cont.sid != comt.ID {
					t.Errorf("comment id not correct on user cont")
				}
				if cont.pid != comt.PostID {
					t.Errorf("comment post id not correct on user cont")
				}
			}

			flagCountIteration := 0
			for i := 0; i < rand.Intn(3); i += 1 {
				odx := rand.Intn(len(users))
				otherUser := users[odx]
				other, _ := DBGetUser(mainDB, 0, otherUser.Email)
				flagCountIteration += 1
				DBCreateFlag(mainDB, other.ID, post1.ID, -1, frSpam, "no reason")
			}
			flagPost := DBGetPost(mainDB, post1.ID)
			if flagPost.FlagCount != int64(flagCountIteration) {
				t.Errorf("flag count not matching")
			}
			DBIgnoreFlagContent(mainDB, post1.ID, -1)
			flagPost = DBGetPost(mainDB, post1.ID)
			if flagPost.FlagCount != -1000 {
				t.Errorf("flag count not matching after handling")
			}

			for i := 0; i < rand.Intn(50); i += 1 {
				odx := rand.Intn(len(users))
				otherUser := users[odx]
				other, _ := DBGetUser(mainDB, 0, otherUser.Email)
				pidx := rand.Intn(len(posts))
				loc := posts[pidx].location
				amount := int64(rand.Intn(50)) * sign(rand.Intn(2) == 0)
				post2, userPoster, tags, prefs := DBVotePost(mainDB, other.ID, source.ID, amount, loc)

				if amount < 0 && post2.Downvotes < amount {
					t.Errorf("Post downvotes not updated")
				} else if amount > 0 && post2.Upvotes < amount {
					t.Errorf("Post upvotes not updated")
				}
				if other.ID != source.Author.ID {
					if amount < 0 && userPoster.Downvotes < amount {
						t.Errorf("Post downvotes for poster not updated")
					} else if amount > 0 && userPoster.Upvotes < amount {
						t.Errorf("Post upvotes for poster not updated")
					}
					uPref := prefs[0]
					if uPref.Kind != upUser && uPref.PID != other.ID {
						t.Errorf("User pref not update user voted for")
					}
					if amount < 0 && uPref.Downvotes < amount {
						t.Errorf("User pref downvotes for poster not updated")
					} else if amount > 0 && uPref.Upvotes < amount {
						t.Errorf("User pref upvotes for poster not updated")
					}
				}
				pPref := prefs[1]
				if pPref.Kind != upPost && pPref.PID != source.ID {
					t.Errorf("User pref not update user voted for")
				}
				if amount < 0 && pPref.Downvotes < amount {
					t.Errorf("User pref downvotes for post not updated")
				} else if amount > 0 && pPref.Upvotes < amount {
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

func FuzzDatabase(f *testing.F) {
	mainDB := getTestDatabase()

	usersTC := []struct {
		name     string
		email    string
		password string
	}{
		{"rb t", "ribet1@new-source.app", "123456"},
		{"ako", "akoblin1@new-source.app", "password"},
		{"ps", "portscan1@new-source.app", "1234"},
		{"noutlook", "mhanoh1@new-source.app", "000"},
		{"monsolo", "solomon1@new-source.app", "00000"},
		{"com grady", "grady1@new-source.app", "p15423"},
		{"mac wag", "wagnerch1@new-source.app", "fniweufjk"},
		{"jmail", "jandrese1@new-source.app", "fcn5893gq8op%&"},
		{"barnot", "barnett1@new-source.app", "fjijfiejfiej"},
		{"yahear", "greear1@new-source.app", "geer"},
		{"toku", "tokuhirom1@new-source.app", "pass"},
		{"fat elk", "fatelk1@new-source.app", "word"},
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
		{"Whats your go-to radio station to listen or stream", 5, []string{"Music", "Stream", "radio"}, []string{"Africa", "South Africa", "Free State", "Drakensberg"}},
		{"Gareth Bale launches lager and ale", 3, []string{"Bale", "Lager"}, []string{"South America", "Brazil", "Sao Paolo", "1888"}},
		{"I know why the caged archon sings", 4, []string{"Genshin", "Nahida"}, []string{"Africa", "Egypt", "Giza", "3"}},
		{"central berg in spring is something else", 5, []string{"Drakensberg", "Spring"}, []string{"Africa", "South Africa", "Free State", "Drakensberg"}},
	}

	for i, u := range usersTC {
		f.Add(u.name, u.email, u.password, posts[i].content)
	}
	wg := new(sync.WaitGroup)
	f.Fuzz(func(t *testing.T, n string, e string, p string, c string) {
		user := DBCreateUser(mainDB, n, e, p)
		if user.ID != 0 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				post := DBCreatePost(mainDB, user.ID, c, utc(), []string{}, []string{}, "en")
				go DBVotePost(mainDB, user.ID, post.ID, 1, []string{})
			}()
		}
	})
	wg.Wait()
	mainDB.Close()
}

func TestUserFlow(t *testing.T) {
	mainDB = getTestDatabase()
	defer mainDB.Close()
	DBClearAllTables(mainDB)
	DBSetup(mainDB)
	r := getTestRouter()

	user := CallAPI(r, "POST", "/api/v1/users", gin.H{
		"name":     "patient x",
		"email":    "patientX@new-source.app",
		"password": "h1z1init",
		"device":   "blankStare",
	})
	if user.Get("payload", "user", "Password") != DBHashPassword("h1z1init") {
		t.Errorf("password hash not matching")
	}
	if user.Get("payload", "user", "Name") != "patient x" {
		t.Errorf("incorrect name")
	}
	device := "blankStare"

	user = CallAPI(r, "POST", "/api/v1/auth/sign-in", gin.H{
		"email":    "patientX@new-source.app",
		"password": "h1z1init",
		"device":   "blankStare",
	})
	if user.Get("payload", "user", "Name") != "patient x" {
		t.Errorf("sign in incorrect")
	}
	user = CallAPI(r, "POST", "/api/v1/auth/sign-in", gin.H{
		"email":    "patientX@new-source",
		"password": "h1z1init",
		"device":   "blankStare",
	})
	if user.Get("reason") != "missing" {
		t.Errorf("expected missing user with wrong email")
	}
	user = CallAPI(r, "POST", "/api/v1/auth/sign-in", gin.H{
		"email":    "patientX@new-source.app",
		"password": "h1z1",
		"device":   "blankStare",
	})
	if user.Get("reason") != "password" {
		t.Errorf("expected incorrect password")
	}

	signOut := CallAPI(r, "POST", "/api/v1/auth/sign-out", gin.H{
		"userId": 1,
		"device": device,
	})
	if signOut.Get("success").(bool) != true {
		t.Errorf("expected sign out")
	}
}

func TestCreation(t *testing.T) {
	mainDB = getTestDatabase()
	defer mainDB.Close()
	DBClearAllTables(mainDB)
	DBSetup(mainDB)
	r := getTestRouter()

	user := CallAPI(r, "POST", "/api/v1/users", gin.H{
		"name":     "patient x",
		"email":    "patientX@new-source.app",
		"password": "h1z1init",
		"device":   "blankStare",
	})
	validationKey := user.Get("payload", "user", "ValidationKey").(string)
	secret := user.Get("payload", "token").(string)
	device := "blankStare"

	pid, _ := strconv.ParseInt(user.Get("payload", "user", "ID").(string), 10, 64)
	posts := CallAPI(r, "GET", fmt.Sprintf("/api/v1/posts?uid=%d", pid), gin.H{})
	if len(posts.Get("payload").([]interface{})) != 0 {
		t.Errorf("posts from new user must be empty")
	}

	CallAPI(
		r, "GET",
		fmt.Sprintf("/api/v1/users/1/verification/%s", validationKey),
		gin.H{},
	)

	post := CallAPI(r, "POST", fmt.Sprintf("/api/v1/posts?secret=%s&device=%s", secret, device), gin.H{
		"userId":    "1",
		"content":   "this is a post",
		"location":  []string{"ZA", "Western Cape"},
		"isPreview": false,
		"locale":    "en",
	})
	if post.Get("payload", "ID").(string) != "1" {
		t.Errorf("expected to create post, but found: %#v", post)
	}

	posts = CallAPI(r, "GET", fmt.Sprintf("/api/v1/posts?uid=%d", pid), gin.H{})
	if len(posts.Get("payload").([]interface{})) != 1 {
		t.Errorf("posts must contain a single post")
	}
}
