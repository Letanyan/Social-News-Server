package main

import (
	"database/sql"
	"fmt"
)

func DBAddCommentNotification(db *sql.DB, userId int64, commentId int64) {
	query := `
	INSERT INTO CommentNotifications (userId, commentId, viewedAt)
	VALUES ($1, $2, NULL);
	`
	_, e := db.Exec(query, userId, commentId)
	DidFail(e, "insert new notification")
}

func DBReadCommentNotification(db *sql.DB, id int64) {
	query := `
	UPDATE CommentNotifications 
	SET viewedAt = (now() at time zone 'utc')
	WHERE id=$1
	`
	_, e := db.Exec(query, id)
	DidFail(e, "update notification viewed at date")
}

func DBGetCommentNotifications(db *sql.DB, userId int64, limit int64, offset int64) []CommentResult {
	query := fmt.Sprintf(`
	SELECT %s, 
		Ratio(p.upvotes, p.downvotes) AS cred, 
		WeightRatio(p.upvotes, p.upvotes, p.downvotes) AS score
	FROM PostComments p
	JOIN CommentNotifications n ON p.id = n.commentId
	JOIN Users u ON u.id = p.userId
	WHERE n.userId = $1
	ORDER BY n.id DESC
	LIMIT $2 OFFSET $3
	`, SQLFieldsForCommentResultAlias())
	rows, e := db.Query(query, userId, limit, offset)
	if DidFail(e, "get comment notifications") {
		return []CommentResult{}
	}
	result := ScanCommentResults(rows, false, false)
	return result
}
