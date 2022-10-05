package main

import (
	"database/sql"
	"fmt"
	"time"
)

type Comment struct {
	ID        int64
	UserID    int64
	ReplyID   int64
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
	Upvotes   float64
	Downvotes float64
}

func SQLFieldsForComment() string {
	return "id, userId, replyId, content, createdAt, updatedAt, upvotes, downvotes"
}

func ScanComment(row *sql.Row) (Comment, error) {
	c := Comment{}
	e := row.Scan(&c.ID, &c.UserID, &c.ReplyID, &c.Content, &c.CreatedAt, &c.UpdatedAt, &c.Upvotes, &c.Downvotes)
	return c, e
}

func ScanComments(rows *sql.Rows) []Comment {
	result := []Comment{}
	for rows.Next() {
		c := Comment{}
		var score float64
		var cred float64
		e := rows.Scan(&c.ID, &c.UserID, &c.ReplyID, &c.Content, &c.CreatedAt, &c.UpdatedAt, &c.Upvotes, &c.Downvotes, &cred, &score)
		if DidFail(e, "scan comment") {
			continue
		}
		result = append(result, c)
	}
	return result
}

func DBCreateComment(db *sql.DB, userId int, content string, postId int, replyId int) (Comment, UserCont) {
	nowTime := formatNow()

	insertComment := fmt.Sprintf(`INSERT INTO Post%d(userId, replyId, content, createdAt, updatedAt) 
	VALUES(%d, %d, $1, $2, $2) RETURNING %s`, postId, userId, replyId, SQLFieldsForComment())
	row := db.QueryRow(insertComment, content, nowTime)
	comment, e := ScanComment(row)
	if DidFail(e, "insert comment") {
		return comment, UserCont{}
	}

	insertCommentForUser := fmt.Sprintf(`INSERT INTO User%dCont(postId, commentId) VALUES(%d, %d)`, userId, postId, comment.ID)
	row = db.QueryRow(insertCommentForUser)
	userCont, e := ScanUserCont(row)
	if DidFail(e, "insert comment ", comment.ID, " for user pref", userId) {
		return comment, userCont
	}

	return comment, userCont
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

func DBUpdateComment(db *sql.DB, postId int, commentId int, content string) Comment {
	updateFromPostComments := fmt.Sprintf(`UPDATE post%d SET content=$1 WHERE id=$2
	RETURNING %s`, postId, SQLFieldsForComment())
	row := db.QueryRow(updateFromPostComments, content, commentId)
	comment, e := ScanComment(row)
	if DidFail(e, "update comment ", commentId, " from post ", postId, " comments table") {
		return Comment{}
	}
	return comment
}

func DBVoteComment(db *sql.DB, userId int64, postId int64, commentId int64, upvoteAmount int64) (Comment, User, []UserPref) {
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
		UPDATE post%d
		SET %s = cooldown(%s, updatedAt, '%s', 31536000) + %d,
		%s = cooldown(%s, updatedAt, '%s', 31536000),
		updatedAt = '%s'
		WHERE id = %d
		RETURNING %s
		`, postId, updateField, updateField, nowTime, upvoteAmount, otherField, otherField, nowTime, nowTime, commentId, SQLFieldsForComment())
	row := db.QueryRow(updateVoteForPost)
	comment, e := ScanComment(row)
	if DidFail(e, "vote for post ", postId) {
		return Comment{}, User{}, []UserPref{}
	}

	user, uPref := DBVoteForUser(db, userId, comment.UserID, upvoteAmount*sign(isUpvote))
	createPref := fmt.Sprintf(`
	INSERT INTO User%dPref (kind, pid, sid) 
	VALUES(2, %d, %d) ON CONFLICT (kind, pid, sid) DO NOTHING;
	UPDATE User%dPref
	SET %s = cooldown(%s, updatedAt, '%s', 31536000) + %d,
	%s = cooldown(%s, updatedAt, '%s', 31536000),
	WHERE kind=2 AND pid=%d AND sid=%d
	RETURNING %s
	`, userId, postId, commentId, userId, updateField, updateField, nowTime, upvoteAmount, otherField, otherField, nowTime, postId, commentId, SQLFieldsForUserPref())
	row = db.QueryRow(createPref)
	cPref, e := ScanUserPrefRow(row)
	if DidFail(e, "vote for post ", postId) {
		return comment, user, []UserPref{uPref}
	}

	return comment, user, []UserPref{uPref, cPref}
}

func DBGetComment(db *sql.DB, postId int64, commentId int64) Comment {
	getComment := fmt.Sprintf(`SELECT %s FROM Post%d WHERE id = %d`, SQLFieldsForComment(), postId, commentId)
	row := db.QueryRow(getComment)
	comment, e := ScanComment(row)
	if DidFail(e, "get comment ", commentId, " for post ", postId) {
		return Comment{}
	}
	return comment
}

// ignore userId if 0, ignore replyId if 0, start < CreatedAt < end ignore if empty, ignore upvotes if 0
func DBGetComments(db *sql.DB, postId int64, userId int64, replyId int64, start string, end string, upvotes int64, downvotes int64, sortOrder SortOrder, limit int64, offset int64) []Comment {
	getComments := fmt.Sprintf(`SELECT %s, RATIO(upvotes, downvotes) AS cred, upvotes * RATIO(upvotes, downvotes) AS score FROM Post%d `, SQLFieldsForComment(), postId)

	cond := ""
	if userId != 0 {
		cond += fmt.Sprintf("userId = %d ", userId)
	}
	if replyId != 0 {
		cond += fmt.Sprintf("replyId = %d ", replyId)
	}
	if len(start) > 0 && len(end) > 0 {
		cond += fmt.Sprintf("createdAt BETWEEN (TIMESTAMP '%s') AND (TIMESTAMP '%s') ", start, end)
	} else if len(start) > 0 {
		cond += fmt.Sprintf("(TIMESTAMP '%s') < createdAt ", start)
	} else if len(end) > 0 {
		cond += fmt.Sprintf("createdAt < (TIMESTAMP '%s') ", end)
	}
	if upvotes != 0 {
		if upvotes > 0 {
			cond += fmt.Sprintf("upvotes > %d ", upvotes)
		} else {
			cond += fmt.Sprintf("upvotes < %d ", upvotes)
		}
	}
	if downvotes != 0 {
		if downvotes > 0 {
			cond += fmt.Sprintf("downvotes > %d ", downvotes)
		} else {
			cond += fmt.Sprintf("downvotes < %d ", downvotes)
		}
	}

	if len(cond) > 0 {
		getComments += "WHERE " + cond + "\n"
	}

	getComments += SQLSortOrder(sortOrder)
	getComments += fmt.Sprintf("LIMIT %d OFFSET %d\n", limit, offset)

	rows, e := db.Query(getComments)
	if DidFail(e, "failed to get comments") {
		return []Comment{}
	}

	result := ScanComments(rows)
	return result
}
