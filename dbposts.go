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
	Tags      []int64
	CreatedAt time.Time
	Location  []string
	Upvotes   float64
	Downvotes float64
}

type PostResult struct {
	ID        int64
	Author    UserProfile
	Content   string
	Tags      []int64
	CreatedAt time.Time
	Location  []string
	Upvotes   float64
	Downvotes float64
}

func SQLFieldsForPost() string {
	return "id, userId, content, tags, createdAt, location, upvotes, downvotes"
}

func SQLFieldsForPostResult() string {
	return "p.id, p.userId, p.content, p.tags, p.createdAt, p.location, p.upvotes, p.downvotes, u.id, u.name, u.registerDate, u.upvotes, u.downvotes"
}

func SQLFieldsForPostResultAlias() string {
	return "p.id, p.userId, p.content, p.tags, p.createdAt, p.location, p.upvotes AS item_up, p.downvotes AS item_down, u.id, u.name, u.registerDate, u.upvotes, u.downvotes"
}

func ScanPost(row *sql.Row) (Post, error) {
	p := Post{}
	e := row.Scan(&p.ID, &p.UserID, &p.Content, pq.Array(&p.Tags), &p.CreatedAt, pq.Array(&p.Location), &p.Upvotes, &p.Downvotes)
	return p, e
}

func ScanPostResult(row *sql.Row) (PostResult, error) {
	p := PostResult{}
	u := UserProfile{}
	var userID int64
	e := row.Scan(&p.ID, &userID, &p.Content, pq.Array(&p.Tags), &p.CreatedAt, pq.Array(&p.Location), &p.Upvotes,
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
		e = rows.Scan(&p.ID, &p.UserID, &p.Content, pq.Array(&p.Tags), &p.CreatedAt, pq.Array(&p.Location), &p.Upvotes, &p.Downvotes, &cred, &score)
		if DidFail(e, "scan post") {
			continue
		}
		result = append(result, p)
	}

	return result
}

func ScanPostResults(rows *sql.Rows, hasVotes bool) []PostResult {
	result := []PostResult{}
	var e error

	for rows.Next() {
		p := PostResult{}
		u := UserProfile{}
		var score float64
		var cred float64
		var userID int64
		var up float64
		var down float64
		if hasVotes {
			e = rows.Scan(&p.ID, &userID, &p.Content, pq.Array(&p.Tags), &p.CreatedAt, pq.Array(&p.Location), &p.Upvotes,
				&p.Downvotes, &u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Upvotes, &up, &down, &cred, &score)
		} else {
			e = rows.Scan(&p.ID, &userID, &p.Content, pq.Array(&p.Tags), &p.CreatedAt, pq.Array(&p.Location), &p.Upvotes,
				&p.Downvotes, &u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Upvotes, &cred, &score)
		}
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
	nowTime := formatTime(t)

	tagObjects := DBCreateTags(db, tags)
	tagIndices := []int64{}
	for _, t := range tagObjects {
		tagIndices = append(tagIndices, t.ID)
	}

	// year := t.Year()
	insertPost := fmt.Sprintf(`
	INSERT INTO posts(userId, content, tags, createdAt, location) 
	VALUES ($1, $2, %s, '%s', %s) RETURNING %s`, SQLFormattedIndexArray(tagIndices), nowTime, SQLFormattedArray(location), SQLFieldsForPost())
	row := db.QueryRow(insertPost, userId, content)

	post, e := ScanPost(row)
	if DidFail(e, "create post") {
		return Post{}
	}

	insertPostForUser := fmt.Sprintf(`INSERT INTO UserCont(userId, postId, commentId) VALUES(%d, %d, -1)`, post.UserID, post.ID)
	_, e = db.Exec(insertPostForUser)
	if DidFail(e, "insert post to user") {
		return Post{}
	}

	return post
}

func DBVotePost(db *sql.DB, userId int64, postId int64, upvoteAmount int64, location []string, date string) (Post, UserProfile, []Tag, []UserPref) {
	var updateField string
	isUpvote := upvoteAmount > 0
	if isUpvote {
		updateField = "upvotes"
	} else {
		updateField = "downvotes"
		upvoteAmount = -upvoteAmount
	}

	voteQuery := SQLMakeVote(upPost, postId, -1, location, upvoteAmount*sign(isUpvote), date)
	updateVoteForPost := fmt.Sprintf(`
	%s
	UPDATE posts SET
	%s = %s + %d
	WHERE id = %d
	RETURNING %s
	`, voteQuery,
		updateField, updateField, upvoteAmount,
		postId, SQLFieldsForPost())

	row := db.QueryRow(updateVoteForPost)
	post, e := ScanPost(row)
	if DidFail(e, "vote for post ", postId) {
		return Post{}, UserProfile{}, []Tag{}, []UserPref{}
	}

	tagResult, tagPrefs := DBVoteTags(db, userId, post.Tags, upvoteAmount*sign(isUpvote), location, date)
	user, userPref := DBVoteForUser(db, userId, post.UserID, upvoteAmount*sign(isUpvote), location, date)
	userPrefForPost := DBCreateUserPref(db, userId, upPost, postId, -1, upvoteAmount*sign(isUpvote))

	prefs := []UserPref{userPref, userPrefForPost}
	prefs = append(prefs, tagPrefs...)

	return post, user, tagResult, prefs
}

func DBDeletePost(db *sql.DB, postId int64) {
	// year := yearFromId(postId)

	deleteFromPosts := `UPDATE posts SET thrashed=true WHERE id=$1 RETURNING userId`
	row := db.QueryRow(deleteFromPosts, postId)
	var userId int64
	e := row.Scan(&userId)
	if !DidFail(e, "get userId for deleting post ", postId) {
		deletePostFromUser := `UPDATE UserCont SET thrashed=true WHERE userId=$1 AND postId=$2`
		_, e = db.Exec(deletePostFromUser, userId, postId)
		DidFail(e, "delete post ", postId, " for user ", userId)
	}
	DidFail(e, "delete post from posts table")

	// deletePostTable := fmt.Sprintf(`DELETE FROM Comments WHERE postId = %d`, postId)
	// _, e = db.Exec(deletePostTable)
	// DidFail(e, "delete post table")
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
		return "ORDER BY cred DESC\n"
	case soUpvotes:
		return "ORDER BY item_up DESC\n"
	case soDownvotes:
		return "ORDER BY item_down DESC\n"
	case soControversial:
		return "ORDER BY COALESCE(1 / NULLIF(ABS(cred - 0.5), 0), 9e90) DESC\n"
	case soCreatedAt:
		return "ORDER BY createdAt DESC\n"
	}
	return ""
}

func DBGetPost(db *sql.DB, id int64) PostResult {
	getPosts := fmt.Sprintf(`SELECT %s
	FROM posts p JOIN users u ON p.userId = u.id  WHERE p.id = $1
	`, SQLFieldsForPostResult())

	row := db.QueryRow(getPosts, id)
	post, e := ScanPostResult(row)
	if DidFail(e, "read row") {
		return PostResult{}
	}
	return post
}

// ignore userId if equals 0. ignore id if equals 0. ignore tags if empty. ignore location if empty.
func DBGetPosts(db *sql.DB, userId int64, tags []string, origin []string, popularIn []string,
	upvotes int64, downvotes int64, sortOrder SortOrder, limit int64, offset int64,
	start string, end string, startDate string, endDate string, forUser int64) []PostResult {
	voteTable := "p"
	if len(popularIn) > 0 {
		voteTable = "v"
	}

	joins := "JOIN users u ON p.userId = u.id\n"
	cond := []string{}
	// cond = append(cond, "p.thrashed = false")
	if len(start) > 0 && len(end) > 0 {
		cond = append(cond, fmt.Sprintf("p.createdAt BETWEEN (TIMESTAMP '%s') AND (TIMESTAMP '%s')", start, end))
	} else if len(start) > 0 {
		cond = append(cond, fmt.Sprintf("(TIMESTAMP '%s') < p.createdAt", start))
	} else if len(end) > 0 {
		cond = append(cond, fmt.Sprintf("p.createdAt < (TIMESTAMP '%s')", end))
	}
	if userId != 0 {
		cond = append(cond, fmt.Sprintf("p.userId = %d", userId))
	}
	if len(tags) > 0 {
		queryTags := SQLFormattedArray(tags)
		cond = append(cond, fmt.Sprintf("%s && p.tags", queryTags))
	}
	if len(origin) > 0 {
		queryLoc := SQLFormattedArray(origin)
		cond = append(cond, fmt.Sprintf("p.location @> %s", queryLoc))
	}
	usingVotesTable := len(popularIn) > 0 || len(startDate) > 0 || len(endDate) > 0
	if usingVotesTable {
		joins += "JOIN Votes v ON v.pid = p.id\n"
		cond = append(cond, "kind=3")
	}

	getPosts := SQLGetItems("posts p", voteTable, SQLFieldsForPostResultAlias(),
		SQLFieldsForPostResult(), joins, popularIn, cond, usingVotesTable,
		upvotes, downvotes,
		sortOrder, limit, offset, startDate, endDate, forUser)

	rows, e := db.Query(getPosts)
	result := []PostResult{}
	if DidFail(e, "get posts\n", getPosts) {
		return result
	}
	defer rows.Close()

	result = ScanPostResults(rows, usingVotesTable)
	return result
}
