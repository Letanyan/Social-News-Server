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
	upBlacklistUser
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
	Upvotes   float64
	Downvotes float64
}

type UserPrefComment struct {
	Comment   CommentResult
	Upvotes   float64
	Downvotes float64
}

type UserPrefTag struct {
	Tag       Tag
	Upvotes   float64
	Downvotes float64
}

func DBCreateUserPref(db *sql.DB, uid int64, kind UserPrefKind, pid int64, sid int64, upvoteAmount int64) UserPref {
	var updateField string
	isUpvote := upvoteAmount > 0
	if isUpvote {
		updateField = "upvotes"
	} else {
		updateField = "downvotes"
		upvoteAmount = -upvoteAmount
	}

	query := fmt.Sprintf(`
	INSERT INTO UserPref(uid, kind, pid, sid)
	VALUES(%d, %d, %d, %d)
	ON CONFLICT(uid, kind, pid, sid)
	DO NOTHING;
	UPDATE UserPref SET
	%s = %s + %d
	WHERE kind=%d AND uid=%d AND pid=%d AND sid=%d
	RETURNING %s
	`, uid, kind, pid, sid,
		updateField, updateField, upvoteAmount,
		kind, uid, pid, sid, SQLFieldsForUserPref())

	row := db.QueryRow(query)
	pref, e := ScanUserPrefRow(row)
	if DidFail(e, "create user pref") {
		return UserPref{}
	}

	return pref
}

func DBWatchUser(db *sql.DB, uid int64, tags []int64, viewTime float64) []UserPref {
	tagArray := SQLFormattedIndexArray(tags)
	tagItems := SQLFormattedIndexList(tags, func(i int64) string {
		return fmt.Sprintf("(%d, 4, %d, -1)", uid, i)
	})
	query := fmt.Sprintf(`
	INSERT INTO UserPref(uid, kind, pid, sid)
	VALUES %s
	ON CONFLICT(uid, kind, pid, sid)
	DO NOTHING;
	UPDATE UserPref SET
	upvotes = upvotes * %f
	WHERE kind=4 AND uid=%d AND pid=ANY(%s)
	RETURNING %s
	`, tagItems, viewTime, uid, tagArray, SQLFieldsForUserPref())

	rows, e := db.Query(query)
	if DidFail(e, "create user pref", query) {
		return []UserPref{}
	}
	result := ScanUserPrefRows(rows)

	return result
}

func SQLFieldsForUserPref() string {
	return "kind, pid, sid, upvotes, downvotes"
}

func SQLFieldsForUserPrefUser() string {
	return "u.id, u.name, u.registerDate, u.upvotes AS item_up, u.downvotes AS item_down, up.upvotes AS sec_up, up.downvotes AS sec_down"
}

func SQLFieldsForUserPrefPost() string {
	return "p.id, p.userId, p.content, p.tags, p.createdAt, p.location, p.upvotes AS item_up, p.downvotes AS item_down, u.id, u.name, u.registerDate, u.upvotes, u.downvotes, up.upvotes AS sec_up, up.downvotes AS sec_down"
}

func SQLFieldsForUserPrefComment() string {
	return "p.id, p.postId, p.userId, p.replyId, p.content, p.createdAt, p.replyCount, p.upvotes AS item_up, p.downvotes AS item_down, u.id, u.name, u.registerDate, u.upvotes, u.downvotes, up.upvotes AS sec_up, up.downvotes AS sec_down"
}

func SQLFieldsForUserPrefTag() string {
	return "t.id, t.name, t.upvotes AS item_up, t.downvotes AS item_down, up.upvotes AS sec_up, up.downvotes AS sec_down"
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
		var up float64
		var down float64
		var userId int64
		e = rows.Scan(&p.ID, &userId, &p.Content, pq.Array(&p.Tags), &p.CreatedAt,
			pq.Array(&p.Location), &p.Upvotes, &p.Downvotes, &u.ID, &u.Name, &u.RegisterDate,
			&u.Upvotes, &u.Upvotes, &up, &down, &cred, &score)
		if DidFail(e, "scan user pref post") {
			continue
		}
		p.Author = u
		result = append(result, UserPrefPost{p, up, down})
	}
	return result
}

func ScanUserPrefComments(rows *sql.Rows) []UserPrefComment {
	result := []UserPrefComment{}
	var cred float64
	var score float64
	var e error
	for rows.Next() {
		c := CommentResult{}
		u := UserProfile{}
		var up float64
		var down float64
		var userId int64
		e = rows.Scan(&c.ID, &c.PostID, &userId, &c.ReplyID, &c.Content, &c.CreatedAt,
			&c.ReplyCount, &c.Upvotes, &c.Downvotes, &u.ID, &u.Name, &u.RegisterDate,
			&u.Upvotes, &u.Upvotes, &up, &down, &cred, &score)
		if DidFail(e, "scan user pref post") {
			continue
		}
		c.Author = u
		result = append(result, UserPrefComment{c, up, down})
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
		e = rows.Scan(&tag.ID, &tag.Name,
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
	query := fmt.Sprintf(`
	SELECT %s, RATIO(upvotes, downvotes) AS cred, upvotes * RATIO(upvotes, downvotes) AS score 
	FROM UserPref WHERE uid=%d `, SQLFieldsForUserPref(), userId)

	cond := ""
	if kind > 0 {
		cond = fmt.Sprintf("AND kind = %d ", kind)
	}
	if pid > 0 {
		cond += fmt.Sprintf("AND pid = %d ", pid)
	}
	if sid > 0 {
		cond += fmt.Sprintf("AND sid = %d ", sid)
	}
	if upvotes > 0 {
		cond += fmt.Sprintf("AND upvotes > %d ", upvotes)
	} else if upvotes < 0 {
		cond += fmt.Sprintf("AND upvotes < %d ", -upvotes)
	}
	if downvotes > 0 {
		cond += fmt.Sprintf("AND downvotes > %d ", downvotes)
	} else if downvotes < 0 {
		cond += fmt.Sprintf("AND downvotes < %d ", -downvotes)
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

func DBGetUserPrefUsers(db *sql.DB, kind UserPrefKind, userId int64, upvoteAmount int64,
	downvoteAmount int64, upvotes int64, downvotes int64, sortOrder SortOrder, search string,
	limit int64, offset int64) []UserPrefUser {
	getUsers := fmt.Sprintf(`
	SELECT %s, RATIO(up.upvotes, up.downvotes) AS cred, up.upvotes * RATIO(up.upvotes, up.downvotes) AS score  
	FROM UserPref up 
	JOIN users u ON up.pid = u.id 
	WHERE kind=%d AND up.uid = %d
	`, SQLFieldsForUserPrefUser(), kind, userId)

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
	if len(search) > 0 {
		search, _ := DBPrepareSearchString(search)
		if len(search) > 0 {
			getUsers += fmt.Sprintf("AND u.Name @@ websearch_to_tsquery('%s') ", search)
		}
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

func DBGetUserPrefPosts(db *sql.DB, userId int64, upvoteAmount int64,
	downvoteAmount int64, authorId int64, tags []int64,
	location []string, upvotes int64, downvotes int64, sortOrder SortOrder, search string,
	limit int64, offset int64, startDate string, endDate string) []UserPrefPost {
	getPosts := fmt.Sprintf(`SELECT %s, RATIO(up.upvotes, up.downvotes) AS cred, up.upvotes * RATIO(up.upvotes, up.downvotes) AS score 
		FROM UserPref up 
		JOIN posts p ON up.pid = p.id
		JOIN users u ON p.userId = u.id
		WHERE up.kind = %d AND up.uid = %d AND p.trashed=false 
		`, SQLFieldsForUserPrefPost(), upPost, userId)

	if len(startDate) > 0 && len(endDate) > 0 {
		getPosts += fmt.Sprintf("AND p.createdAt BETWEEN (TIMESTAMP '%s') AND (TIMESTAMP '%s') ", startDate, endDate)
	} else if len(startDate) > 0 {
		getPosts += fmt.Sprintf("AND p.createdAt > (TIMESTAMP '%s') ", startDate)
	} else if len(endDate) > 0 {
		getPosts += fmt.Sprintf("AND p.createdAt < (TIMESTAMP '%s') ", endDate)
	}
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
	if len(search) > 0 {
		search, altTags := DBPrepareSearchString(search)
		tags = append(tags, altTags...)
		if len(search) > 0 {
			getPosts += fmt.Sprintf("AND p.Content @@ websearch_to_tsquery('%s') ", search)
		}
	}
	if len(tags) > 0 {
		queryTags := SQLFormattedIndexArray(tags)
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

	getPosts += SQLSortOrder(sortOrder)

	getPosts += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	rows, e := db.Query(getPosts)
	if DidFail(e, "get posts") {
		return []UserPrefPost{}
	}
	defer rows.Close()

	result := ScanUserPrefPosts(rows)
	return result
}

func DBGetUserPrefComments(db *sql.DB, userId int64, upvoteAmount int64,
	downvoteAmount int64, authorId int64, replyId int64,
	upvotes int64, downvotes int64, sortOrder SortOrder, search string,
	limit int64, offset int64, startDate string, endDate string) []UserPrefComment {
	getComments := fmt.Sprintf(`SELECT %s, RATIO(up.upvotes, up.downvotes) AS cred, up.upvotes * RATIO(up.upvotes, up.downvotes) AS score 
		FROM UserPref up 
		JOIN Comments p ON up.pid = p.postId AND up.sid = p.id
		JOIN Users u ON p.userId = u.id
		WHERE up.kind = %d AND up.uid = %d AND p.trashed=false 
		`, SQLFieldsForUserPrefComment(), upComment, userId)

	if len(startDate) > 0 && len(endDate) > 0 {
		getComments += fmt.Sprintf("AND p.createdAt BETWEEN (TIMESTAMP '%s') AND (TIMESTAMP '%s') ", startDate, endDate)
	} else if len(startDate) > 0 {
		getComments += fmt.Sprintf("AND p.createdAt > (TIMESTAMP '%s') ", startDate)
	} else if len(endDate) > 0 {
		getComments += fmt.Sprintf("AND p.createdAt < (TIMESTAMP '%s') ", endDate)
	}
	if upvoteAmount > 0 {
		getComments += fmt.Sprintf("AND up.upvotes > %d ", upvoteAmount)
	} else if upvoteAmount < 0 {
		getComments += fmt.Sprintf("AND up.upvotes < %d ", -upvoteAmount)
	}
	if downvoteAmount > 0 {
		getComments += fmt.Sprintf("AND up.downvotes > %d ", downvoteAmount)
	} else if downvoteAmount < 0 {
		getComments += fmt.Sprintf("AND up.downvotes < %d ", -downvoteAmount)
	}
	if authorId != 0 {
		getComments += fmt.Sprintf("AND p.userId = %d ", authorId)
	}
	if replyId != 0 {
		getComments += fmt.Sprintf("AND p.replyId = %d ", replyId)
	}
	if upvotes > 0 {
		getComments += fmt.Sprintf("AND p.upvotes > %d ", upvotes)
	} else if upvotes < 0 {
		getComments += fmt.Sprintf("AND p.upvotes < %d ", -upvotes)
	}
	if downvotes > 0 {
		getComments += fmt.Sprintf("AND p.downvotes > %d ", downvotes)
	} else if downvotes < 0 {
		getComments += fmt.Sprintf("AND p.downvotes < %d ", -downvotes)
	}
	if len(search) > 0 {
		search, _ := DBPrepareSearchString(search)
		if len(search) > 0 {
			getComments += fmt.Sprintf("AND p.Content @@ websearch_to_tsquery('%s') ", search)
		}
	}

	getComments += SQLSortOrder(sortOrder)

	getComments += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	rows, e := db.Query(getComments)
	if DidFail(e, "get posts") {
		return []UserPrefComment{}
	}
	defer rows.Close()

	result := ScanUserPrefComments(rows)
	return result
}

func DBGetUserPrefTags(db *sql.DB, userId int64, upvoteAmount int64, downvoteAmount int64,
	tags []string, location []string, upvotes int64, downvotes int64,
	sortOrder SortOrder, search string, limit int64, offset int64) []UserPrefTag {
	getTags := fmt.Sprintf(`SELECT %s, RATIO(up.upvotes, up.downvotes) AS cred, up.upvotes * RATIO(up.upvotes, up.downvotes) AS score 
	FROM UserPref up
	JOIN tags t ON up.pid = t.id
	WHERE up.kind = 4 AND up.uid = %d
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
	if len(search) > 0 {
		search, _ := DBPrepareSearchString(search)
		if len(search) > 0 {
			getTags += fmt.Sprintf("AND t.Name @@ websearch_to_tsquery('%s') ", search)
		}
	}

	getTags += SQLSortOrder(sortOrder)
	getTags += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	rows, e := db.Query(getTags)
	if DidFail(e, "get tags", getTags) {
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

func SQLFieldsForUserCont() string {
	return "postId, commentId"
}

func SQLFieldsForUserContPost() string {
	return "p.id, p.userId, p.content, p.tags, p.createdAt, p.location, p.upvotes AS item_up, p.downvotes AS item_down," +
		"u.id, u.name, u.registerDate, u.upvotes, u.downvotes"
}

func SQLFieldsForUserContComment() string {
	return "p.id, p.postId, p.userId, p.replyId, p.content, p.createdAt, p.replyCount, p.upvotes AS item_up, p.downvotes AS item_down," +
		"u.id, u.name, u.registerDate, u.upvotes, u.downvotes"
}

func ScanUserCont(row *sql.Row) (UserCont, error) {
	up := UserCont{}
	e := row.Scan(&up.PostID, &up.CommentID)
	return up, e
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

type UserContPlaylist int

const (
	ucpCreated UserContPlaylist = iota
	ucpViewed
	ucpReadLater
)

func DBCreateUserCont(db *sql.DB, kind UserContPlaylist, userId int64, postId int64, commentId int64) UserCont {
	query := fmt.Sprintf(`
	INSERT INTO UserCont(userId, postId, commentId, kind)
	VALUES(%d, %d, %d, %d)
	RETURNING %s;
	`, userId, postId, commentId, kind, SQLFieldsForUserCont())
	row := db.QueryRow(query)
	userCont, e := ScanUserCont(row)
	if DidFail(e, "insert into user cont") {
		return UserCont{}
	}
	return userCont
}

func DBGetUserContPost(db *sql.DB, kind UserContPlaylist, userId int64, tags []int64,
	location []string, upvotes int64, downvotes int64, sortOrder SortOrder, search string,
	limit int64, offset int64, startDate string, endDate string) []PostResult {
	query := fmt.Sprintf(`SELECT %s, RATIO(p.upvotes, p.downvotes) AS cred, p.upvotes * RATIO(p.upvotes, p.downvotes) AS score 
	FROM UserCont up 
	JOIN posts p ON up.postId = p.id 
	JOIN users u ON p.userId = u.id
	WHERE up.userId = %d AND p.trashed=false AND up.commentId <= 0 AND kind = %d
	`, SQLFieldsForPostResultAlias(), userId, kind)

	if len(search) > 0 {
		search, altTags := DBPrepareSearchString(search)
		tags = append(tags, altTags...)
		if len(search) > 0 {
			query += fmt.Sprintf("AND p.Content @@ websearch_to_tsquery('%s') ", search)
		}
	}
	if len(tags) > 0 {
		queryTags := SQLFormattedIndexArray(tags)
		query += fmt.Sprintf("AND %s && p.tags ", queryTags)
	}
	if len(location) > 0 {
		queryLoc := SQLFormattedArray(location)
		query += fmt.Sprintf("AND p.location @> %s ", queryLoc)
	}
	if upvotes != 0 {
		if upvotes > 0 {
			query += fmt.Sprintf("AND p.upvotes > %d ", upvotes)
		} else {
			query += fmt.Sprintf("AND p.upvotes < %d ", -upvotes)
		}
	}
	if downvotes != 0 {
		if upvotes > 0 {
			query += fmt.Sprintf("AND p.downvotes > %d ", upvotes)
		} else {
			query += fmt.Sprintf("AND p.downvotes < %d ", -upvotes)
		}
	}

	sDate := parseTime(startDate)
	eDate := parseTime(endDate)
	years := yearsBetweenDates(sDate, eDate)
	query = BuildUnionForYears(query, years)

	query += SQLSortOrder(sortOrder)

	query += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	rows, e := db.Query(query)
	result := []PostResult{}
	if DidFail(e, "get user content") {
		return result
	}
	result = ScanPostResults(rows, false)

	return result
}

func DBGetUserContComments(db *sql.DB, userId int64, authorId int64, replyId int64,
	upvotes int64, downvotes int64, sortOrder SortOrder, search string,
	limit int64, offset int64, startDate string, endDate string) []CommentResult {
	getComments := fmt.Sprintf(`SELECT %s, RATIO(p.upvotes, p.downvotes) AS cred, p.upvotes * RATIO(p.upvotes, p.downvotes) AS score 
		FROM UserCont up 
		JOIN Comments p ON up.postId = p.postId AND up.commentId = p.id
		JOIN Users u ON p.userId = u.id
		WHERE up.commentid > 0 AND up.userid = %d AND p.trashed=false 
		`, SQLFieldsForCommentResultAlias(), userId)

	if len(startDate) > 0 && len(endDate) > 0 {
		getComments += fmt.Sprintf("AND p.createdAt BETWEEN (TIMESTAMP '%s') AND (TIMESTAMP '%s') ", startDate, endDate)
	} else if len(startDate) > 0 {
		getComments += fmt.Sprintf("AND p.createdAt > (TIMESTAMP '%s') ", startDate)
	} else if len(endDate) > 0 {
		getComments += fmt.Sprintf("AND p.createdAt < (TIMESTAMP '%s') ", endDate)
	}
	if authorId != 0 {
		getComments += fmt.Sprintf("AND p.userId = %d ", authorId)
	}
	if replyId != 0 {
		getComments += fmt.Sprintf("AND p.replyId = %d ", replyId)
	}
	if upvotes > 0 {
		getComments += fmt.Sprintf("AND p.upvotes > %d ", upvotes)
	} else if upvotes < 0 {
		getComments += fmt.Sprintf("AND p.upvotes < %d ", -upvotes)
	}
	if downvotes > 0 {
		getComments += fmt.Sprintf("AND p.downvotes > %d ", downvotes)
	} else if downvotes < 0 {
		getComments += fmt.Sprintf("AND p.downvotes < %d ", -downvotes)
	}
	if len(search) > 0 {
		search, _ := DBPrepareSearchString(search)
		if len(search) > 0 {
			getComments += fmt.Sprintf("AND p.Content @@ websearch_to_tsquery('%s') ", search)
		}
	}

	getComments += SQLSortOrder(sortOrder)

	getComments += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	rows, e := db.Query(getComments)
	if DidFail(e, "get posts") {
		return []CommentResult{}
	}
	defer rows.Close()

	result := ScanCommentResults(rows, false)
	return result
}
