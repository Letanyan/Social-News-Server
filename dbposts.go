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

func DBCreatePost(db *sql.DB, userId int64, content string, tags []string, location []string) Post {
	t := utc()
	nowTime := formatNow()
	year := t.Year()
	postId := idFromTime(t)
	insertPost := fmt.Sprintf(`INSERT INTO posts%d(id, userId, content, tags, createdAt, updatedAt, location) 
	VALUES (%d, $1, $2, %s, '%s', '%s', %s) RETURNING id, userId, content, tags, createdAt, updatedAt, location, upvotes, downvotes`, year, postId, SQLFormattedArray(tags), nowTime, nowTime, SQLFormattedArray(location))
	row := db.QueryRow(insertPost, userId, content)

	var id int64
	var createdAt time.Time
	var updatedAt time.Time
	var up float64
	var down float64
	e := row.Scan(&id, &userId, &content, pq.Array(&tags), &createdAt, &updatedAt, pq.Array(&location), &up, &down)
	if DidFail(e, "create post") {
		return Post{}
	}

	insertPostForUser := fmt.Sprintf(`INSERT INTO User%dCont(postId, commentId) VALUES(%d, -1)`, userId, postId)
	_, e = db.Exec(insertPostForUser)
	if DidFail(e, "insert post to user") {
		return Post{}
	}

	createPostTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS Post%d (
		id BIGSERIAL NOT NULL,
		userId BIGINT,
		replyId BIGINT,
		content text,
		upvotes DOUBLE PRECISION DEFAULT 0.0,
		downvotes DOUBLE PRECISION DEFAULT 0.0,
		createdAt timestamp,
		updatedAt timestamp,

		PRIMARY KEY (id)
	);`, postId)

	_, e = db.Exec(createPostTable)
	if DidFail(e, "create post table for user ", postId) {
		return Post{}
	}

	DBCreateTags(db, tags, location)

	return Post{id, userId, content, tags, createdAt, updatedAt, location, up, down}
}

func DBVotePost(db *sql.DB, userId int64, postId int64, upvoteAmount int64, location []string) (Post, User, []Tag, []UserPref) {
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
		RETURNING id, userId, content, tags, createdAt, updatedAt, location, upvotes, downvotes
		`, year, updateField, updateField, nowTime, upvoteAmount, otherField, otherField, nowTime, nowTime, postId)
	rows, e := db.Query(updateVoteForPost)
	if DidFail(e, "vote for post ", postId) {
		return Post{}, User{}, []Tag{}, []UserPref{}
	}
	var posterId int64
	var content string
	var tags []string
	var createdAt time.Time
	var updatedAt time.Time
	var loc []string
	var up float64
	var down float64
	for rows.Next() {
		e = rows.Scan(&postId, &posterId, &content, pq.Array(&tags), &createdAt, &updatedAt, pq.Array(&loc), &up, &down)
		if DidFail(e, "scan upvote and downvote for post ", postId) {
			continue
		}
	}

	tagResult, tagPrefs := DBVoteTags(db, userId, tags, isUpvote, location)
	user, userPref := DBVoteForUser(db, userId, posterId, upvoteAmount*sign(isUpvote))
	createPref := fmt.Sprintf(`
	INSERT INTO User%dPref (kind, pid, sid) 
	VALUES(3, %d, -1) ON CONFLICT (kind, pid, sid) DO NOTHING;
	UPDATE User%dPref
	SET %s = cooldown(%s, updatedAt, '%s', 31536000) + %d,
	%s = cooldown(%s, updatedAt, '%s', 31536000)
	WHERE kind=3 AND pid=%d
	RETURNING kind, pid, sid, upvotes, downvotes
	`, userId, postId, userId, updateField, updateField, nowTime, upvoteAmount, otherField, otherField, nowTime, postId)
	row := db.QueryRow(createPref)
	userPrefForPost, e := ScanUserPrefRow(row, false)
	if DidFail(e, "vote for post ", postId) {
		return Post{}, User{}, []Tag{}, []UserPref{}
	}

	post := Post{postId, posterId, content, tags, createdAt, updatedAt, loc, up, down}
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

	deletePostTable := fmt.Sprintf(`DROP TABLE Post%d`, postId)
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

func DBGetPost(db *sql.DB, id int64) Post {
	getPosts := fmt.Sprintf(`SELECT id, userId, content, tags, location, createdAt, updatedAt, upvotes, downvotes, RATIO(upvotes, downvotes) AS cred, upvotes * RATIO(upvotes, downvotes) AS score 
	FROM posts%d WHERE id = $1
	`, yearFromId(id))

	row := db.QueryRow(getPosts, id)
	var userId int64
	var content string
	var tags []string
	var loc []string
	var createdAt time.Time
	var updatedAt time.Time
	var upvotes float64
	var downvotes float64
	var cred float64
	var score float64

	e := row.Scan(&id, &userId, &content, pq.Array(&tags), pq.Array(&loc), &createdAt, &updatedAt, &upvotes, &downvotes, &cred, &score)
	if DidFail(e, "read row") {
		return Post{}
	}

	post := Post{id, userId, content, tags, createdAt, updatedAt, loc, upvotes, downvotes}
	return post
}

// ignore userId if equals 0. ignore id if equals 0. ignore tags if empty. ignore location if empty.
func DBGetPosts(db *sql.DB, userId int64, tags []string, location []string, upvotes int64, downvotes int64, sortOrder SortOrder, limit int64, offset int64, startDate string, endDate string) []Post {
	getPosts := fmt.Sprintf(`SELECT id, userId, content, tags, location, createdAt, updatedAt, upvotes, downvotes, RATIO(upvotes, downvotes) AS cred, upvotes * RATIO(upvotes, downvotes) AS score 
		FROM posts{} 
		WHERE createdAt BETWEEN (TIMESTAMP '%s') AND (TIMESTAMP '%s')
		`, startDate, endDate)
	if userId != 0 {
		getPosts += fmt.Sprintf("AND userId = %d\n", userId)
	}
	if len(tags) > 0 {
		queryTags := SQLFormattedArray(tags)
		getPosts += fmt.Sprintf("AND %s && tags\n", queryTags)
	}
	if len(location) > 0 {
		queryLoc := SQLFormattedArray(location)
		getPosts += fmt.Sprintf("AND location @> %s\n", queryLoc)
	}
	if upvotes != 0 {
		if upvotes > 0 {
			getPosts += fmt.Sprintf("AND upvotes > %d\n", upvotes)
		} else {
			getPosts += fmt.Sprintf("AND upvotes < %d\n", -upvotes)
		}
	}
	if downvotes != 0 {
		if upvotes > 0 {
			getPosts += fmt.Sprintf("AND downvotes > %d\n", upvotes)
		} else {
			getPosts += fmt.Sprintf("AND downvotes < %d\n", -upvotes)
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

	result := []Post{}
	for rows.Next() {
		var id int64
		var userId int64
		var content string
		var tags []string
		var loc []string
		var createdAt time.Time
		var updatedAt time.Time
		var upvotes float64
		var downvotes float64
		var cred float64
		var score float64

		e = rows.Scan(&id, &userId, &content, pq.Array(&tags), pq.Array(&loc), &createdAt, &updatedAt, &upvotes, &downvotes, &cred, &score)
		if DidFail(e, "read row") {
			continue
		}
		post := Post{id, userId, content, tags, createdAt, updatedAt, loc, upvotes, downvotes}
		result = append(result, post)
	}

	return result
}
