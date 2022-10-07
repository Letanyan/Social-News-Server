package main

import (
	"database/sql"
	"fmt"
	"time"
)

type Comment struct {
	ID        int64
	PostID    int64
	UserID    int64
	ReplyID   int64
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
	Upvotes   float64
	Downvotes float64
}

type CommentResult struct {
	ID        int64
	PostID    int64
	Author    UserProfile
	ReplyID   int64
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
	Upvotes   float64
	Downvotes float64
}

func SQLFieldsForComment() string {
	return "id, postId, userId, replyId, content, createdAt, updatedAt, upvotes, downvotes"
}

func SQLFieldsForCommentResult() string {
	return "p.id, p.postId, p.userId, p.replyId, p.content, p.createdAt, p.updatedAt, p.upvotes, p.downvotes, u.id, u.name, u.registerDate, u.upvotes, u.downvotes"
}

func ScanComment(row *sql.Row) (Comment, error) {
	c := Comment{}
	e := row.Scan(&c.ID, &c.PostID, &c.UserID, &c.ReplyID, &c.Content, &c.CreatedAt, &c.UpdatedAt, &c.Upvotes, &c.Downvotes)
	return c, e
}

func ScanCommentResult(row *sql.Row) (CommentResult, error) {
	c := CommentResult{}
	var userId int64
	e := row.Scan(&c.ID, &c.PostID, &userId, &c.ReplyID, &c.Content, &c.CreatedAt, &c.UpdatedAt, &c.Upvotes, &c.Downvotes, &c.Author.ID, &c.Author.Name, &c.Author.RegisterDate, &c.Author.Upvotes, &c.Author.Downvotes)
	return c, e
}

func ScanComments(rows *sql.Rows) []Comment {
	result := []Comment{}
	for rows.Next() {
		c := Comment{}
		var score float64
		var cred float64
		e := rows.Scan(&c.ID, &c.PostID, &c.UserID, &c.ReplyID, &c.Content, &c.CreatedAt, &c.UpdatedAt, &c.Upvotes, &c.Downvotes, &cred, &score)
		if DidFail(e, "scan comment") {
			continue
		}
		result = append(result, c)
	}
	return result
}

func ScanCommentResults(rows *sql.Rows) []CommentResult {
	result := []CommentResult{}
	for rows.Next() {
		c := CommentResult{}
		var score float64
		var cred float64
		var userId int64
		e := rows.Scan(&c.ID, &c.PostID, &userId, &c.ReplyID, &c.Content, &c.CreatedAt, &c.UpdatedAt, &c.Upvotes, &c.Downvotes,
			&c.Author.ID, &c.Author.Name, &c.Author.RegisterDate, &c.Author.Upvotes, &c.Author.Downvotes, &cred, &score)
		if DidFail(e, "scan comment") {
			continue
		}
		result = append(result, c)
	}
	return result
}

func DBCreateComment(db *sql.DB, userId int, content string, postId int, replyId int) (Comment, UserCont) {
	nowTime := formatNow()

	insertComment := fmt.Sprintf(`INSERT INTO Comments(userId, postId, replyId, content, createdAt, updatedAt) 
	VALUES(%d, $3, %d, $1, $2, $2) RETURNING %s`, userId, replyId, SQLFieldsForComment())
	row := db.QueryRow(insertComment, content, nowTime, postId)
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
	getUserId := `SELECT userId FROM Comments WHERE id=$1`
	row := db.QueryRow(getUserId, commentId)
	var userId int64
	e := row.Scan(&userId)
	if !DidFail(e, "get userId for deleting comment ", commentId, " from post ", postId) {
		deletePostFromUser := fmt.Sprintf(`DELETE FROM User%dCont WHERE postId=$1 AND commentId=$2`, userId)
		_, e = db.Exec(deletePostFromUser, postId, commentId)
		DidFail(e, "delete comment ", commentId, " for post ", postId, " for user ", userId)
	}

	deleteFromPostComments := `DELETE FROM comments WHERE id=$1`
	_, e = db.Exec(deleteFromPostComments, commentId)
	DidFail(e, "delete comment ", commentId, " from post ", postId, " comments table")
}

func DBUpdateComment(db *sql.DB, postId int, commentId int, content string) Comment {
	updateFromPostComments := fmt.Sprintf(`UPDATE comments SET content=$1 WHERE id=$2
	RETURNING %s`, SQLFieldsForComment())
	row := db.QueryRow(updateFromPostComments, content, commentId)
	comment, e := ScanComment(row)
	if DidFail(e, "update comment ", commentId, " from post ", postId, " comments table") {
		return Comment{}
	}
	return comment
}

func DBVoteComment(db *sql.DB, userId int64, postId int64, commentId int64, upvoteAmount int64) (Comment, UserProfile, []UserPref) {
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
		UPDATE Comments
		SET %s = cooldown(%s, updatedAt, '%s', 31536000) + %d,
		%s = cooldown(%s, updatedAt, '%s', 31536000),
		updatedAt = '%s'
		WHERE id = %d
		RETURNING %s
		`, updateField, updateField, nowTime, upvoteAmount, otherField, otherField, nowTime, nowTime, commentId, SQLFieldsForComment())
	row := db.QueryRow(updateVoteForPost)
	comment, e := ScanComment(row)
	if DidFail(e, "vote for post ", postId) {
		return Comment{}, UserProfile{}, []UserPref{}
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

func DBGetComment(db *sql.DB, postId int64, commentId int64) CommentResult {
	getComment := fmt.Sprintf(`SELECT %s FROM Comments p JOIN users u ON p.userId = u.id WHERE id = %d`, SQLFieldsForCommentResult(), commentId)
	row := db.QueryRow(getComment)
	comment, e := ScanCommentResult(row)
	if DidFail(e, "get comment ", commentId, " for post ", postId) {
		return CommentResult{}
	}
	return comment
}

// ignore userId if 0, ignore replyId if 0, start < CreatedAt < end ignore if empty, ignore upvotes if 0
func DBGetComments(db *sql.DB, postId int64, userId int64, replyId int64, start string, end string, upvotes int64, downvotes int64, sortOrder SortOrder, limit int64, offset int64) []CommentResult {
	getComments := fmt.Sprintf(`SELECT %s, RATIO(upvotes, downvotes) AS cred, upvotes * RATIO(upvotes, downvotes) AS score FROM Comments p JOIN users u ON p.userId = u.id`, SQLFieldsForCommentResult())

	cond := ""
	if userId != 0 {
		cond += fmt.Sprintf("p.userId = %d ", userId)
	}
	if replyId != 0 {
		cond += fmt.Sprintf("p.replyId = %d ", replyId)
	}
	if len(start) > 0 && len(end) > 0 {
		cond += fmt.Sprintf("p.createdAt BETWEEN (TIMESTAMP '%s') AND (TIMESTAMP '%s') ", start, end)
	} else if len(start) > 0 {
		cond += fmt.Sprintf("(TIMESTAMP '%s') < p.createdAt ", start)
	} else if len(end) > 0 {
		cond += fmt.Sprintf("p.createdAt < (TIMESTAMP '%s') ", end)
	}
	if upvotes != 0 {
		if upvotes > 0 {
			cond += fmt.Sprintf("p.upvotes > %d ", upvotes)
		} else {
			cond += fmt.Sprintf("p.upvotes < %d ", upvotes)
		}
	}
	if downvotes != 0 {
		if downvotes > 0 {
			cond += fmt.Sprintf("p.downvotes > %d ", downvotes)
		} else {
			cond += fmt.Sprintf("p.downvotes < %d ", downvotes)
		}
	}

	if len(cond) > 0 {
		getComments += "WHERE " + cond + "\n"
	}

	getComments += SQLSortOrder(sortOrder)
	getComments += fmt.Sprintf("LIMIT %d OFFSET %d\n", limit, offset)

	rows, e := db.Query(getComments)
	if DidFail(e, "failed to get comments") {
		return []CommentResult{}
	}

	result := ScanCommentResults(rows)
	return result
}
