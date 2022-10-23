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

type FlaggedContent struct {
	Content   PostResult
	Kind      FlagReason
	Reason    string
	CreatedAt time.Time
}

func SQLFieldsForFlaggedPost() string {
	return "p.id, p.userId, p.content, p.tags, p.createdAt, p.location, p.upvotes AS item_up, p.downvotes AS item_down, u.id, u.name, u.registerDate, u.upvotes, u.downvotes, f.sid, f.kind, f.reason, f.createdAt"
}

func ScanFlaggedPosts(rows *sql.Rows) []FlaggedContent {
	result := []FlaggedContent{}
	var e error
	for rows.Next() {
		p := PostResult{}
		u := UserProfile{}
		var commentId int64
		var kind FlagReason
		var createdAt time.Time
		var userId int64
		var reason string
		e = rows.Scan(&p.ID, &userId, &p.Content, pq.Array(&p.Tags), &p.CreatedAt,
			pq.Array(&p.Location), &p.Upvotes, &p.Downvotes, &u.ID, &u.Name, &u.RegisterDate,
			&u.Upvotes, &u.Upvotes, &commentId, &kind, &reason, &createdAt)
		if DidFail(e, "scan user pref post") {
			continue
		}
		p.Author = u
		result = append(result, FlaggedContent{p, kind, reason, createdAt})
	}
	return result
}

func DBCreateFlag(db *sql.DB, uid int64, pid int64, sid int64, kind FlagReason, reason string) {
	insertFlag := `
	INSERT INTO Flags(uid, pid, sid, kind, reason)
	VALUES($1, $2, $3, $4, $5);
	`
	_, e := db.Exec(insertFlag, uid, pid, sid, kind, reason)

	if DidFail(e, "create flag") {
		return
	}
}

func DBGetFlags(db *sql.DB, kind FlagReason, limit int64, offset int64) []FlaggedContent {
	getPosts := fmt.Sprintf(`
	SELECT %s
	FROM Flags f 
	JOIN posts p ON f.pid = p.id
	JOIN users u ON p.userId = u.id
	WHERE f.kind = %d
	ORDER BY f.createdAt
	`, SQLFieldsForFlaggedPost(), kind)

	getPosts += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	rows, e := db.Query(getPosts)
	if DidFail(e, "get flagged posts") {
		return []FlaggedContent{}
	}
	defer rows.Close()

	result := ScanFlaggedPosts(rows)
	return result
}

func DBHandleFlag(db *sql.DB, id int64, pid int64, sid int64, action string) {
	switch action {
	case "ignore":
		DBDeleteFlag(db, id)
	case "remove":
		DBRemoveFlagContent(db, id, pid, sid)
	case "report":
		DBReportFlagContent(db, id, pid, sid)
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

func DBRemoveFlagContent(db *sql.DB, id int64, pid int64, sid int64) {
	if sid > 0 {
		DBDeleteComment(db, pid, sid)
	} else {
		DBDeletePost(db, pid)
	}
	DBDeleteFlag(db, id)
}

func DBReportFlagContent(db *sql.DB, id int64, pid int64, sid int64) {
	// FIXME: Report to appropriate authorities
	DBRemoveFlagContent(db, id, pid, sid)
}
