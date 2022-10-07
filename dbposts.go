package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

type Post struct {
	ID        int64
	UserID    int64
	Content   string
	Tags      []string
	CreatedAt time.Time
	UpdatedAt time.Time
	Location  []string
	Upvotes   float64
	Downvotes float64
}

type PostResult struct {
	ID        int64
	Author    UserProfile
	Content   string
	Tags      []string
	CreatedAt time.Time
	UpdatedAt time.Time
	Location  []string
	Upvotes   float64
	Downvotes float64
}

func SQLFieldsForPost() string {
	return "id, userId, content, tags, createdAt, updatedAt, location, upvotes, downvotes"
}

func SQLFieldsForPostResult() string {
	return "p.id, p.userId, p.content, p.tags, p.createdAt, p.updatedAt, p.location, p.upvotes, p.downvotes, u.id, u.name, u.registerDate, u.upvotes, u.downvotes"
}

func ScanPost(row *sql.Row) (Post, error) {
	p := Post{}
	e := row.Scan(&p.ID, &p.UserID, &p.Content, pq.Array(&p.Tags), &p.CreatedAt, &p.UpdatedAt, pq.Array(&p.Location), &p.Upvotes, &p.Downvotes)
	return p, e
}

func ScanPostResult(row *sql.Row) (PostResult, error) {
	p := PostResult{}
	u := UserProfile{}
	var userID int64
	e := row.Scan(&p.ID, &userID, &p.Content, pq.Array(&p.Tags), &p.CreatedAt, &p.UpdatedAt, pq.Array(&p.Location), &p.Upvotes,
		&p.Downvotes, &u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Upvotes)
	p.Author = u
	return p, e
}

func ScanPosts(rows *sql.Rows) []Post {
	result := []Post{}
	var e error

	for rows.Next() {
		p := Post{}
		var score float64
		var cred float64
		e = rows.Scan(&p.ID, &p.UserID, &p.Content, pq.Array(&p.Tags), &p.CreatedAt, &p.UpdatedAt, pq.Array(&p.Location), &p.Upvotes, &p.Downvotes, &cred, &score)
		if DidFail(e, "scan post") {
			continue
		}
		result = append(result, p)
	}

	return result
}

func ScanPostResults(rows *sql.Rows) []PostResult {
	result := []PostResult{}
	var e error

	for rows.Next() {
		p := PostResult{}
		u := UserProfile{}
		var score float64
		var cred float64
		var userID int64
		e = rows.Scan(&p.ID, &userID, &p.Content, pq.Array(&p.Tags), &p.CreatedAt, &p.UpdatedAt, pq.Array(&p.Location), &p.Upvotes,
			&p.Downvotes, &u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Upvotes, &cred, &score)
		if DidFail(e, "scan post") {
			continue
		}
		p.Author = u
		result = append(result, p)
	}

	return result
}

func DBCreatePost(db *sql.DB, userId int64, content string, tags []string, location []string) Post {
	t := utc()
	nowTime := formatNow()
	year := t.Year()
	insertPost := fmt.Sprintf(`INSERT INTO posts%d(id, userId, content, tags, createdAt, updatedAt, location) 
	VALUES (nextval('posts%d_id_seq') * 10000 + extract(year from now() at time zone ('utc')), $1, $2, %s, '%s', '%s', %s) RETURNING %s`, year, year, SQLFormattedArray(tags), nowTime, nowTime, SQLFormattedArray(location), SQLFieldsForPost())
	row := db.QueryRow(insertPost, userId, content)

	post, e := ScanPost(row)
	if DidFail(e, "create post") {
		return Post{}
	}

	insertPostForUser := fmt.Sprintf(`INSERT INTO User%dCont(postId, commentId) VALUES(%d, -1)`, post.UserID, post.ID)
	_, e = db.Exec(insertPostForUser)
	if DidFail(e, "insert post to user") {
		return Post{}
	}

	DBCreateTags(db, tags, location)

	return post
}

func DBVotePost(db *sql.DB, userId int64, postId int64, upvoteAmount int64, location []string) (Post, UserProfile, []Tag, []UserPref) {
	currentTime := time.Now().UTC()
	nowTime := formatTime(currentTime)
	var updateField string
	var otherField string
	isUpvote := upvoteAmount > 0
	if isUpvote {
		updateField = "upvotes"
		otherField = "downvotes"
	} else {
		updateField = "downvotes"
		otherField = "upvotes"
		upvoteAmount = -upvoteAmount
	}
	year := yearFromId(postId)
	updateVoteForPost := fmt.Sprintf(`
		UPDATE posts%d 
		SET %s = cooldown(%s, updatedAt, '%s', 31536000) + %d,
		%s = cooldown(%s, updatedAt, '%s', 31536000),
		updatedAt = '%s'
		WHERE id = %d
		RETURNING %s
		`, year, updateField, updateField, nowTime, upvoteAmount, otherField, otherField, nowTime, nowTime, postId, SQLFieldsForPost())
	row := db.QueryRow(updateVoteForPost)
	post, e := ScanPost(row)
	if DidFail(e, "vote for post ", postId) {
		return Post{}, UserProfile{}, []Tag{}, []UserPref{}
	}

	tagResult, tagPrefs := DBVoteTags(db, userId, post.Tags, upvoteAmount, location)
	user, userPref := DBVoteForUser(db, userId, post.UserID, upvoteAmount*sign(isUpvote))
	createPref := fmt.Sprintf(`
	INSERT INTO User%dPref (kind, pid, sid) 
	VALUES(3, %d, -1) ON CONFLICT (kind, pid, sid) DO NOTHING;
	UPDATE User%dPref
	SET %s = cooldown(%s, updatedAt, '%s', 31536000) + %d,
	%s = cooldown(%s, updatedAt, '%s', 31536000)
	WHERE kind=3 AND pid=%d
	RETURNING %s
	`, userId, postId, userId, updateField, updateField, nowTime, upvoteAmount, otherField, otherField, nowTime, postId, SQLFieldsForUserPref())
	row = db.QueryRow(createPref)
	userPrefForPost, e := ScanUserPrefRow(row)
	if DidFail(e, "vote for post ", postId) {
		return Post{}, UserProfile{}, []Tag{}, []UserPref{}
	}

	prefs := []UserPref{userPref, userPrefForPost}
	prefs = append(prefs, tagPrefs...)

	return post, user, tagResult, prefs
}

func DBDeletePost(db *sql.DB, postId int64) {
	year := yearFromId(postId)

	deleteFromPosts := fmt.Sprintf(`DELETE FROM posts%d WHERE id=$1 RETURNING userId`, year)
	row := db.QueryRow(deleteFromPosts, postId)
	var userId int64
	e := row.Scan(&userId)
	if !DidFail(e, "get userId for deleting post ", postId) {
		deletePostFromUser := fmt.Sprintf(`DELETE FROM User%dCont WHERE postId=$2`, userId)
		_, e = db.Exec(deletePostFromUser, postId)
		DidFail(e, "delete post ", postId, " for user ", userId)
	}
	DidFail(e, "delete post from posts table")

	deletePostTable := fmt.Sprintf(`DELETE FROM Comments WHERE postId = %d`, postId)
	_, e = db.Exec(deletePostTable)
	DidFail(e, "delete post table")
}

type SortOrder int

const (
	soScore SortOrder = iota + 1
	soCred
	soUpvotes
	soDownvotes
	soControversial
	soCreatedAt
)

func SortOrderFromString(text string) (SortOrder, error) {
	switch strings.ToLower(text) {
	case "score":
		return soScore, nil
	case "cred":
		return soCred, nil
	case "upvotes":
		return soUpvotes, nil
	case "downvotes":
		return soDownvotes, nil
	case "controversial":
		return soControversial, nil
	case "createdat":
		return soCreatedAt, nil
	}
	return soUpvotes, errors.New("no known order for " + text)
}

func SQLSortOrder(so SortOrder) string {
	switch so {
	case soScore:
		return "ORDER BY score DESC\n"
	case soCred:
		return "ORDER BY score DESC\n"
	case soUpvotes:
		return "ORDER BY score DESC\n"
	case soDownvotes:
		return "ORDER BY score DESC\n"
	case soControversial:
		return "ORDER BY COALESCE(1 / NULLIF(ABS(cred - 0.5), 0), 9e90) DESC\n"
	case soCreatedAt:
		return "ORDER BY createdAt DESC\n"
	}
	return ""
}

func DBGetPost(db *sql.DB, id int64) PostResult {
	getPosts := fmt.Sprintf(`SELECT %s
	FROM posts%d p JOIN users u ON p.userId = u.id  WHERE p.id = $1
	`, SQLFieldsForPostResult(), yearFromId(id))

	row := db.QueryRow(getPosts, id)
	post, e := ScanPostResult(row)
	if DidFail(e, "read row") {
		return PostResult{}
	}
	return post
}

// ignore userId if equals 0. ignore id if equals 0. ignore tags if empty. ignore location if empty.
func DBGetPosts(db *sql.DB, userId int64, tags []string, location []string, upvotes int64, downvotes int64, sortOrder SortOrder, limit int64, offset int64, startDate string, endDate string) []Post {
	getPosts := fmt.Sprintf(`SELECT %s, RATIO(upvotes, downvotes) AS cred, upvotes * RATIO(upvotes, downvotes) AS score 
		FROM posts{} p JOIN users u ON p.userId = u.id
		WHERE createdAt BETWEEN (TIMESTAMP '%s') AND (TIMESTAMP '%s')
		`, SQLFieldsForPostResult(), startDate, endDate)
	if userId != 0 {
		getPosts += fmt.Sprintf("AND p.userId = %d\n", userId)
	}
	if len(tags) > 0 {
		queryTags := SQLFormattedArray(tags)
		getPosts += fmt.Sprintf("AND %s && p.tags\n", queryTags)
	}
	if len(location) > 0 {
		queryLoc := SQLFormattedArray(location)
		getPosts += fmt.Sprintf("AND p.location @> %s\n", queryLoc)
	}
	if upvotes != 0 {
		if upvotes > 0 {
			getPosts += fmt.Sprintf("AND p.upvotes > %d\n", upvotes)
		} else {
			getPosts += fmt.Sprintf("AND p.upvotes < %d\n", -upvotes)
		}
	}
	if downvotes != 0 {
		if upvotes > 0 {
			getPosts += fmt.Sprintf("AND p.downvotes > %d\n", downvotes)
		} else {
			getPosts += fmt.Sprintf("AND p.downvotes < %d\n", -downvotes)
		}
	}

	sDate := parseTime(startDate)
	eDate := parseTime(endDate)
	years := yearsBetweenDates(sDate, eDate)
	postQueries := BuildUnionForYears(getPosts, years)

	postQueries += SQLSortOrder(sortOrder)

	postQueries += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	rows, e := db.Query(postQueries)
	if DidFail(e, "get posts") {
		return []Post{}
	}
	defer rows.Close()

	result := ScanPosts(rows)
	return result
}
