package main

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
)

func DBCreatePost(db *sql.DB, userId int, content string, tags []string) {
	t := utc()
	nowTime := formatNow()
	year := t.Year()
	postId := idFromTime(t)
	insertPost := fmt.Sprintf(`INSERT INTO posts%d(id, userId, content, tags, createdAt, updatedAt) 
	VALUES (%d, $1, $2, %s, '%s', '%s')`, year, postId, SQLFormattedArray(tags), nowTime, nowTime)
	_, e := db.Exec(insertPost, userId, content)
	if DidFail(e, "create post") {
		return
	}

	insertPostForUser := fmt.Sprintf(`INSERT INTO User%dCont(postId, commentId) VALUES(%d, -1)`, userId, postId)
	_, e = db.Exec(insertPostForUser)
	if DidFail(e, "insert post to user") {
		return
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
		return
	}
}

func DBVotePost(db *sql.DB, userId int64, postId int64, upvoteAmount int64) {
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
		SELECT userId, tags FROM posts%d WHERE id = %d;
		UPDATE posts%d 
		SET %s = cooldown(%s, updatedAt, '%s', 31536000) + %d,
		%s = cooldown(%s, updatedAt, '%s', 31536000),
		updatedAt = '%s'
		WHERE id = %d;
		`, year, postId, year, updateField, updateField, nowTime, upvoteAmount, otherField, otherField, nowTime, nowTime, postId)
	rows, e := db.Query(updateVoteForPost)
	if DidFail(e, "vote for post ", postId) {
		return
	}
	var tags []string
	var posterId int64
	for rows.Next() {
		e = rows.Scan(&posterId, pq.Array(&tags))
		if DidFail(e, "scan upvote and downvote for post ", postId) {
			continue
		}
	}

	DBVoteTags(db, userId, tags, isUpvote)
	DBVoteForUser(db, userId, posterId, isUpvote)
	createPref := fmt.Sprintf(`
	INSERT INTO User%dPref (kind, pid, sid) 
	VALUES(3, %d, -1) ON CONFLICT (kind, pid, sid) DO NOTHING;
	UPDATE User%dPref
	SET %s = %s + %d
	WHERE kind=3 AND pid=%d
	`, userId, postId, userId, updateField, updateField, upvoteAmount, postId)
	_, e = db.Exec(createPref)
	if DidFail(e, "vote for post ", postId) {
		return
	}
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

func DBGetPosts(db *sql.DB, userId int64, tags []string, sortOrder SortOrder, limit int, offset int, startDate string, endDate string) []string {
	getPosts := fmt.Sprintf(`SELECT id, userId, content, tags, createdAt, cred, upvotes * cred AS score 
		FROM (SELECT id, userId, content, tags, createdAt, upvotes, COALESCE(upvotes / NULLIF(upvotes + downvotes, 0), 0.0) AS cred FROM posts{}) compute 
		WHERE createdAt BETWEEN (TIMESTAMP '%s') AND (TIMESTAMP '%s')
		`, startDate, endDate)

	if len(tags) > 0 {
		queryTags := SQLFormattedArray(tags)
		getPosts += fmt.Sprintf("AND %s && tags\n", queryTags)
	}

	sDate := parseTime(startDate)
	eDate := parseTime(endDate)
	years := yearsBetweenDates(sDate, eDate)
	postQueries := BuildUnionForYears(getPosts, years)

	switch sortOrder {
	case soScore:
		postQueries += "ORDER BY score\n"
	case soCred:
		postQueries += "ORDER BY score\n"
	case soUpvotes:
		postQueries += "ORDER BY score\n"
	case soDownvotes:
		postQueries += "ORDER BY score\n"
	case soControversial:
		postQueries += "ORDER BY COALESCE(1 / NULLIF(ABS(cred - 0.5), 0), 9e90)\n"
	case soCreatedAt:
		postQueries += "ORDER BY createdAt\n"
	}

	postQueries += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	rows, e := db.Query(postQueries)
	if DidFail(e, "get posts") {
		return []string{}
	}
	defer rows.Close()

	result := []string{}
	for rows.Next() {
		var id int64
		var userId int64
		var content string
		var tags []string
		var createdAt time.Time
		var cred float64
		var score float64

		e = rows.Scan(&id, &userId, &content, pq.Array(&tags), &createdAt, &cred, &score)
		if DidFail(e, "read row") {
			continue
		}

		result = append(result, fmt.Sprint(id, "][", userId, "][", content, "][", createdAt, "][", cred, "][", score, "]"))
	}

	return result
}
