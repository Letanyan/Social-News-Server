package main

import (
	"database/sql"
	"fmt"
	"strings"
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

func SQLFieldsForCommentResultAlias() string {
	return "p.id, p.postId, p.userId, p.replyId, p.content, p.createdAt, p.updatedAt, p.upvotes AS item_up, p.downvotes AS item_down, u.id, u.name, u.registerDate, u.upvotes, u.downvotes"
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
			e = rows.Scan(&c.ID, &c.PostID, &userId, &c.ReplyID, &c.Content, &c.CreatedAt, &c.UpdatedAt, &c.Upvotes, &c.Downvotes,
				&c.Author.ID, &c.Author.Name, &c.Author.RegisterDate, &c.Author.Upvotes, &c.Author.Downvotes, &up, &down, &cred, &score)
		} else {
			e = rows.Scan(&c.ID, &c.PostID, &userId, &c.ReplyID, &c.Content, &c.CreatedAt, &c.UpdatedAt, &c.Upvotes, &c.Downvotes,
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
	INSERT INTO Comments(id, userId, postId, replyId, content, createdAt, updatedAt) 
	VALUES(nextval('comments_id_seq') * 10000 + extract(year from now() at time zone ('utc')), %d, $3, %d, $1, $2, $2) 
	RETURNING %s`, userId, replyId, SQLFieldsForComment())
	row := db.QueryRow(insertComment, content, nowTime, postId)
	comment, e := ScanComment(row)
	if DidFail(e, "insert comment") {
		return comment, UserCont{}
	}

	insertCommentForUser := fmt.Sprintf(`
	INSERT INTO User%dCont(postId, commentId) 
	VALUES(%d, %d) 
	RETURNING %s`, userId, postId, comment.ID, SQLFieldsForUserCont())
	row = db.QueryRow(insertCommentForUser)
	userCont, e := ScanUserCont(row)
	if DidFail(e, "insert comment ", comment.ID, " for user pref", userId) {
		return comment, userCont
	}

	return comment, userCont
}

func DBDeleteComment(db *sql.DB, postId int64, commentId int64) {
	deleteFromPostComments := `DELETE FROM comments WHERE id=$1 RETURNING userId`
	row := db.QueryRow(deleteFromPostComments, commentId)
	var userId int64
	e := row.Scan(&userId)
	if DidFail(e, "delete comment ", commentId, " from post ", postId, " comments table") {
		return
	}

	deletePostFromUser := fmt.Sprintf(`DELETE FROM User%dCont WHERE postId=$1 AND commentId=$2`, userId)
	_, e = db.Exec(deletePostFromUser, postId, commentId)
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

func DBVoteComment(db *sql.DB, userId int64, postId int64, commentId int64, upvoteAmount int64, location []string) (Comment, UserProfile, []UserPref) {
	currentTime := utc()
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
	locArray := SQLFormattedArray(location)
	updateVoteForPost := fmt.Sprintf(`
		INSERT INTO votes(kind, pid, sid, location) VALUES(2, %d, %d, %s)
		ON CONFLICT (kind, pid, sid, location) DO NOTHING;
		UPDATE votes SET
		%s = cooldown(%s, updatedAt, '%s', 31536000) + %d,
		%s = cooldown(%s, updatedAt, '%s', 31536000),
		updatedAt = '%s'
		WHERE kind=2 AND pid=%d AND sid=%d AND location=%s;

		UPDATE Comments SET 
		%s = cooldown(%s, updatedAt, '%s', 31536000) + %d,
		%s = cooldown(%s, updatedAt, '%s', 31536000),
		updatedAt = '%s'
		WHERE id = %d
		RETURNING %s
		`, postId, commentId, locArray,
		updateField, updateField, nowTime, upvoteAmount,
		otherField, otherField, nowTime, nowTime, postId, commentId, locArray,
		updateField, updateField, nowTime, upvoteAmount,
		otherField, otherField, nowTime, nowTime, commentId, SQLFieldsForComment())
	row := db.QueryRow(updateVoteForPost)
	comment, e := ScanComment(row)
	if DidFail(e, "vote for post ", postId) {
		return Comment{}, UserProfile{}, []UserPref{}
	}

	user, uPref := DBVoteForUser(db, userId, comment.UserID, upvoteAmount*sign(isUpvote), location)
	createPref := fmt.Sprintf(`
	INSERT INTO User%dPref (kind, pid, sid) 
	VALUES(2, %d, %d) ON CONFLICT (kind, pid, sid) DO NOTHING;
	UPDATE User%dPref SET 
	%s = cooldown(%s, updatedAt, '%s', 31536000) + %d,
	%s = cooldown(%s, updatedAt, '%s', 31536000),
	updatedAt = '%s'
	WHERE kind=2 AND pid=%d AND sid=%d
	RETURNING %s
	`, userId, postId, commentId, userId,
		updateField, updateField, nowTime, upvoteAmount,
		otherField, otherField, nowTime, nowTime, postId, commentId, SQLFieldsForUserPref())
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
func DBGetComments(db *sql.DB, postId int64, userId int64, replyId int64, start string, end string, popularIn []string, upvotes int64, downvotes int64, sortOrder SortOrder, limit int64, offset int64) []CommentResult {
	voteTable := "p"
	if len(popularIn) > 0 {
		voteTable = "v"
	}
	getComments := fmt.Sprintf(`SELECT %s{agg}, RATIO(p.upvotes, p.downvotes) AS cred, p.upvotes * RATIO(p.upvotes, p.downvotes) AS score
	FROM Comments p 
	JOIN users u ON p.userId = u.id
	`, SQLFieldsForCommentResultAlias())

	cond := fmt.Sprintf("WHERE postId = %d\n", postId)
	if userId != 0 {
		cond += fmt.Sprintf("AND p.userId = %d\n", userId)
	}
	if replyId != 0 {
		cond += fmt.Sprintf("AND p.replyId = %d\n", replyId)
	}
	if len(start) > 0 && len(end) > 0 {
		cond += fmt.Sprintf("AND p.createdAt BETWEEN (TIMESTAMP '%s') AND (TIMESTAMP '%s')\n", start, end)
	} else if len(start) > 0 {
		cond += fmt.Sprintf("AND (TIMESTAMP '%s') < p.createdAt\n", start)
	} else if len(end) > 0 {
		cond += fmt.Sprintf("AND p.createdAt < (TIMESTAMP '%s')\n", end)
	}
	if upvotes != 0 {
		if upvotes > 0 {
			cond += fmt.Sprintf("AND %s.upvotes > %d\n", voteTable, upvotes)
		} else {
			cond += fmt.Sprintf("AND %s.upvotes < %d\n", voteTable, -upvotes)
		}
	}
	if downvotes != 0 {
		if downvotes > 0 {
			cond += fmt.Sprintf("AND %s.downvotes > %d\n", voteTable, downvotes)
		} else {
			cond += fmt.Sprintf("AND %s.downvotes < %d\n", voteTable, -downvotes)
		}
	}
	if len(popularIn) > 0 {
		queryLoc := SQLFormattedArray(popularIn)
		cond += fmt.Sprintf("AND v.kind=2 AND v.location @> %s\n", queryLoc)
		getComments += "JOIN votes v ON v.pid = p.id\n"
	}

	getComments += cond + "\n"
	if len(popularIn) > 0 {
		getComments += fmt.Sprintf("GROUP BY %s, cred, score\n", SQLFieldsForPostResult())
		getComments = strings.ReplaceAll(getComments, "{agg}", ", SUM(v.upvotes) AS sec_up, SUM(v.downvotes) AS sec_down")
	} else {
		getComments = strings.ReplaceAll(getComments, "{agg}", "")
	}

	getComments += SQLSortOrder(sortOrder)
	getComments += fmt.Sprintf("LIMIT %d OFFSET %d\n", limit, offset)

	rows, e := db.Query(getComments)
	if DidFail(e, "failed to get comments") {
		return []CommentResult{}
	}

	result := ScanCommentResults(rows, len(popularIn) > 0)
	return result
}
