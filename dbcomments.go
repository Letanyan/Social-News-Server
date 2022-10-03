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

func DBVoteComment(db *sql.DB, userId int64, postId int64, commentId int64, upvoteAmount int64) {
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
	updateVoteForPost := fmt.Sprintf(`
		SELECT userId FROM post%d WHERE id = %d;
		UPDATE post%d
		SET %s = cooldown(%s, updatedAt, '%s', 31536000) + %d,
		%s = cooldown(%s, updatedAt, '%s', 31536000),
		updatedAt = '%s'
		WHERE id = %d;
		`, postId, commentId, postId, updateField, updateField, nowTime, upvoteAmount, otherField, otherField, nowTime, nowTime, commentId)
	rows, e := db.Query(updateVoteForPost)
	if DidFail(e, "vote for post ", postId) {
		return
	}
	var posterId int64
	for rows.Next() {
		e = rows.Scan(&posterId)
		if DidFail(e, "scan upvote and downvote for post ", postId) {
			continue
		}
	}

	DBVoteForUser(db, userId, posterId, isUpvote)
	createPref := fmt.Sprintf(`
	INSERT INTO User%dPref (kind, pid, sid) 
	VALUES(2, %d, %d) ON CONFLICT (kind, pid, sid) DO NOTHING;
	UPDATE User%dPref
	SET %s = %s + %d
	WHERE kind=2 AND pid=%d AND sid=%d
	`, userId, postId, commentId, userId, updateField, updateField, upvoteAmount, postId, commentId)
	_, e = db.Exec(createPref)
	if DidFail(e, "vote for post ", postId) {
		return
	}
}
