package main

import (
	"database/sql"
	"fmt"
	"time"
)

func DBCreateComment(db *sql.DB, userId int, content string, postId int, replyId int) {
	nowTime := formatNow()

	insertComment := fmt.Sprintf(`INSERT INTO Post%d(userId, replyId, content, createdAt, updatedAt) 
	VALUES(%d, %d, $1, $2, $2) RETURNING id`, postId, userId, replyId)
	row := db.QueryRow(insertComment, content, nowTime)
	var commentId int64
	e := row.Scan(&commentId)
	if DidFail(e, "get comment ID") {
		return
	}

	insertCommentForUser := fmt.Sprintf(`INSERT INTO User%dCont(postId, commentId) VALUES(%d, %d)`, userId, postId, commentId)
	_, e = db.Exec(insertCommentForUser)
	if DidFail(e, "insert comment ", commentId, " to user ", userId) {
		return
	}
}

func DBDeleteComment(db *sql.DB, postId int, commentId int) {
	getUserId := fmt.Sprintf(`SELECT userId FROM post%d WHERE id=$1`, postId)
	row := db.QueryRow(getUserId, commentId)
	var userId int64
	e := row.Scan(&userId)
	if !DidFail(e, "get userId for deleting comment ", commentId, " from post ", postId) {
		deletePostFromUser := fmt.Sprintf(`DELETE FROM User%dCont WHERE postId=$1 AND commentId=$2`, userId)
		_, e = db.Exec(deletePostFromUser, postId, commentId)
		DidFail(e, "delete comment ", commentId, " for post ", postId, " for user ", userId)
	}

	deleteFromPostComments := fmt.Sprintf(`DELETE FROM post%d WHERE id=$1`, postId)
	_, e = db.Exec(deleteFromPostComments, commentId)
	DidFail(e, "delete comment ", commentId, " from post ", postId, " comments table")
}

func DBUpdateComment(db *sql.DB, postId int, commentId int, content string) {
	updateFromPostComments := fmt.Sprintf(`UPDATE post%d SET content=$1 WHERE id=$2`, postId)
	_, e := db.Exec(updateFromPostComments, content, commentId)
	DidFail(e, "update comment ", commentId, " from post ", postId, " comments table")
}

func DBVoteComment(db *sql.DB, userId int64, postId int64, commentId int64, isUpvote bool) {
	getOldVote := fmt.Sprintf(`SELECT upvotes, downvotes FROM User%dPref WHERE kind=2 AND pid=$1 AND sid=$2`, userId)
	rows, e := db.Query(getOldVote, postId, commentId)
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
		updateVoteForComment := fmt.Sprintf(`
			SELECT userId, upvotes, downvotes, updatedAt, date_frac(updatedAt, '%s', 604800.0) FROM post%d WHERE id = %d;
			UPDATE post%d
			SET %s = %s + date_frac(updatedAt, '%s', 604800.0),
			updatedAt = '%s'
			WHERE id = %d;
			`, nowTime, postId, commentId, postId, updateField, updateField, nowTime, nowTime, commentId)
		rows, e = db.Query(updateVoteForComment)
		if DidFail(e, "vote for post ", postId) {
			return
		}
		var updatedAt time.Time
		var posterId int64
		for rows.Next() {
			e = rows.Scan(&posterId, &upvote, &downvote, &updatedAt, &voteAmount)
			if DidFail(e, "scan upvote and downvote for post ", postId) {
				continue
			}
		}

		DBVoteForUser(db, userId, posterId, isUpvote)
		createPref := fmt.Sprintf(`INSERT INTO User%dPref (kind, pid, sid, %s) VALUES(2, $1, $2, $3)`, userId, updateField)
		_, e = db.Exec(createPref, postId, commentId, voteAmount)
		if DidFail(e, "vote for post ", postId) {
			return
		}
	} else if (isUpvote && upvote > 0) || (!isUpvote && downvote > 0) {
		updatePref := fmt.Sprintf(`
			UPDATE User%dPref SET %s = 0 WHERE kind=2 AND pid=%d AND sid=%d
			`, userId, updateField, postId, commentId)
		_, e = db.Exec(updatePref)
		if DidFail(e, "update ", updateField, " for post ", postId) {
			return
		}
		if isUpvote {
			voteAmount = upvote
		} else {
			voteAmount = downvote
		}
		updateVoteForComment := fmt.Sprintf(`UPDATE post%d
			SET %s = %s - %f
			WHERE id = $1
			`, postId, updateField, updateField, voteAmount)
		_, e = db.Exec(updateVoteForComment, commentId)
		if DidFail(e, "vote for post ", postId) {
			return
		}
	} else {
		if isUpvote {
			voteAmount = downvote
		} else {
			voteAmount = upvote
		}
		updateVoteForComment := fmt.Sprintf(`
			SELECT upvotes, downvotes, updatedAt, date_frac(updatedAt, '%s', 604800.0) FROM post%d WHERE id = %d;
			UPDATE post%d
			SET %s = %s + date_frac(updatedAt, '%s', 604800.0),
			updatedAt = TIMESTAMP '%s',
			%s = %s - %f
			WHERE id = %d;
			`, nowTime, postId, commentId, postId, updateField, updateField, nowTime, nowTime, otherField, otherField, voteAmount, commentId)
		rows, e = db.Query(updateVoteForComment)
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
			UPDATE User%dPref SET %s = $1, %s = 0 WHERE kind=2 AND pid=$2 AND sid=$3
			`, userId, updateField, otherField)
		_, e = db.Exec(createPref, voteAmount, postId, commentId)
		if DidFail(e, "update ", updateField, " for post ", postId) {
			return
		}
	}
}
