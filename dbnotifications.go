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

func DBGetCommentNotifications(db *sql.DB, userId int64,
	postId int64, replyId int64, isReview int8,
	upvotes int64, downvotes int64, sortOrder SortOrder, limit int64, offset int64,
	startCreated string, endCreated string, search string) []CommentResult {

	cond := []string{"n.userId=$1", "p.trashed=false"}
	if postId > 0 {
		cond = append(cond, fmt.Sprintf("postId = %d\n", postId))
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
	if upvotes > 0 {
		cond = append(cond, fmt.Sprintf("p.upvotes >= %d", upvotes))
	} else if upvotes < 0 {
		cond = append(cond, fmt.Sprintf("p.upvotes < %d", -upvotes))
	}
	if downvotes > 0 {
		cond = append(cond, fmt.Sprintf("p.downvotes >= %d", downvotes))
	} else if downvotes < 0 {
		cond = append(cond, fmt.Sprintf("p.downvotes < %d", -downvotes))
	}
	rankString := ""
	if len(search) > 0 {
		search, _ := DBPrepareSearchString(db, search)
		if len(search) > 0 {
			cond = append(cond, fmt.Sprintf("p.ContentLocaleVector @@ to_tsquery('english', '%s')", search))
		}
		if sortOrder == soRank {
			rankString = fmt.Sprintf(", ts_rank(p.ContentLocaleVector, to_tsquery('english', '%s')) AS rank", search)
		}
	}

	query := fmt.Sprintf(`
	SELECT %s, 
		Ratio(p.upvotes, p.downvotes) AS cred, 
		WeightRatio(p.upvotes, p.upvotes, p.downvotes) AS score%s
	FROM PostComments p
	JOIN CommentNotifications n ON p.id = n.commentId
	JOIN Users u ON u.id = p.userId
	WHERE %s
	ORDER BY n.id DESC
	LIMIT $2 OFFSET $3
	`, SQLFieldsForCommentResultAlias(), rankString, JoinStrings(cond, " AND "))
	rows, e := db.Query(query, userId, limit, offset)
	if DidFail(e, "get comment notifications") {
		return []CommentResult{}
	}
	result := ScanCommentResults(rows, false, false)
	return result
}
