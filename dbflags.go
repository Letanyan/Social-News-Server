package main

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
)

type FlagReason int

const (
	frSexual FlagReason = iota
	frViolent
	frHateful
	frHarassment
	frHarmful
	frAbuse
	frSpam
	frOther
)

type FlaggedPost struct {
	ID        int64
	Content   PostResult
	Kind      FlagReason
	Reason    string
	CreatedAt time.Time
}

type FlaggedComment struct {
	ID        int64
	Content   CommentResult
	Kind      FlagReason
	Reason    string
	CreatedAt time.Time
}

func SQLFieldsForFlaggedPost() string {
	return "p.id, p.userId, p.content, p.tags, p.createdAt, p.location, p.upvotes AS item_up, p.downvotes AS item_down, p.flagCount, u.id, u.name, u.registerDate, u.upvotes, u.downvotes, u.email, f.id, f.kind, f.reason, f.createdAt"
}

func SQLFieldsForFlaggedComment() string {
	return "p.id, p.postId, p.userId, p.replyId, p.content, p.createdAt, p.upvotes, p.downvotes, p.replyCount, p.flagCount, u.id, u.name, u.registerDate, u.upvotes, u.downvotes, u.email, f.id, f.kind, f.reason, f.createdAt"
}

func ScanFlaggedPosts(rows *sql.Rows) []FlaggedPost {
	result := []FlaggedPost{}
	var e error
	defer rows.Close()
	for rows.Next() {
		p := PostResult{}
		u := UserProfile{}
		var kind FlagReason
		var createdAt time.Time
		var userId int64
		var reason string
		var id int64
		var email string
		e = rows.Scan(&p.ID, &userId, &p.Content, pq.Array(&p.Tags), &p.CreatedAt,
			pq.Array(&p.Location), &p.Upvotes, &p.Downvotes, &p.FlagCount, &u.ID, &u.Name, &u.RegisterDate,
			&u.Upvotes, &u.Upvotes, &email, &id, &kind, &reason, &createdAt)
		if DidFail(e, "scan user pref post") {
			continue
		}
		u.IsAgent = len(email) == 0
		p.Author = u
		result = append(result, FlaggedPost{id, p, kind, reason, createdAt})
	}
	return result
}

func ScanFlaggedComments(rows *sql.Rows) []FlaggedComment {
	result := []FlaggedComment{}
	var e error
	defer rows.Close()
	for rows.Next() {
		p := CommentResult{}
		u := UserProfile{}
		var kind FlagReason
		var createdAt time.Time
		var userId int64
		var reason string
		var id int64
		var email string
		e = rows.Scan(&p.ID, &p.PostID, &userId, &p.ReplyID, &p.Content, &p.CreatedAt, &p.Upvotes, &p.Downvotes, &p.ReplyCount, &p.FlagCount,
			&u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Upvotes,
			&email, &id, &kind, &reason, &createdAt)
		if DidFail(e, "scan user pref post") {
			continue
		}
		u.IsAgent = len(email) == 0
		p.Author = u
		result = append(result, FlaggedComment{id, p, kind, reason, createdAt})
	}
	return result
}

func DBCreateFlag(db *sql.DB, uid int64, pid int64, sid int64, kind FlagReason, reason string) {
	updateFlag := ""
	if sid <= 0 {
		post := DBGetPost(db, pid)
		updateFlag = fmt.Sprintf(`
		UPDATE Posts
		SET flagCount = flagCount + 1
		WHERE id=%d
		`, post.ID)
	} else {
		comment := DBGetComment(db, pid, sid)
		updateFlag = fmt.Sprintf(`
		UPDATE PostComments
		SET flagCount = flagCount + 1
		WHERE postId=%d AND id=%d
		`, comment.PostID, comment.ID)
	}

	insertFlag := fmt.Sprintf(`
	INSERT INTO Flags(uid, pid, sid, kind, reason)
	VALUES(%d, %d, %d, %d, '%s');
	%s
	`, uid, pid, sid, kind, reason, updateFlag)
	_, e := db.Exec(insertFlag)

	if DidFail(e, "create flag", insertFlag) {
		return
	}
}

func DBGetFlaggedPosts(db *sql.DB, kind FlagReason, limit int64, offset int64) []FlaggedPost {
	getPosts := fmt.Sprintf(`
	SELECT %s
	FROM Flags f 
	JOIN posts p ON f.pid = p.id
	JOIN users u ON p.userId = u.id
	WHERE f.kind = %d AND f.sid < 0
	ORDER BY p.flagCount DESC
	`, SQLFieldsForFlaggedPost(), kind)

	getPosts += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	rows, e := db.Query(getPosts)
	if DidFail(e, "get flagged posts") {
		return []FlaggedPost{}
	}

	result := ScanFlaggedPosts(rows)
	return result
}

func DBGetFlaggedComments(db *sql.DB, kind FlagReason, limit int64, offset int64) []FlaggedComment {
	getComments := fmt.Sprintf(`
	SELECT %s
	FROM Flags f 
	JOIN PostComments p ON f.pid=p.postId AND f.sid=p.id
	JOIN Users u ON p.userId=u.id
	WHERE f.kind = %d
	ORDER BY p.flagCount DESC
	`, SQLFieldsForFlaggedComment(), kind)

	getComments += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	rows, e := db.Query(getComments)
	if DidFail(e, "get flagged posts") {
		return []FlaggedComment{}
	}

	result := ScanFlaggedComments(rows)
	return result
}

func DBHandleFlag(db *sql.DB, id int64, pid int64, sid int64, action string) {
	switch action {
	case "ignore":
		DBDeleteFlag(db, id)
	case "ignore_all":
		DBIgnoreFlagContent(db, pid, sid)
	case "remove":
		DBRemoveFlagContent(db, id, pid, sid)
	case "report":
		DBReportFlagContent(db, id, pid, sid)
	case "block1":
		DBBlockFlagUser(db, id, pid, sid, 1)
	case "block2":
		DBBlockFlagUser(db, id, pid, sid, 2)
	case "block7":
		DBBlockFlagUser(db, id, pid, sid, 7)
	case "block14":
		DBBlockFlagUser(db, id, pid, sid, 14)
	case "block21":
		DBBlockFlagUser(db, id, pid, sid, 21)
	case "block28":
		DBBlockFlagUser(db, id, pid, sid, 28)
	case "perm":
		DBBlockFlagUser(db, id, pid, sid, 36500)
	}
}

func DBDeleteFlag(db *sql.DB, id int64) {
	action := `
	DELETE FROM Flags WHERE id = $1
	`
	_, e := db.Exec(action, id)
	if DidFail(e, "delete flag") {
		return
	}
}

func DBIgnoreFlagContent(db *sql.DB, pid int64, sid int64) {
	updateFlag := ""
	// arbitrarily set flagCount to -1000 as a buffer
	if sid <= 0 {
		updateFlag = fmt.Sprintf(`
		UPDATE Posts SET flagCount=%d WHERE id=%d
		`, -1000, pid)
	} else {
		updateFlag = fmt.Sprintf(`
		UPDATE PostComments SET flagCount=%d WHERE postId=%d AND id=%d 
		`, -1000, pid, sid)
	}

	action := fmt.Sprintf(`
	DELETE FROM Flags WHERE pid=%d AND sid=%d;
	%s
	`, pid, sid, updateFlag)
	_, e := db.Exec(action)
	if DidFail(e, "delete flag") {
		return
	}
}

func DBRemoveFlagContent(db *sql.DB, id int64, pid int64, sid int64) {
	if sid > 0 {
		DBDeleteComment(db, pid, sid)
	} else {
		DBDeletePost(db, pid)
	}
	DBDeleteFlag(db, id)
}

func DBBlockFlagUser(db *sql.DB, id int64, pid int64, sid int64, duration int) {
	if sid > 0 {
		p := DBGetComment(db, pid, sid)
		DBBlockUser(db, p.Author.ID, duration)
	} else {
		p := DBGetPost(db, pid)
		DBBlockUser(db, p.Author.ID, duration)
	}
	DBRemoveFlagContent(db, id, pid, sid)
}

func DBReportFlagContent(db *sql.DB, id int64, pid int64, sid int64) {
	// FIXME: Report to appropriate authorities
	DBRemoveFlagContent(db, id, pid, sid)
}
