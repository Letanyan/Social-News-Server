package main

import (
	"database/sql"
	"fmt"
	"time"
)

type Comment struct {
	ID         int64
	PostID     int64
	UserID     int64
	ReplyID    int64
	Content    string
	CreatedAt  time.Time
	Upvotes    int64
	Downvotes  int64
	ReplyCount int64
	Trashed    bool
	Edited     bool
	IsReview   bool

	Score float64
	Cred  float64
	Rank  float64
}

type CommentResult struct {
	ID         int64
	PostID     int64
	Author     UserProfile
	ReplyID    int64
	Content    string
	CreatedAt  time.Time
	Upvotes    int64
	Downvotes  int64
	ReplyCount int64
	Trashed    bool
	Edited     bool
	IsReview   bool

	Score float64
	Cred  float64
	Rank  float64
}

func SQLFieldsForComment() string {
	return "id, postId, userId, replyId, content, createdAt, upvotes, downvotes, replyCount, trashed, edited, isReview"
}

func SQLFieldsForCommentResult() string {
	return "p.id, p.postId, p.userId, p.replyId, p.content, p.createdAt, p.upvotes, p.downvotes, p.replyCount, p.trashed, p.edited, p.isReview, u.id, u.name, u.registerDate, u.upvotes, u.downvotes, u.email"
}

func SQLFieldsForCommentResultAlias() string {
	return "p.id, p.postId, p.userId, p.replyId, p.content, p.createdAt, p.upvotes AS item_up, p.downvotes AS item_down, p.replyCount, p.trashed, p.edited, p.isReview, u.id, u.name, u.registerDate, u.upvotes, u.downvotes, u.email"
}

func ScanComment(row *sql.Row) (Comment, error) {
	c := Comment{}
	e := row.Scan(&c.ID, &c.PostID, &c.UserID, &c.ReplyID, &c.Content, &c.CreatedAt, &c.Upvotes, &c.Downvotes, &c.ReplyCount, &c.Trashed, &c.Edited, &c.IsReview)
	return c, e
}

func ScanCommentResult(row *sql.Row) (CommentResult, error) {
	c := CommentResult{}
	var userId int64
	var email string
	e := row.Scan(&c.ID, &c.PostID, &userId, &c.ReplyID, &c.Content, &c.CreatedAt, &c.Upvotes, &c.Downvotes, &c.ReplyCount, &c.Trashed, &c.Edited, &c.IsReview, &c.Author.ID, &c.Author.Name, &c.Author.RegisterDate, &c.Author.Upvotes, &c.Author.Downvotes, &email)
	c.Author.IsAgent = len(email) == 0
	return c, e
}

func ScanComments(rows *sql.Rows) []Comment {
	result := []Comment{}
	for rows.Next() {
		c := Comment{}
		e := rows.Scan(&c.ID, &c.PostID, &c.UserID, &c.ReplyID, &c.Content, &c.CreatedAt, &c.Upvotes, &c.Downvotes, &c.ReplyCount, &c.Trashed, &c.Edited, &c.IsReview, &c.Cred, &c.Score)
		if DidFail(e, "scan comment") {
			continue
		}
		result = append(result, c)
	}
	return result
}

func ScanCommentResults(rows *sql.Rows, hasVotes bool, hasRank bool) []CommentResult {
	result := []CommentResult{}
	for rows.Next() {
		c := CommentResult{}
		var userId int64
		var up int64
		var down int64
		var e error
		var email string
		if hasRank {
			if hasVotes {
				e = rows.Scan(&c.ID, &c.PostID, &userId, &c.ReplyID, &c.Content, &c.CreatedAt, &c.Upvotes, &c.Downvotes, &c.ReplyCount, &c.Trashed, &c.Edited, &c.IsReview,
					&c.Author.ID, &c.Author.Name, &c.Author.RegisterDate, &c.Author.Upvotes, &c.Author.Downvotes, &email, &up, &down, &c.Cred, &c.Score, &c.Rank)
			} else {
				e = rows.Scan(&c.ID, &c.PostID, &userId, &c.ReplyID, &c.Content, &c.CreatedAt, &c.Upvotes, &c.Downvotes, &c.ReplyCount, &c.Trashed, &c.Edited, &c.IsReview,
					&c.Author.ID, &c.Author.Name, &c.Author.RegisterDate, &c.Author.Upvotes, &c.Author.Downvotes, &email, &c.Cred, &c.Score, &c.Rank)
			}
		} else {
			if hasVotes {
				e = rows.Scan(&c.ID, &c.PostID, &userId, &c.ReplyID, &c.Content, &c.CreatedAt, &c.Upvotes, &c.Downvotes, &c.ReplyCount, &c.Trashed, &c.Edited, &c.IsReview,
					&c.Author.ID, &c.Author.Name, &c.Author.RegisterDate, &c.Author.Upvotes, &c.Author.Downvotes, &email, &up, &down, &c.Cred, &c.Score)
			} else {
				e = rows.Scan(&c.ID, &c.PostID, &userId, &c.ReplyID, &c.Content, &c.CreatedAt, &c.Upvotes, &c.Downvotes, &c.ReplyCount, &c.Trashed, &c.Edited, &c.IsReview,
					&c.Author.ID, &c.Author.Name, &c.Author.RegisterDate, &c.Author.Upvotes, &c.Author.Downvotes, &email, &c.Cred, &c.Score)
			}
		}
		c.Author.IsAgent = len(email) == 0

		if DidFail(e, "scan comment") {
			continue
		}
		result = append(result, c)
	}
	return result
}

func DBCreateComment(db *sql.DB, userId int64, content string, postId int64, replyId int64, isReview bool) (Comment, UserCont) {
	t := utc()
	nowTime := formatTime(t)

	insertComment := fmt.Sprintf(`
	INSERT INTO Comments(id, userId, postId, replyId, content, createdAt, isReview) 
	VALUES(nextval('comments_id_seq') * 10000 + extract(year from now() at time zone ('utc')), %d, %d, %d, $1, $2, $3) 
	RETURNING %s`, userId, postId, replyId, SQLFieldsForComment())
	row := db.QueryRow(insertComment, content, nowTime, isReview)
	comment, e := ScanComment(row)
	if DidFail(e, "insert comment") {
		return comment, UserCont{}
	}

	updateReplyCount := ""
	if replyId > 0 {
		updateReplyCount = fmt.Sprintf(`
		UPDATE Comments 
		SET replyCount = replyCount + 1 
		WHERE id = %d;
		`, replyId)
	}
	updateCommentCount := fmt.Sprintf(`
	UPDATE Posts
	SET commentCount = commentCount + 1
	WHERE id = %d;
	`, postId)

	insertCommentForUser := fmt.Sprintf(`
	%s
	%s
	INSERT INTO UserCont(uid, pid, sid) 
	VALUES(%d, %d, %d) 
	RETURNING %s`, updateReplyCount, updateCommentCount, userId, postId, comment.ID, SQLFieldsForUserCont())
	row = db.QueryRow(insertCommentForUser)
	userCont, e := ScanUserCont(row)
	if DidFail(e, "insert comment ", comment.ID, " for user pref", userId) {
		return comment, userCont
	}

	return comment, userCont
}

func DBDeleteComment(db *sql.DB, postId int64, commentId int64) {
	deleteFromPostComments := `
	UPDATE Comments 
	SET trashed=true 
	WHERE id=$1 
	RETURNING userId, replyId`
	row := db.QueryRow(deleteFromPostComments, commentId)
	var userId int64
	var replyId int64
	e := row.Scan(&userId, &replyId)
	if DidFail(e, "delete comment ", commentId, " from post ", postId, " comments table") {
		return
	}

	if replyId > 0 {
		updateReplyCount := fmt.Sprintf(`
		UPDATE Comments
		SET replyCount = replyCount - 1
		WHERE id = %d;
		`, replyId)
		_, e = db.Exec(updateReplyCount)
		DidFail(e, "delete comment ", commentId, " for post ", postId, " for user ", userId)
	}
}

func DBUpdateComment(db *sql.DB, postId int64, commentId int64, content string) Comment {
	updateFromPostComments := fmt.Sprintf(`UPDATE comments SET content=$1, Edited=true WHERE id=$2 AND postId=$3
	RETURNING %s`, SQLFieldsForComment())
	row := db.QueryRow(updateFromPostComments, content, commentId, postId)
	comment, e := ScanComment(row)
	if DidFail(e, "update comment ", commentId, " from post ", postId, " comments table") {
		return Comment{}
	}
	return comment
}

func DBVoteComment(db *sql.DB, userId int64, postId int64, commentId int64, upvoteAmount int64, location []string) (Comment, UserProfile, []UserPref) {
	var updateField string
	isUpvote := upvoteAmount > 0
	if isUpvote {
		updateField = "upvotes"
	} else {
		updateField = "downvotes"
		upvoteAmount = -upvoteAmount
	}
	locIndex := DBCreateLocation(db, location)
	voteQuery := SQLMakeVote(upComment, postId, commentId, locIndex, upvoteAmount, isUpvote)
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

	user, uPref := DBVoteForUser(db, userId, comment.UserID, upvoteAmount*sign(isUpvote), location)
	cPref := DBCreateUserPref(db, userId, upComment, postId, commentId, upvoteAmount*sign(isUpvote))

	return comment, user, []UserPref{uPref, cPref}
}

func DBGetComment(db *sql.DB, postId int64, commentId int64) CommentResult {
	getComment := fmt.Sprintf(`SELECT %s FROM Comments p JOIN users u ON p.userId = u.id WHERE p.id = %d`, SQLFieldsForCommentResult(), commentId)
	row := db.QueryRow(getComment)
	comment, e := ScanCommentResult(row)
	if DidFail(e, "get comment ", commentId, " for post ", postId) {
		return CommentResult{}
	}
	return comment
}

// ignore postId if 0, ignore userId if 0, ignore replyId if less than 0, start < CreatedAt < end ignore if empty, ignore upvotes if 0
func DBGetComments(db *sql.DB, postId int64, userId int64, replyId int64, isReview int8,
	start string, end string, popularIn []string, upvotes int64, downvotes int64,
	sortOrder SortOrder, limit int64, offset int64,
	startCreated string, endCreated string, forUser int64, search string) []CommentResult {
	voteTable := "p"
	usingVotesTable := len(popularIn) > 0 || len(start) > 0 || len(end) > 0
	if usingVotesTable {
		voteTable = "v"
	}

	joins := "JOIN users u ON p.userId = u.id\n"
	cond := []string{"p.trashed=false"}
	if postId > 0 {
		cond = append(cond, fmt.Sprintf("postId = %d\n", postId))
	}
	if userId > 0 {
		cond = append(cond, fmt.Sprintf("p.userId = %d\n", userId))
	}
	if replyId >= 0 {
		cond = append(cond, fmt.Sprintf("p.replyId = %d\n", replyId))
	}
	if len(startCreated) > 0 && len(endCreated) > 0 {
		cond = append(cond, fmt.Sprintf("p.createdAt BETWEEN (TIMESTAMP '%s') AND (TIMESTAMP '%s')\n", startCreated, endCreated))
	} else if len(startCreated) > 0 {
		cond = append(cond, fmt.Sprintf("(TIMESTAMP '%s') < p.createdAt\n", startCreated))
	} else if len(endCreated) > 0 {
		cond = append(cond, fmt.Sprintf("p.createdAt < (TIMESTAMP '%s')\n", endCreated))
	}
	if isReview == 1 {
		cond = append(cond, "p.isReview=true")
	} else if isReview == 0 {
		cond = append(cond, "p.isReview=false")
	}

	if usingVotesTable {
		joins += "JOIN Votes v ON v.pid = p.postId AND v.sid = p.id\n"
		cond = append(cond, "kind=2")
	}

	locArray := ""
	if len(popularIn) > 0 {
		locArray = DBGetLocationIndex(db, popularIn)
	}
	getComments := SQLGetItems(db, "Comments p", voteTable, SQLFieldsForCommentResultAlias(),
		SQLFieldsForCommentResult(), joins, locArray, cond, usingVotesTable,
		upvotes, downvotes,
		sortOrder, limit, offset, start, end, forUser, search)

	rows, e := db.Query(getComments)
	if DidFail(e, "failed to get comments", getComments) {
		return []CommentResult{}
	}

	result := ScanCommentResults(rows, usingVotesTable, len(search) > 0 && sortOrder == soRank)
	return result
}
