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
	existingFlags := `
	SELECT COUNT(*)
	FROM Flags
	WHERE uid=$1 AND pid=$2 AND sid=$3
	`
	row := db.QueryRow(existingFlags, uid, pid, sid)
	var flagCount int64
	e := row.Scan(&flagCount)
	if DidFail(e, "get flag count for", uid, pid, sid) {
		flagCount = 0
	}

	updateFlag := ""
	// increment flag count only for unique user
	if flagCount <= 0 {
		if sid <= 0 {
			post := DBGetPost(db, pid)
			updateFlag = fmt.Sprintf(`
			UPDATE Posts p
			SET flagCount = flagCount + CEIL(weightRatio(u.judge, u.judge, u.jury))
			FROM Users u
			WHERE u.id=%d AND p.id=%d
			`, uid, post.ID)
		} else {
			comment := DBGetComment(db, pid, sid)
			updateFlag = fmt.Sprintf(`
			UPDATE PostComments p
			SET flagCount = flagCount + CEIL(weightRatio(u.judge, u.judge, u.jury))
			FROM Users u
			WHERE u.id=%d AND postId=%d AND p.id=%d
			`, uid, comment.PostID, comment.ID)
		}
	}

	insertFlag := fmt.Sprintf(`
	INSERT INTO Flags(uid, pid, sid, kind, reason)
	VALUES(%d, %d, %d, %d, '%s');
	%s
	`, uid, pid, sid, kind, reason, updateFlag)
	_, e = db.Exec(insertFlag)

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

func DBGetFlagsForContent(db *sql.DB, pid int64, sid int64) map[FlagReason]int {
	result := map[FlagReason]int{}

	query := `
	SELECT f.kind, COUNT(*)
	FROM Flags f
	WHERE pid=$1 AND sid=$2
	GROUP BY f.kind
	`
	rows, e := db.Query(query, pid, sid)
	if DidFail(e, "get flags for post") {
		return result
	}
	defer rows.Close()
	var kind FlagReason
	var count int
	for rows.Next() {
		rows.Scan(&kind, &count)
		result[kind] = count
	}

	return result
}

func DBHandleFlag(db *sql.DB, id int64, pid int64, sid int64, action string) {
	switch action {
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

func DBDeleteFlags(db *sql.DB, pid int64, sid int64) {
	action := `
	DELETE FROM Flags WHERE pid = $1 AND sid = $2
	`
	_, e := db.Exec(action, pid, sid)
	if DidFail(e, "delete flag") {
		return
	}
}

func DBIgnoreFlagContent(db *sql.DB, pid int64, sid int64) {
	DBUpdateUsersAuthority(db, pid, sid, false)
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
	DBUpdateUsersAuthority(db, pid, sid, true)
	if sid > 0 {
		DBDeleteComment(db, pid, sid)
	} else {
		DBDeletePost(db, pid)
	}
	DBDeleteFlags(db, pid, sid)
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

func DBUpdateUsersAuthority(db *sql.DB, pid int64, sid int64, correctDecision bool) {
	updateJudge := "jury = jury + 1"
	if correctDecision {
		updateJudge = "judge = judge + 1"
	}
	query := fmt.Sprintf(`
	UPDATE Users AS u
	SET %s
	FROM Flags f
	WHERE f.pid = $1 AND f.sid = $2 AND f.uid = u.id
	`, updateJudge)

	_, e := db.Exec(query, pid, sid)

	DidFail(e, "update user judge")
}
