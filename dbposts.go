package main

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
)

func DBCreatePost(db *sql.DB, userId int, content string, tags []string) {
	nowTime := formatNow()
	insertPost := fmt.Sprintf(`INSERT INTO posts(userId, content, tags, createdAt, updatedAt) 
	VALUES ($1, $2, %s, '%s', '%s') RETURNING id`, SQLFormattedArray(tags), nowTime, nowTime)
	row := db.QueryRow(insertPost, userId, content)
	var postId int64
	e := row.Scan(&postId)
	if DidFail(e, "get post ID") {
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

func DBVotePost(db *sql.DB, userId int64, postId int64, isUpvote bool) {
	getOldVote := fmt.Sprintf(`SELECT upvotes, downvotes FROM User%dPref WHERE kind=3 AND pid=$1`, userId)
	rows, e := db.Query(getOldVote, postId)
	if DidFail(e, "get downvote and upvote for post ", postId) {
		return
	}
	count := 0
	var upvote float64
	var downvote float64
	for rows.Next() {
		e = rows.Scan(&upvote, &downvote)
		if DidFail(e, "scan upvote and downvote for post ", postId) {
			continue
		}
		count += 1
	}

	currentTime := time.Now().UTC()
	nowTime := formatTime(currentTime)
	var updateField string
	var otherField string
	voteAmount := 0.0
	if isUpvote {
		updateField = "upvotes"
		otherField = "downvotes"
	} else {
		updateField = "downvotes"
		otherField = "upvotes"
	}
	if count == 0 {
		updateVoteForPost := fmt.Sprintf(`
			SELECT userId, tags, upvotes, downvotes, updatedAt, date_frac(updatedAt, '%s', 604800) FROM posts WHERE id = %d;
			UPDATE posts 
			SET %s = %s + date_frac(updatedAt, '%s', 604800),
			updatedAt = '%s'
			WHERE id = %d;
			`, nowTime, postId, updateField, updateField, nowTime, nowTime, postId)
		rows, e = db.Query(updateVoteForPost)
		if DidFail(e, "vote for post ", postId) {
			return
		}
		var updatedAt time.Time
		var tags []string
		var posterId int64
		for rows.Next() {
			e = rows.Scan(&posterId, pq.Array(&tags), &upvote, &downvote, &updatedAt, &voteAmount)
			if DidFail(e, "scan upvote and downvote for post ", postId) {
				continue
			}
		}

		DBVoteTags(db, userId, tags, isUpvote)
		DBVoteForUser(db, userId, posterId, isUpvote)
		createPref := fmt.Sprintf(`INSERT INTO User%dPref (kind, pid, sid, %s) VALUES(3, $1, -1, $2)`, userId, updateField)
		_, e = db.Exec(createPref, postId, voteAmount)
		if DidFail(e, "vote for post ", postId) {
			return
		}
	} else if (isUpvote && upvote > 0) || (!isUpvote && downvote > 0) {
		updatePref := fmt.Sprintf(`
			UPDATE User%dPref SET %s = 0 WHERE kind=3 AND pid=%d
			`, userId, updateField, postId)
		_, e = db.Exec(updatePref)
		if DidFail(e, "update ", updateField, " for post ", postId) {
			return
		}
		if isUpvote {
			voteAmount = upvote
		} else {
			voteAmount = downvote
		}
		updateVoteForPost := fmt.Sprintf(`UPDATE posts 
			SET %s = %s - %f
			WHERE id = $1
			`, updateField, updateField, voteAmount)
		_, e = db.Exec(updateVoteForPost, postId)
		if DidFail(e, "vote for post ", postId) {
			return
		}
	} else {
		if isUpvote {
			voteAmount = downvote
		} else {
			voteAmount = upvote
		}
		updateVoteForPost := fmt.Sprintf(`
			SELECT upvotes, downvotes, updatedAt, date_frac(updatedAt, '%s', 604800.0) FROM posts WHERE id = %d;
			UPDATE posts 
			SET %s = %s + date_frac(updatedAt, '%s', 604800.0),
			updatedAt = TIMESTAMP '%s',
			%s = %s - %f
			WHERE id = %d;
			`, nowTime, postId, updateField, updateField, nowTime, nowTime, otherField, otherField, voteAmount, postId)
		rows, e = db.Query(updateVoteForPost)
		if DidFail(e, "vote for post ", postId) {
			return
		}
		var updatedAt time.Time
		for rows.Next() {
			e = rows.Scan(&upvote, &downvote, &updatedAt, &voteAmount)
			if DidFail(e, "scan upvote and downvote for post ", postId) {
				continue
			}
		}

		createPref := fmt.Sprintf(`
			UPDATE User%dPref SET %s = $1, %s = 0 WHERE kind=3 AND pid=$2
			`, userId, updateField, otherField)
		_, e = db.Exec(createPref, voteAmount, postId)
		if DidFail(e, "update ", updateField, " for post ", postId) {
			return
		}
	}
}

func DBDeletePost(db *sql.DB, postId int64) {
	getUserId := `SELECT userId FROM posts WHERE id=$1`
	row := db.QueryRow(getUserId, postId)
	var userId int64
	e := row.Scan(&userId)
	if !DidFail(e, "get userId for deleting post ", postId) {
		deletePostFromUser := fmt.Sprintf(`DELETE FROM User%dCont WHERE postId=$2`, userId)
		_, e = db.Exec(deletePostFromUser, postId)
		DidFail(e, "delete post ", postId, " for user ", userId)
	}

	deleteFromPosts := `DELETE FROM posts WHERE id=$1`
	_, e = db.Exec(deleteFromPosts, postId)
	DidFail(e, "delete post from posts table")

	deletePostTable := fmt.Sprintf(`DROP TABLE User%dCont`, postId)
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
		FROM (SELECT id, userId, content, tags, createdAt, upvotes, COALESCE(upvotes / NULLIF(upvotes + downvotes, 0), 0.0) AS cred FROM posts) compute 
		WHERE createdAt BETWEEN (TIMESTAMP '%s') AND (TIMESTAMP '%s')
		
		`, startDate, endDate)

	if len(tags) > 0 {
		queryTags := SQLFormattedArray(tags)
		getPosts += fmt.Sprintf("AND %s && tags\n", queryTags)
	}

	switch sortOrder {
	case soScore:
		getPosts += "ORDER BY score\n"
	case soCred:
		getPosts += "ORDER BY score\n"
	case soUpvotes:
		getPosts += "ORDER BY score\n"
	case soDownvotes:
		getPosts += "ORDER BY score\n"
	case soControversial:
		getPosts += "ORDER BY COALESCE(1 / NULLIF(ABS(cred - 0.5), 0), 9e90)\n"
	case soCreatedAt:
		getPosts += "ORDER BY createdAt\n"
	}
	getPosts += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	rows, e := db.Query(getPosts)
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
