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
	Upvotes   float64
	Downvotes float64
}

func SQLFieldsForComment() string {
	return "id, postId, userId, replyId, content, createdAt, upvotes, downvotes"
}

func SQLFieldsForCommentResult() string {
	return "p.id, p.postId, p.userId, p.replyId, p.content, p.createdAt, p.upvotes, p.downvotes, u.id, u.name, u.registerDate, u.upvotes, u.downvotes"
}

func SQLFieldsForCommentResultAlias() string {
	return "p.id, p.postId, p.userId, p.replyId, p.content, p.createdAt, p.upvotes AS item_up, p.downvotes AS item_down, u.id, u.name, u.registerDate, u.upvotes, u.downvotes"
}

func ScanComment(row *sql.Row) (Comment, error) {
	c := Comment{}
	e := row.Scan(&c.ID, &c.PostID, &c.UserID, &c.ReplyID, &c.Content, &c.CreatedAt, &c.Upvotes, &c.Downvotes)
	return c, e
}

func ScanCommentResult(row *sql.Row) (CommentResult, error) {
	c := CommentResult{}
	var userId int64
	e := row.Scan(&c.ID, &c.PostID, &userId, &c.ReplyID, &c.Content, &c.CreatedAt, &c.Upvotes, &c.Downvotes, &c.Author.ID, &c.Author.Name, &c.Author.RegisterDate, &c.Author.Upvotes, &c.Author.Downvotes)
	return c, e
}

func ScanComments(rows *sql.Rows) []Comment {
	result := []Comment{}
	for rows.Next() {
		c := Comment{}
		var score float64
		var cred float64
		e := rows.Scan(&c.ID, &c.PostID, &c.UserID, &c.ReplyID, &c.Content, &c.CreatedAt, &c.Upvotes, &c.Downvotes, &cred, &score)
		if DidFail(e, "scan comment") {
			continue
		}
		result = append(result, c)
	}
	return result
}

func ScanCommentResults(rows *sql.Rows, hasVotes bool) []CommentResult {
	result := []CommentResult{}
	for rows.Next() {
		c := CommentResult{}
		var score float64
		var cred float64
		var userId int64
		var up float64
		var down float64
		var e error
		if hasVotes {
			e = rows.Scan(&c.ID, &c.PostID, &userId, &c.ReplyID, &c.Content, &c.CreatedAt, &c.Upvotes, &c.Downvotes,
				&c.Author.ID, &c.Author.Name, &c.Author.RegisterDate, &c.Author.Upvotes, &c.Author.Downvotes, &up, &down, &cred, &score)
		} else {
			e = rows.Scan(&c.ID, &c.PostID, &userId, &c.ReplyID, &c.Content, &c.CreatedAt, &c.Upvotes, &c.Downvotes,
				&c.Author.ID, &c.Author.Name, &c.Author.RegisterDate, &c.Author.Upvotes, &c.Author.Downvotes, &cred, &score)
		}
		if DidFail(e, "scan comment") {
			continue
		}
		result = append(result, c)
	}
	return result
}

func DBCreateComment(db *sql.DB, userId int64, content string, postId int64, replyId int64) (Comment, UserCont) {
	t := utc()
	nowTime := formatTime(t)

	insertComment := fmt.Sprintf(`
	INSERT INTO Comments(id, userId, postId, replyId, content, createdAt) 
	VALUES(nextval('comments_id_seq') * 10000 + extract(year from now() at time zone ('utc')), %d, %d, %d, $1, $2) 
	RETURNING %s`, userId, postId, replyId, SQLFieldsForComment())
	row := db.QueryRow(insertComment, content, nowTime)
	comment, e := ScanComment(row)
	if DidFail(e, "insert comment") {
		return comment, UserCont{}
	}

	insertCommentForUser := fmt.Sprintf(`
	INSERT INTO UserCont(userId, postId, commentId) 
	VALUES(%d, %d, %d) 
	RETURNING %s`, userId, postId, comment.ID, SQLFieldsForUserCont())
	row = db.QueryRow(insertCommentForUser)
	userCont, e := ScanUserCont(row)
	if DidFail(e, "insert comment ", comment.ID, " for user pref", userId) {
		return comment, userCont
	}

	return comment, userCont
}

func DBDeleteComment(db *sql.DB, postId int64, commentId int64) {
	deleteFromPostComments := `UPDATE comments SET thrashed=true WHERE id=$1 RETURNING userId`
	row := db.QueryRow(deleteFromPostComments, commentId)
	var userId int64
	e := row.Scan(&userId)
	if DidFail(e, "delete comment ", commentId, " from post ", postId, " comments table") {
		return
	}

	deletePostFromUser := `UPDATE UserCont SET thrashed=true WHERE userId=$1 AND postId=$2 AND commentId=$3`
	_, e = db.Exec(deletePostFromUser, userId, postId, commentId)
	DidFail(e, "delete comment ", commentId, " for post ", postId, " for user ", userId)
}

func DBUpdateComment(db *sql.DB, postId int64, commentId int64, content string) Comment {
	updateFromPostComments := fmt.Sprintf(`UPDATE comments SET content=$1 WHERE id=$2
	RETURNING %s`, SQLFieldsForComment())
	row := db.QueryRow(updateFromPostComments, content, commentId)
	comment, e := ScanComment(row)
	if DidFail(e, "update comment ", commentId, " from post ", postId, " comments table") {
		return Comment{}
	}
	return comment
}

func DBVoteComment(db *sql.DB, userId int64, postId int64, commentId int64, upvoteAmount int64, location []string, date string) (Comment, UserProfile, []UserPref) {
	var updateField string
	isUpvote := upvoteAmount > 0
	if isUpvote {
		updateField = "upvotes"
	} else {
		updateField = "downvotes"
		upvoteAmount = -upvoteAmount
	}
	voteQuery := SQLMakeVote(upComment, postId, commentId, location, upvoteAmount*sign(isUpvote), date)
	updateVoteForPost := fmt.Sprintf(`
		%s
		UPDATE Comments SET 
		%s = %s + %d
		WHERE id = %d
		RETURNING %s
		`, voteQuery,
		updateField, updateField, upvoteAmount,
		commentId, SQLFieldsForComment())

	row := db.QueryRow(updateVoteForPost)
	comment, e := ScanComment(row)
	if DidFail(e, "vote for post ", postId) {
		return Comment{}, UserProfile{}, []UserPref{}
	}

	user, uPref := DBVoteForUser(db, userId, comment.UserID, upvoteAmount*sign(isUpvote), location, date)
	cPref := DBCreateUserPref(db, userId, upComment, postId, commentId, upvoteAmount*sign(isUpvote))

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
func DBGetComments(db *sql.DB, postId int64, userId int64, replyId int64,
	start string, end string, popularIn []string, upvotes int64, downvotes int64,
	sortOrder SortOrder, limit int64, offset int64,
	startDate string, endDate string, forUser int64) []CommentResult {
	voteTable := "p"
	if len(popularIn) > 0 {
		voteTable = "v"
	}

	joins := "JOIN users u ON p.userId = u.id\n"
	cond := []string{fmt.Sprintf("postId = %d\n", postId), "thrashed=false"}
	if userId != 0 {
		cond = append(cond, fmt.Sprintf("p.userId = %d\n", userId))
	}
	if replyId != 0 {
		cond = append(cond, fmt.Sprintf("p.replyId = %d\n", replyId))
	}
	if len(start) > 0 && len(end) > 0 {
		cond = append(cond, fmt.Sprintf("p.createdAt BETWEEN (TIMESTAMP '%s') AND (TIMESTAMP '%s')\n", start, end))
	} else if len(start) > 0 {
		cond = append(cond, fmt.Sprintf("(TIMESTAMP '%s') < p.createdAt\n", start))
	} else if len(end) > 0 {
		cond = append(cond, fmt.Sprintf("p.createdAt < (TIMESTAMP '%s')\n", end))
	}

	usingVotesTable := len(popularIn) > 0 || len(startDate) > 0 || len(endDate) > 0
	if usingVotesTable {
		joins += "JOIN Votes v ON v.pid = p.id\n"
		cond = append(cond, "kind=2")
	}

	getComments := SQLGetItems("Comments p", voteTable, SQLFieldsForCommentResultAlias(),
		SQLFieldsForCommentResult(), joins, popularIn, cond, usingVotesTable,
		upvotes, downvotes,
		sortOrder, limit, offset, startDate, endDate, forUser)

	rows, e := db.Query(getComments)
	if DidFail(e, "failed to get comments") {
		return []CommentResult{}
	}

	result := ScanCommentResults(rows, usingVotesTable)
	return result
}
