package main

import (
	"database/sql"
	"fmt"

	"github.com/lib/pq"
)

type UserPrefKind int

const (
	upUser UserPrefKind = iota + 1
	upComment
	upPost
	upTag
)

type UserPref struct {
	Kind      UserPrefKind
	PID       int64
	SID       int64
	Upvotes   float64
	Downvotes float64
}

type UserPrefUser struct {
	User      UserProfile
	Upvotes   float64
	Downvotes float64
}

type UserPrefPost struct {
	Post      PostResult
	CommentID int64
	Upvotes   float64
	Downvotes float64
}

type UserPrefTag struct {
	Tag       Tag
	Upvotes   float64
	Downvotes float64
}

func SQLFieldsForUserPref() string {
	return "kind, pid, sid, upvotes, downvotes"
}

func SQLFieldsForUserPrefUser() string {
	return "u.id, u.name, u.registerDate, u.upvotes AS item_up, u.downvotes AS item_down, up.upvotes AS sec_up, up.downvotes AS sec_down"
}

func SQLFieldsForUserPrefPost() string {
	return "p.id, p.userId, p.content, p.tags, p.createdAt, p.updatedAt, p.location, p.upvotes AS item_up, p.downvotes AS item_down, u.id, u.name, u.registerDate, u.upvotes, u.downvotes, up.sid, up.upvotes AS sec_up, up.downvotes AS sec_down"
}

func SQLFieldsForUserPrefTag() string {
	return "t.id, t.name, t.updatedAt, t.location, t.upvotes AS item_up, t.downvotes AS item_down, up.upvotes AS sec_up, up.downvotes AS sec_down"
}

func ScanUserPrefRow(row *sql.Row) (UserPref, error) {
	up := UserPref{}
	e := row.Scan(&up.Kind, &up.PID, &up.SID, &up.Upvotes, &up.Downvotes)
	return up, e
}

func ScanUserPrefRows(rows *sql.Rows) []UserPref {
	result := []UserPref{}
	var cred float64
	var score float64
	var e error
	for rows.Next() {
		up := UserPref{}
		e = rows.Scan(&up.Kind, &up.PID, &up.SID, &up.Upvotes, &up.Downvotes, &cred, &score)
		if DidFail(e, "scan user pref") {
			continue
		}
		result = append(result, up)
	}
	return result
}

func ScanUserPrefUsers(rows *sql.Rows) []UserPrefUser {
	result := []UserPrefUser{}
	var cred float64
	var score float64
	var e error
	for rows.Next() {
		u := UserProfile{}
		var up float64
		var down float64
		e = rows.Scan(&u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Downvotes, &up, &down, &cred, &score)
		if DidFail(e, "scan user pref users") {
			continue
		}
		result = append(result, UserPrefUser{u, up, down})
	}
	return result
}

func ScanUserPrefPosts(rows *sql.Rows) []UserPrefPost {
	result := []UserPrefPost{}
	var cred float64
	var score float64
	var e error
	for rows.Next() {
		p := PostResult{}
		u := UserProfile{}
		var commentId int64
		var up float64
		var down float64
		var userId int64
		e = rows.Scan(&p.ID, &userId, &p.Content, pq.Array(&p.Tags), &p.CreatedAt, &p.UpdatedAt,
			pq.Array(&p.Location), &p.Upvotes, &p.Downvotes, &u.ID, &u.Name, &u.RegisterDate,
			&u.Upvotes, &u.Upvotes, &commentId, &up, &down, &cred, &score)
		if DidFail(e, "scan user pref post") {
			continue
		}
		p.Author = u
		result = append(result, UserPrefPost{p, int64(commentId), up, down})
	}
	return result
}

func ScanUserPrefTags(rows *sql.Rows) []UserPrefTag {
	result := []UserPrefTag{}
	var cred float64
	var score float64
	var e error
	for rows.Next() {
		tag := Tag{}
		var up float64
		var down float64
		e = rows.Scan(&tag.ID, &tag.Name, &tag.UpdatedAt, pq.Array(&tag.Location),
			&tag.Upvotes, &tag.Downvotes, &up, &down, &cred, &score)
		if DidFail(e, "scan user pref tag") {
			continue
		}
		result = append(result, UserPrefTag{tag, up, down})
	}
	return result
}

// ignore kind if it equals 0. ignore pid if it equals 0. ignore sid if it equals 0
func DBGetUserPref(db *sql.DB, userId int64, sortOrder SortOrder, kind UserPrefKind, pid int64, sid int64, upvotes int64, downvotes int64, limit int64, offset int64) []UserPref {
	query := fmt.Sprintf(`SELECT %s, RATIO(upvotes, downvotes) AS cred, upvotes * RATIO(upvotes, downvotes) AS score 
	FROM User%dPref`, SQLFieldsForUserPref(), userId)

	cond := ""
	if kind > 0 {
		cond = fmt.Sprintf(" WHERE kind = %d", kind)
	}
	if pid > 0 {
		if len(cond) == 0 {
			cond += " WHERE "
		} else {
			cond += " AND "
		}
		cond += fmt.Sprintf("pid = %d", pid)
	}
	if sid > 0 {
		if len(cond) == 0 {
			cond += " WHERE "
		} else {
			cond += " AND "
		}
		cond += fmt.Sprintf("sid = %d", sid)
	}
	if upvotes != 0 {
		if len(cond) == 0 {
			cond += " WHERE "
		} else {
			cond += " AND "
		}
		if upvotes > 0 {
			cond += fmt.Sprintf("upvotes > %d", upvotes)
		} else {
			cond += fmt.Sprintf("upvotes < %d", -upvotes)
		}
	}
	if downvotes != 0 {
		if len(cond) == 0 {
			cond += " WHERE "
		} else {
			cond += " AND "
		}
		if downvotes > 0 {
			cond += fmt.Sprintf("downvotes > %d", downvotes)
		} else {
			cond += fmt.Sprintf("downvotes < %d", -downvotes)
		}
	}

	query += cond + "\n"
	query += SQLSortOrder(sortOrder)

	query += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)
	rows, e := db.Query(query)
	result := []UserPref{}
	if DidFail(e, "select user prefs for ", userId, " of kind ", kind, " pid:", pid, " sid: ", sid) {
		return result
	}
	result = ScanUserPrefRows(rows)

	return result
}

func DBGetUserPrefUsers(db *sql.DB, userId int64, upvoteAmount int64, downvoteAmount int64, upvotes int64, downvotes int64, sortOrder SortOrder, limit int64, offset int64) []UserPrefUser {
	getUsers := fmt.Sprintf(`SELECT %s, RATIO(up.upvotes, up.downvotes) AS cred, up.upvotes * RATIO(up.upvotes, up.downvotes) AS score  
	FROM User%dPref up JOIN users u ON up.pid = u.id WHERE kind = 1
	`, SQLFieldsForUserPrefUser(), userId)

	if upvoteAmount > 0 {
		getUsers += fmt.Sprintf("AND up.upvotes > %d ", upvoteAmount)
	} else if upvoteAmount < 0 {
		getUsers += fmt.Sprintf("AND up.upvotes < %d ", -upvoteAmount)
	}
	if downvoteAmount > 0 {
		getUsers += fmt.Sprintf("AND up.downvotes > %d ", downvoteAmount)
	} else if downvoteAmount < 0 {
		getUsers += fmt.Sprintf("AND up.downvotes < %d ", -downvoteAmount)
	}

	if upvotes > 0 {
		getUsers += fmt.Sprintf("AND u.upvotes > %d ", upvotes)
	} else if upvotes < 0 {
		getUsers += fmt.Sprintf("AND u.upvotes < %d ", -upvotes)
	}
	if downvotes > 0 {
		getUsers += fmt.Sprintf("AND u.downvotes > %d ", downvotes)
	} else if downvotes < 0 {
		getUsers += fmt.Sprintf("AND u.downvotes < %d ", -downvotes)
	}

	getUsers += SQLSortOrder(sortOrder)
	getUsers += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	rows, e := db.Query(getUsers)
	if DidFail(e, "get users") {
		return []UserPrefUser{}
	}

	result := ScanUserPrefUsers(rows)

	return result
}

func DBGetUserPrefPosts(db *sql.DB, userId int64, upvoteAmount int64, downvoteAmount int64, isComment bool, authorId int64, tags []string, location []string, upvotes int64, downvotes int64, sortOrder SortOrder, limit int64, offset int64, startDate string, endDate string) []UserPrefPost {
	kind := upPost
	if isComment {
		kind = upComment
	}
	getPosts := fmt.Sprintf(`SELECT %s, RATIO(up.upvotes, up.downvotes) AS cred, up.upvotes * RATIO(up.upvotes, up.downvotes) AS score 
		FROM user%dpref up 
		JOIN posts{} p ON up.pid = p.id
		JOIN users u ON p.userId = u.id
		WHERE createdAt BETWEEN (TIMESTAMP '%s') AND (TIMESTAMP '%s') AND kind = %d
		`, SQLFieldsForUserPrefPost(), userId, startDate, endDate, kind)

	if upvoteAmount > 0 {
		getPosts += fmt.Sprintf("AND up.upvotes > %d ", upvoteAmount)
	} else if upvoteAmount < 0 {
		getPosts += fmt.Sprintf("AND up.upvotes < %d ", -upvoteAmount)
	}
	if downvoteAmount > 0 {
		getPosts += fmt.Sprintf("AND up.downvotes > %d ", downvoteAmount)
	} else if downvoteAmount < 0 {
		getPosts += fmt.Sprintf("AND up.downvotes < %d ", -downvoteAmount)
	}
	if authorId != 0 {
		getPosts += fmt.Sprintf("AND p.userId = %d ", authorId)
	}
	if len(tags) > 0 {
		queryTags := SQLFormattedArray(tags)
		getPosts += fmt.Sprintf("AND %s && p.tags ", queryTags)
	}
	if len(location) > 0 {
		queryLoc := SQLFormattedArray(location)
		getPosts += fmt.Sprintf("AND p.location @> %s ", queryLoc)
	}
	if upvotes > 0 {
		getPosts += fmt.Sprintf("AND p.upvotes > %d ", upvotes)
	} else if upvotes < 0 {
		getPosts += fmt.Sprintf("AND p.upvotes < %d ", -upvotes)
	}
	if downvotes > 0 {
		getPosts += fmt.Sprintf("AND p.downvotes > %d ", downvotes)
	} else if downvotes < 0 {
		getPosts += fmt.Sprintf("AND p.downvotes < %d ", -downvotes)
	}

	sDate := parseTime(startDate)
	eDate := parseTime(endDate)
	years := yearsBetweenDates(sDate, eDate)
	postQueries := BuildUnionForYears(getPosts, years)

	postQueries += SQLSortOrder(sortOrder)

	postQueries += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	rows, e := db.Query(postQueries)
	if DidFail(e, "get posts") {
		return []UserPrefPost{}
	}
	defer rows.Close()

	result := ScanUserPrefPosts(rows)
	return result
}

func DBGetUserPrefTags(db *sql.DB, userId int64, upvoteAmount int64, downvoteAmount int64, tags []string, location []string, upvotes int64, downvotes int64, sortOrder SortOrder, limit int64, offset int64) []UserPrefTag {
	getTags := fmt.Sprintf(`SELECT %s, RATIO(up.upvotes, up.downvotes) AS cred, up.upvotes * RATIO(up.upvotes, up.downvotes) AS score 
	FROM user%dpref up
	JOIN tags t ON up.pid = t.id
	WHERE kind = 4
	`, SQLFieldsForUserPrefTag(), userId)

	if upvoteAmount > 0 {
		getTags += fmt.Sprintf("AND up.upvotes > %d ", upvoteAmount)
	} else if upvoteAmount < 0 {
		getTags += fmt.Sprintf("AND up.upvotes < %d ", -upvoteAmount)
	}
	if downvoteAmount > 0 {
		getTags += fmt.Sprintf("AND up.downvotes > %d ", downvoteAmount)
	} else if downvoteAmount < 0 {
		getTags += fmt.Sprintf("AND up.downvotes < %d ", -downvoteAmount)
	}
	if len(tags) > 0 {
		tagArray := SQLFormattedArray(tags)
		getTags += fmt.Sprintf("AND ARRAY[t.name] <@ %s\n", tagArray)
	}
	if len(location) > 0 {
		locArray := SQLFormattedArray(location)
		getTags += fmt.Sprintf("AND t.location @> %s\n", locArray)
	}
	if upvotes > 0 {
		getTags += fmt.Sprintf("AND t.upvotes > %d\n", upvotes)
	} else if upvotes < 0 {
		getTags += fmt.Sprintf("AND t.upvotes < %d\n", -upvotes)
	}
	if downvotes > 0 {
		getTags += fmt.Sprintf("t.downvotes > %d\n", downvotes)
	} else if downvotes < 0 {
		getTags += fmt.Sprintf("t.downvotes < %d\n", -downvotes)
	}

	getTags += SQLSortOrder(sortOrder)
	getTags += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	fmt.Println(getTags)
	rows, e := db.Query(getTags)
	if DidFail(e, "get tags") {
		return []UserPrefTag{}
	}
	defer rows.Close()

	result := ScanUserPrefTags(rows)

	return result
}

type UserCont struct {
	PostID    int64
	CommentID int64
}

type UserContResult struct {
	Post      PostResult
	CommentId int64
}

func SQLFieldsForUserCont() string {
	return "postId, commentId"
}

func SQLFieldsForUserContResult() string {
	return "p.id, p.userId, p.content, p.tags, p.createdAt, p.updatedAt, p.location, p.upvotes, p.downvotes," +
		"u.id, u.name, u.registerDate, u.upvotes, u.downvotes, up.commentId"
}

func ScanUserCont(row *sql.Row) (UserCont, error) {
	up := UserCont{}
	e := row.Scan(&up.PostID, &up.CommentID)
	return up, e
}

func ScanUserContResult(row *sql.Row) (UserContResult, error) {
	p := PostResult{}
	u := UserProfile{}
	var userId int64
	var commentId int64
	e := row.Scan(&p.ID, &userId, &p.Content, pq.Array(&p.Tags), &p.CreatedAt, &p.UpdatedAt, pq.Array(&p.Location), &p.Upvotes,
		&p.Downvotes, &u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Upvotes, &commentId)
	p.Author = u
	return UserContResult{p, commentId}, e
}

func ScanUserConts(rows *sql.Rows) []UserCont {
	result := []UserCont{}
	var e error
	for rows.Next() {
		up := UserCont{}
		e = rows.Scan(&up.PostID, &up.CommentID)
		if DidFail(e, "scan user pref") {
			continue
		}
		result = append(result, up)
	}
	return result
}

func ScanUserContResults(rows *sql.Rows) []UserContResult {
	result := []UserContResult{}
	var e error
	for rows.Next() {
		p := PostResult{}
		u := UserProfile{}
		var userId int64
		var commentId int64
		var score float64
		var cred float64
		e = rows.Scan(&p.ID, &userId, &p.Content, pq.Array(&p.Tags), &p.CreatedAt, &p.UpdatedAt, pq.Array(&p.Location), &p.Upvotes,
			&p.Downvotes, &u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Upvotes, &commentId, &cred, &score)
		p.Author = u
		if DidFail(e, "scan user pref") {
			continue
		}
		result = append(result, UserContResult{p, commentId})
	}
	return result
}

func DBGetUserCont(db *sql.DB, userId int64, isPosts bool, tags []string, location []string, upvotes int64, downvotes int64, sortOrder SortOrder, limit int64, offset int64, startDate string, endDate string) []UserContResult {
	query := fmt.Sprintf(`SELECT %s, RATIO(p.upvotes, p.downvotes) AS cred, p.upvotes * RATIO(p.upvotes, p.downvotes) AS score 
	FROM User%dCont up 
	JOIN posts{} p ON up.postId = p.id 
	JOIN users u ON p.userId = u.id
	`, SQLFieldsForUserContResult(), userId)

	if isPosts {
		query += "WHERE up.commentId <= 0\n"
	} else {
		query += "WHERE up.commentId > 0\n"
	}
	if len(tags) > 0 {
		queryTags := SQLFormattedArray(tags)
		query += fmt.Sprintf("AND %s && p.tags\n", queryTags)
	}
	if len(location) > 0 {
		queryLoc := SQLFormattedArray(location)
		query += fmt.Sprintf("AND p.location @> %s\n", queryLoc)
	}
	if upvotes != 0 {
		if upvotes > 0 {
			query += fmt.Sprintf("AND p.upvotes > %d\n", upvotes)
		} else {
			query += fmt.Sprintf("AND p.upvotes < %d\n", -upvotes)
		}
	}
	if downvotes != 0 {
		if upvotes > 0 {
			query += fmt.Sprintf("AND p.downvotes > %d\n", upvotes)
		} else {
			query += fmt.Sprintf("AND p.downvotes < %d\n", -upvotes)
		}
	}

	sDate := parseTime(startDate)
	eDate := parseTime(endDate)
	years := yearsBetweenDates(sDate, eDate)
	query = BuildUnionForYears(query, years)

	query += SQLSortOrder(sortOrder)

	query += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	rows, e := db.Query(query)
	result := []UserContResult{}
	if DidFail(e, "get user content") {
		return result
	}
	result = ScanUserContResults(rows)

	return result
}
