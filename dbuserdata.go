package main

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/lib/pq"
)

type UserPrefKind int

const (
	upUser UserPrefKind = iota
	upComment
	upPost
	upTag
	upWatchTag
)

type UserPref struct {
	Kind      UserPrefKind
	PID       int64
	SID       int64
	Upvotes   int64
	Downvotes int64
}

type UserPrefUser struct {
	User      UserProfile
	Upvotes   int64
	Downvotes int64
}

type UserPrefPost struct {
	Post      PostResult
	Upvotes   int64
	Downvotes int64
}

type UserPrefComment struct {
	Comment   CommentResult
	Upvotes   int64
	Downvotes int64
}

type UserPrefTag struct {
	Tag       Tag
	Upvotes   int64
	Downvotes int64
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
	%s = %s + %d,
	updatedOn = (now() at time zone 'utc')
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

func DBWatchUser(db *sql.DB, uid int64, postId int64, viewTime int64, location []string) []UserPref {
	post := DBGetPost(db, postId)
	locIndex := DBCreateLocation(db, location)
	locArray := SQLFormattedIndexArray(locIndex)
	tags := []int64{}
	for i := range tags {
		t, e := strconv.ParseInt(post.Tags[i], 10, 64)
		if DidFail(e, "convert tag to int", post.Tags[i]) {
			continue
		}
		tags = append(tags, t)
	}
	_, result := DBVoteTags(db, uid, tags, viewTime, locArray, false)
	return result
}

func SQLFieldsForUserPref() string {
	return "kind, pid, sid, upvotes, downvotes"
}

func SQLFieldsForUserPrefResolved() string {
	return "up.kind, up.pid, up.sid, up.upvotes, up.downvotes"
}

func SQLFieldsForUserPrefUser() string {
	return "p.id, p.name, p.registerDate, p.upvotes AS item_up, p.downvotes AS item_down, p.email, up.upvotes AS sec_up, up.downvotes AS sec_down"
}

func SQLFieldsForUserPrefPost() string {
	return "p.id, p.userId, p.content, p.tags, p.createdAt, p.location, p.upvotes AS item_up, p.downvotes AS item_down, p.CommentCount, p.trashed, u.id, u.name, u.registerDate, u.upvotes, u.downvotes, u.email, up.upvotes AS sec_up, up.downvotes AS sec_down"
}

func SQLFieldsForUserPrefComment() string {
	return "p.id, p.postId, p.userId, p.replyId, p.content, p.createdAt, p.replyCount, p.upvotes AS item_up, p.downvotes AS item_down, p.trashed, u.id, u.name, u.registerDate, u.upvotes, u.downvotes, u.email, up.upvotes AS sec_up, up.downvotes AS sec_down"
}

func SQLFieldsForUserPrefTag() string {
	return "p.id, p.name, p.upvotes AS item_up, p.downvotes AS item_down, up.upvotes AS sec_up, up.downvotes AS sec_down"
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
	defer rows.Close()
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
	defer rows.Close()
	for rows.Next() {
		u := UserProfile{}
		var up int64
		var down int64
		var email string
		e = rows.Scan(&u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Downvotes, &email, &up, &down, &cred, &score)
		if DidFail(e, "scan user pref users") {
			continue
		}
		u.IsAgent = len(email) == 0
		result = append(result, UserPrefUser{u, up, down})
	}
	return result
}

func ScanUserPrefPosts(rows *sql.Rows) []UserPrefPost {
	result := []UserPrefPost{}
	var cred float64
	var score float64
	var e error
	defer rows.Close()
	for rows.Next() {
		p := PostResult{}
		u := UserProfile{}
		var up int64
		var down int64
		var userId int64
		var email string
		e = rows.Scan(&p.ID, &userId, &p.Content, pq.Array(&p.Tags), &p.CreatedAt,
			pq.Array(&p.Location), &p.Upvotes, &p.Downvotes, &p.CommentCount, &p.Trashed, &u.ID, &u.Name, &u.RegisterDate,
			&u.Upvotes, &u.Downvotes, &email, &up, &down, &cred, &score)
		if DidFail(e, "scan user pref post") {
			continue
		}
		u.IsAgent = len(email) == 0
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
	defer rows.Close()
	for rows.Next() {
		c := CommentResult{}
		u := UserProfile{}
		var up int64
		var down int64
		var userId int64
		var email string
		e = rows.Scan(&c.ID, &c.PostID, &userId, &c.ReplyID, &c.Content, &c.CreatedAt,
			&c.ReplyCount, &c.Upvotes, &c.Downvotes, &c.Trashed, &u.ID, &u.Name, &u.RegisterDate,
			&u.Upvotes, &u.Downvotes, &email, &up, &down, &cred, &score)
		if DidFail(e, "scan user pref post") {
			continue
		}
		u.IsAgent = len(email) == 0
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
	defer rows.Close()
	for rows.Next() {
		tag := Tag{}
		var up int64
		var down int64
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
func DBGetUserPref(db *sql.DB, isOwner bool, userId int64, sortOrder SortOrder, kind UserPrefKind, pid int64, sid int64, upvotes int64, downvotes int64, limit int64, offset int64) []UserPref {
	query := fmt.Sprintf(`
	SELECT %s, RATIO(up.upvotes, up.downvotes) AS cred, up.upvotes * RATIO(up.upvotes, up.downvotes) AS score 
	FROM UserPref up
	JOIN Users p ON p.ID = up.uid
	WHERE up.uid=%d `, SQLFieldsForUserPrefResolved(), userId)

	cond := ""
	if kind > 0 {
		cond = fmt.Sprintf("AND up.kind = %d ", kind)
	}
	if !isOwner {
		switch kind {
		case upComment:
			cond += "AND p.publicCommentVotes "
		case upPost:
			cond += "AND p.publicPostVotes "
		case upTag:
			cond += "AND p.publicTagVotes "
		case upUser:
			cond += "AND p.publicUserVotes "
		}
	}

	if pid > 0 {
		cond += fmt.Sprintf("AND up.pid = %d ", pid)
	}
	if sid > 0 {
		cond += fmt.Sprintf("AND up.sid = %d ", sid)
	}
	if upvotes > 0 {
		cond += fmt.Sprintf("AND up.upvotes >= %d ", upvotes)
	} else if upvotes < 0 {
		cond += fmt.Sprintf("AND up.upvotes < %d ", -upvotes)
	}
	if downvotes > 0 {
		cond += fmt.Sprintf("AND up.downvotes >= %d ", downvotes)
	} else if downvotes < 0 {
		cond += fmt.Sprintf("AND up.downvotes < %d ", -downvotes)
	}

	query += cond + "\n"
	query += SQLSortOrder(sortOrder, false)

	query += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)
	rows, e := db.Query(query)
	result := []UserPref{}
	if DidFail(e, "select user prefs for ", userId, " of kind ", kind, " pid:", pid, " sid: ", sid) {
		return result
	}
	result = ScanUserPrefRows(rows)

	return result
}

func DBGetUserPrefUsers(db *sql.DB, isOwner bool, userId int64, upvoteAmount int64,
	downvoteAmount int64, upvotes int64, downvotes int64, sortOrder SortOrder, search string,
	limit int64, offset int64, startDate string, endDate string) []UserPrefUser {
	getUsers := fmt.Sprintf(`
	SELECT %s, RATIO(up.upvotes, up.downvotes) AS cred, up.upvotes * RATIO(up.upvotes, up.downvotes) AS score  
	FROM UserPref up 
	JOIN Users p ON up.pid = p.id 
	JOIN Users x ON up.uid = x.id 
	WHERE kind=%d AND up.uid=%d
	`, SQLFieldsForUserPrefUser(), upUser, userId)

	if !isOwner {
		getUsers += "AND x.publicUserVotes "
	}
	if len(startDate) > 0 && len(endDate) > 0 {
		getUsers += fmt.Sprintf("AND up.updatedOn BETWEEN (TIMESTAMP '%s') AND (TIMESTAMP '%s') ", startDate, endDate)
	} else if len(startDate) > 0 {
		getUsers += fmt.Sprintf("AND up.updatedOn > (TIMESTAMP '%s') ", startDate)
	} else if len(endDate) > 0 {
		getUsers += fmt.Sprintf("AND up.updatedOn < (TIMESTAMP '%s') ", endDate)
	}
	if upvoteAmount > 0 {
		getUsers += fmt.Sprintf("AND up.upvotes >= %d ", upvoteAmount)
	} else if upvoteAmount < 0 {
		getUsers += fmt.Sprintf("AND up.upvotes < %d ", -upvoteAmount)
	}
	if downvoteAmount > 0 {
		getUsers += fmt.Sprintf("AND up.downvotes >= %d ", downvoteAmount)
	} else if downvoteAmount < 0 {
		getUsers += fmt.Sprintf("AND up.downvotes < %d ", -downvoteAmount)
	}

	if upvotes > 0 {
		getUsers += fmt.Sprintf("AND p.upvotes >= %d ", upvotes)
	} else if upvotes < 0 {
		getUsers += fmt.Sprintf("AND p.upvotes < %d ", -upvotes)
	}
	if downvotes > 0 {
		getUsers += fmt.Sprintf("AND p.downvotes >= %d ", downvotes)
	} else if downvotes < 0 {
		getUsers += fmt.Sprintf("AND p.downvotes < %d ", -downvotes)
	}
	if len(search) > 0 {
		search, _ := DBPrepareSearchString(db, search)
		if len(search) > 0 {
			getUsers += fmt.Sprintf("AND p.Name @@ to_tsquery('%s') ", search)
		}
	}

	getUsers += SQLSortOrder(sortOrder, true)
	getUsers += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	rows, e := db.Query(getUsers)
	if DidFail(e, "get users") {
		return []UserPrefUser{}
	}

	result := ScanUserPrefUsers(rows)

	return result
}

func DBGetUserPrefPosts(db *sql.DB, isOwner bool, userId int64, upvoteAmount int64,
	downvoteAmount int64, authorId int64, tags []int64,
	location []string, upvotes int64, downvotes int64, sortOrder SortOrder, search string,
	limit int64, offset int64, startDate string, endDate string) []UserPrefPost {
	getPosts := fmt.Sprintf(`SELECT %s, RATIO(up.upvotes, up.downvotes) AS cred, up.upvotes * RATIO(up.upvotes, up.downvotes) AS score 
		FROM UserPref up 
		JOIN Posts p ON up.pid=p.id
		JOIN Users u ON p.userId=u.id
		JOIN Users x ON up.uid = x.id 
		WHERE up.kind = %d AND up.uid = %d AND p.trashed=false 
		`, SQLFieldsForUserPrefPost(), upPost, userId)

	if !isOwner {
		getPosts += "AND x.publicPostVotes "
	}
	if len(startDate) > 0 && len(endDate) > 0 {
		getPosts += fmt.Sprintf("AND up.updatedOn BETWEEN (TIMESTAMP '%s') AND (TIMESTAMP '%s') ", startDate, endDate)
	} else if len(startDate) > 0 {
		getPosts += fmt.Sprintf("AND up.updatedOn > (TIMESTAMP '%s') ", startDate)
	} else if len(endDate) > 0 {
		getPosts += fmt.Sprintf("AND up.updatedOn < (TIMESTAMP '%s') ", endDate)
	}
	if upvoteAmount > 0 {
		getPosts += fmt.Sprintf("AND up.upvotes >= %d ", upvoteAmount)
	} else if upvoteAmount < 0 {
		getPosts += fmt.Sprintf("AND up.upvotes < %d ", -upvoteAmount)
	}
	if downvoteAmount > 0 {
		getPosts += fmt.Sprintf("AND up.downvotes >= %d ", downvoteAmount)
	} else if downvoteAmount < 0 {
		getPosts += fmt.Sprintf("AND up.downvotes < %d ", -downvoteAmount)
	}

	if authorId > 0 {
		getPosts += fmt.Sprintf("AND p.userId = %d ", authorId)
	}
	if len(search) > 0 {
		search, altTags := DBPrepareSearchString(db, search)
		tags = append(tags, altTags...)
		if len(search) > 0 {
			getPosts += fmt.Sprintf("AND p.Content @@ to_tsquery('%s') ", search)
		}
	}
	if len(tags) > 0 {
		queryTags := SQLFormattedIndexArray(tags)
		getPosts += fmt.Sprintf("AND %s && p.tags ", queryTags)
	}
	if len(location) > 0 {
		locIndex := DBCreateLocation(db, location)
		queryLoc := SQLFormattedIndexArray(locIndex)
		getPosts += fmt.Sprintf("AND p.location @> %s ", queryLoc)
	}
	if upvotes > 0 {
		getPosts += fmt.Sprintf("AND p.upvotes >= %d ", upvotes)
	} else if upvotes < 0 {
		getPosts += fmt.Sprintf("AND p.upvotes < %d ", -upvotes)
	}
	if downvotes > 0 {
		getPosts += fmt.Sprintf("AND p.downvotes >= %d ", downvotes)
	} else if downvotes < 0 {
		getPosts += fmt.Sprintf("AND p.downvotes < %d ", -downvotes)
	}

	getPosts += SQLSortOrder(sortOrder, true)

	getPosts += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	rows, e := db.Query(getPosts)
	if DidFail(e, "get posts") {
		return []UserPrefPost{}
	}

	result := ScanUserPrefPosts(rows)
	return result
}

func DBGetUserPrefComments(db *sql.DB, isOwner bool, userId int64, upvoteAmount int64,
	downvoteAmount int64, authorId int64, replyId int64,
	upvotes int64, downvotes int64, sortOrder SortOrder, search string,
	limit int64, offset int64, startDate string, endDate string) []UserPrefComment {
	getComments := fmt.Sprintf(`SELECT %s, RATIO(up.upvotes, up.downvotes) AS cred, up.upvotes * RATIO(up.upvotes, up.downvotes) AS score 
		FROM UserPref up 
		JOIN Comments p ON up.pid = p.postId AND up.sid = p.id
		JOIN Users u ON p.userId = u.id
		JOIN Users x ON up.uid = x.id 
		WHERE up.kind = %d AND up.uid = %d AND p.trashed=false 
		`, SQLFieldsForUserPrefComment(), upComment, userId)

	if !isOwner {
		getComments += "AND x.publicCommentVotes "
	}
	if len(startDate) > 0 && len(endDate) > 0 {
		getComments += fmt.Sprintf("AND up.updatedOn BETWEEN (TIMESTAMP '%s') AND (TIMESTAMP '%s') ", startDate, endDate)
	} else if len(startDate) > 0 {
		getComments += fmt.Sprintf("AND up.updatedOn > (TIMESTAMP '%s') ", startDate)
	} else if len(endDate) > 0 {
		getComments += fmt.Sprintf("AND up.updatedOn < (TIMESTAMP '%s') ", endDate)
	}
	if upvoteAmount > 0 {
		getComments += fmt.Sprintf("AND up.upvotes >= %d ", upvoteAmount)
	} else if upvoteAmount < 0 {
		getComments += fmt.Sprintf("AND up.upvotes < %d ", -upvoteAmount)
	}
	if downvoteAmount > 0 {
		getComments += fmt.Sprintf("AND up.downvotes >= %d ", downvoteAmount)
	} else if downvoteAmount < 0 {
		getComments += fmt.Sprintf("AND up.downvotes < %d ", -downvoteAmount)
	}
	if authorId > 0 {
		getComments += fmt.Sprintf("AND p.userId = %d ", authorId)
	}
	if replyId > 0 {
		getComments += fmt.Sprintf("AND p.replyId = %d ", replyId)
	}
	if upvotes > 0 {
		getComments += fmt.Sprintf("AND p.upvotes >= %d ", upvotes)
	} else if upvotes < 0 {
		getComments += fmt.Sprintf("AND p.upvotes < %d ", -upvotes)
	}
	if downvotes > 0 {
		getComments += fmt.Sprintf("AND p.downvotes >= %d ", downvotes)
	} else if downvotes < 0 {
		getComments += fmt.Sprintf("AND p.downvotes < %d ", -downvotes)
	}
	if len(search) > 0 {
		search, _ := DBPrepareSearchString(db, search)
		if len(search) > 0 {
			getComments += fmt.Sprintf("AND p.Content @@ to_tsquery('%s') ", search)
		}
	}

	getComments += SQLSortOrder(sortOrder, true)

	getComments += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	rows, e := db.Query(getComments)
	if DidFail(e, "get posts") {
		return []UserPrefComment{}
	}
	defer rows.Close()

	result := ScanUserPrefComments(rows)
	return result
}

func DBGetUserPrefTags(db *sql.DB, isWatched bool, isOwner bool, userId int64, upvoteAmount int64, downvoteAmount int64,
	tags []string, upvotes int64, downvotes int64,
	sortOrder SortOrder, search string, limit int64, offset int64, startDate string, endDate string) []UserPrefTag {

	kind := upTag
	if isWatched {
		kind = upWatchTag
	}
	getTags := fmt.Sprintf(`SELECT %s, RATIO(up.upvotes, up.downvotes) AS cred, up.upvotes * RATIO(up.upvotes, up.downvotes) AS score 
	FROM UserPref up
	JOIN tags p ON up.pid = p.id
	JOIN Users u ON u.id = up.uid
	JOIN Users x ON up.uid = x.id 
	WHERE up.kind=%d AND up.uid=%d
	`, SQLFieldsForUserPrefTag(), kind, userId)

	if !isOwner {
		if isWatched {
			getTags += "AND false "
		} else {
			getTags += "AND x.publicTagVotes "
		}
	}
	if len(startDate) > 0 && len(endDate) > 0 {
		getTags += fmt.Sprintf("AND up.updatedOn BETWEEN (TIMESTAMP '%s') AND (TIMESTAMP '%s') ", startDate, endDate)
	} else if len(startDate) > 0 {
		getTags += fmt.Sprintf("AND up.updatedOn > (TIMESTAMP '%s') ", startDate)
	} else if len(endDate) > 0 {
		getTags += fmt.Sprintf("AND up.updatedOn < (TIMESTAMP '%s') ", endDate)
	}
	if upvoteAmount > 0 {
		getTags += fmt.Sprintf("AND up.upvotes >= %d ", upvoteAmount)
	} else if upvoteAmount < 0 {
		getTags += fmt.Sprintf("AND up.upvotes < %d ", -upvoteAmount)
	}
	if downvoteAmount > 0 {
		getTags += fmt.Sprintf("AND up.downvotes >= %d ", downvoteAmount)
	} else if downvoteAmount < 0 {
		getTags += fmt.Sprintf("AND up.downvotes < %d ", -downvoteAmount)
	}
	if len(tags) > 0 {
		tagArray := SQLFormattedArray(tags)
		getTags += fmt.Sprintf("AND ARRAY[p.name] <@ %s\n", tagArray)
	}
	if upvotes > 0 {
		getTags += fmt.Sprintf("AND p.upvotes >= %d\n", upvotes)
	} else if upvotes < 0 {
		getTags += fmt.Sprintf("AND p.upvotes < %d\n", -upvotes)
	}
	if downvotes > 0 {
		getTags += fmt.Sprintf("p.downvotes >= %d\n", downvotes)
	} else if downvotes < 0 {
		getTags += fmt.Sprintf("p.downvotes < %d\n", -downvotes)
	}
	if len(search) > 0 {
		search, _ := DBPrepareSearchString(db, search)
		if len(search) > 0 {
			getTags += fmt.Sprintf("AND p.Name @@ to_tsquery('%s') ", search)
		}
	}

	getTags += SQLSortOrder(sortOrder, true)
	getTags += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	rows, e := db.Query(getTags)
	if DidFail(e, "get tags", getTags) {
		return []UserPrefTag{}
	}

	result := ScanUserPrefTags(rows)

	return result
}

type UserCont struct {
	pid int64
	sid int64
}

func SQLFieldsForUserCont() string {
	return "pid, sid"
}

func SQLFieldsForUserContPost() string {
	return "p.id, p.userId, p.content, p.tags, p.createdAt, p.location, p.upvotes AS item_up, p.downvotes AS item_down, p.CommentCount, p.trashed, " +
		"u.id, u.name, u.registerDate, u.upvotes, u.downvotes"
}

func SQLFieldsForUserContComment() string {
	return "p.id, p.postId, p.userId, p.replyId, p.content, p.createdAt, p.replyCount, p.upvotes AS item_up, p.downvotes AS item_down, p.trashed, " +
		"u.id, u.name, u.registerDate, u.upvotes, u.downvotes"
}

func ScanUserCont(row *sql.Row) (UserCont, error) {
	up := UserCont{}
	e := row.Scan(&up.pid, &up.sid)
	return up, e
}

func ScanUserConts(rows *sql.Rows) []UserCont {
	result := []UserCont{}
	var e error
	for rows.Next() {
		up := UserCont{}
		e = rows.Scan(&up.pid, &up.sid)
		if DidFail(e, "scan user pref") {
			continue
		}
		result = append(result, up)
	}
	return result
}

type UserContKind int

const (
	ucpCreated UserContKind = iota
	ucpViewed
	ucpReadLater
	ucpUserFollow
	ucpUserIgnored
	ucpUserRecommended
	ucpTagFollow
)

func DBCreateUserCont(db *sql.DB, kind UserContKind, userId int64, postId int64, commentId int64) UserCont {
	query := fmt.Sprintf(`
	INSERT INTO UserCont(uid, pid, sid, kind)
	VALUES(%d, %d, %d, %d)
	ON CONFLICT (uid, pid, sid, kind) DO NOTHING
	RETURNING %s;
	`, userId, postId, commentId, kind, SQLFieldsForUserCont())
	rows, e := db.Query(query)
	if DidFail(e, "insert into user cont") {
		return UserCont{}
	}
	userCont := ScanUserConts(rows)
	if len(userCont) > 0 {
		return userCont[0]
	} else {
		return UserCont{}
	}
}

func DBRefreshUserContRecommended(db *sql.DB, userId int64, postId []int64) {
	removeOld := fmt.Sprintf(`
	DELETE FROM UserCont
	WHERE kind=%d AND uid=%d 
	AND (now() at time zone ('utc')) - addedOn > INTERVAL '18 HOURS';
	`, ucpUserRecommended, userId)
	_, e := db.Exec(removeOld)
	if DidFail(e, "remove old recommendations") {
		return
	}

	values := SQLFormattedIndexList(postId, func(id int64) string {
		return fmt.Sprintf("(%d, %d, -1, %d)", userId, id, ucpUserRecommended)
	})
	addNew := fmt.Sprintf(`
	INSERT INTO UserCont (uid, pid, sid, kind)
	VALUES %s
	ON CONFLICT DO NOTHING;
	`, values)
	_, e = db.Exec(addNew)
	if DidFail(e, "add new recommendations") {
		return
	}
}

func DBDeleteUserCont(db *sql.DB, kind UserContKind, userId int64, postId int64, commentId int64) UserCont {
	query := fmt.Sprintf(`
	DELETE FROM UserCont
	WHERE uid=%d AND pid=%d AND sid=%d AND kind=%d
	RETURNING %s;
	`, userId, postId, commentId, kind, SQLFieldsForUserCont())
	row := db.QueryRow(query)
	userCont, e := ScanUserCont(row)
	if DidFail(e, "delete from user cont") {
		return UserCont{}
	}
	return userCont
}

func DBGetUserContPost(db *sql.DB, isOwner bool, kind UserContKind, userId int64, tags []int64,
	location []string, upvotes int64, downvotes int64, sortOrder SortOrder, search string,
	limit int64, offset int64, startDate string, endDate string) []PostResult {
	query := fmt.Sprintf(`SELECT %s, RATIO(p.upvotes, p.downvotes) AS cred, p.upvotes * RATIO(p.upvotes, p.downvotes) AS score 
	FROM UserCont up 
	JOIN posts p ON up.pid = p.id 
	JOIN users u ON p.userId = u.id
	JOIN Users x ON up.uid = x.id 
	WHERE up.uid = %d AND p.trashed=false AND up.sid <= 0 AND kind = %d
	`, SQLFieldsForPostResultAlias(), userId, kind)

	if len(search) > 0 {
		search, altTags := DBPrepareSearchString(db, search)
		tags = append(tags, altTags...)
		if len(search) > 0 {
			query += fmt.Sprintf("AND p.Content @@ to_tsquery('%s') ", search)
		}
	}
	if !isOwner {
		switch kind {
		case ucpReadLater:
			query += "AND x.publicReadLater "
		case ucpViewed:
			query += "AND x.publicViews "
		}
	}
	if len(tags) > 0 {
		queryTags := SQLFormattedIndexArray(tags)
		query += fmt.Sprintf("AND %s && p.tags ", queryTags)
	}
	if len(location) > 0 {
		locIndex := DBCreateLocation(db, location)
		queryLoc := SQLFormattedIndexArray(locIndex)
		query += fmt.Sprintf("AND p.location @> %s ", queryLoc)
	}
	if upvotes > 0 {
		if upvotes > 0 {
			query += fmt.Sprintf("AND p.upvotes >= %d ", upvotes)
		} else {
			query += fmt.Sprintf("AND p.upvotes < %d ", -upvotes)
		}
	}
	if downvotes > 0 {
		if upvotes > 0 {
			query += fmt.Sprintf("AND p.downvotes >= %d ", upvotes)
		} else {
			query += fmt.Sprintf("AND p.downvotes < %d ", -upvotes)
		}
	}
	dateField := "createdAt"
	if kind == ucpReadLater || kind == ucpViewed {
		dateField = "addedOn"
	}
	if len(startDate) > 0 && len(endDate) > 0 {
		query += fmt.Sprintf("AND %s BETWEEN (TIMESTAMP '%s') AND (TIMESTAMP '%s') ", dateField, startDate, endDate)
	} else if len(startDate) > 0 {
		query += fmt.Sprintf("AND %s > (TIMESTAMP '%s') ", dateField, startDate)
	} else if len(endDate) > 0 {
		query += fmt.Sprintf("AND %s < (TIMESTAMP '%s') ", dateField, endDate)
	}

	query += SQLSortOrder(sortOrder, false)

	query += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	rows, e := db.Query(query)
	result := []PostResult{}
	if DidFail(e, "get user content") {
		return result
	}
	result = ScanPostResults(rows, false, false)

	return result
}

func DBGetUserContComments(db *sql.DB, userId int64, authorId int64, replyId int64, isReview bool,
	upvotes int64, downvotes int64, sortOrder SortOrder, search string,
	limit int64, offset int64, startDate string, endDate string) []CommentResult {
	getComments := fmt.Sprintf(`SELECT %s, RATIO(p.upvotes, p.downvotes) AS cred, p.upvotes * RATIO(p.upvotes, p.downvotes) AS score 
		FROM UserCont up 
		JOIN Comments p ON up.pid = p.postId AND up.sid = p.id
		JOIN Users u ON p.userId = u.id
		WHERE up.sid > 0 AND up.uid = %d AND p.trashed=false 
		`, SQLFieldsForCommentResultAlias(), userId)

	if len(startDate) > 0 && len(endDate) > 0 {
		getComments += fmt.Sprintf("AND p.createdAt BETWEEN (TIMESTAMP '%s') AND (TIMESTAMP '%s') ", startDate, endDate)
	} else if len(startDate) > 0 {
		getComments += fmt.Sprintf("AND p.createdAt > (TIMESTAMP '%s') ", startDate)
	} else if len(endDate) > 0 {
		getComments += fmt.Sprintf("AND p.createdAt < (TIMESTAMP '%s') ", endDate)
	}
	if authorId > 0 {
		getComments += fmt.Sprintf("AND p.userId = %d ", authorId)
	}
	if replyId > 0 {
		getComments += fmt.Sprintf("AND p.replyId = %d ", replyId)
	}
	if upvotes > 0 {
		getComments += fmt.Sprintf("AND p.upvotes >= %d ", upvotes)
	} else if upvotes < 0 {
		getComments += fmt.Sprintf("AND p.upvotes < %d ", -upvotes)
	}
	if downvotes > 0 {
		getComments += fmt.Sprintf("AND p.downvotes >= %d ", downvotes)
	} else if downvotes < 0 {
		getComments += fmt.Sprintf("AND p.downvotes < %d ", -downvotes)
	}
	if len(search) > 0 {
		search, _ := DBPrepareSearchString(db, search)
		if len(search) > 0 {
			getComments += fmt.Sprintf("AND p.Content @@ to_tsquery('%s') ", search)
		}
	}
	if isReview {
		getComments += "AND p.isReview=true "
	} else {
		getComments += "AND p.isReview=false "
	}

	getComments += SQLSortOrder(sortOrder, false)

	getComments += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	rows, e := db.Query(getComments)
	if DidFail(e, "get posts") {
		return []CommentResult{}
	}

	result := ScanCommentResults(rows, false, false)
	return result
}

func DBGetUserContUsers(db *sql.DB, isOwner bool, userId int64, kind UserContKind, limit int64, offset int64, startDate string, endDate string) []UserProfile {
	permission := ""
	if !isOwner {
		switch kind {
		case ucpUserFollow:
			permission = "AND x.publicFollowing "
		case ucpUserIgnored:
			permission = "AND x.publicIgnored "
		}
	}

	query := fmt.Sprintf(`SELECT %s 
	FROM UserCont up
	JOIN Users p ON up.pid = p.id
	JOIN Users x ON up.uid = x.id 
	WHERE up.uid = %d AND up.sid <= 0 AND up.kind = %d %s
	`, SQLFieldsForUserProfile(), userId, kind, permission)
	if len(startDate) > 0 && len(endDate) > 0 {
		query += fmt.Sprintf("AND up.addedOn BETWEEN (TIMESTAMP '%s') AND (TIMESTAMP '%s') ", startDate, endDate)
	} else if len(startDate) > 0 {
		query += fmt.Sprintf("AND up.addedOn > (TIMESTAMP '%s') ", startDate)
	} else if len(endDate) > 0 {
		query += fmt.Sprintf("AND up.addedOn < (TIMESTAMP '%s') ", endDate)
	}
	if limit > 0 {
		query += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)
	}

	rows, e := db.Query(query)
	result := []UserProfile{}
	if DidFail(e, "get user content") {
		return result
	}
	result = ScanUserProfiles(rows, false, false, false)

	return result
}

func DBGetUserContTag(db *sql.DB, isOwner bool, userId int64, kind UserContKind, limit int64, offset int64, startDate string, endDate string) []Tag {
	permission := ""
	if !isOwner {
		switch kind {
		case ucpTagFollow:
			permission = "AND x.publicTagFollow "
		}
	}

	query := fmt.Sprintf(`SELECT %s 
	FROM UserCont up
	JOIN Tags p ON up.pid = p.id
	JOIN Users x ON up.uid = x.id
	WHERE up.uid = %d AND up.sid <= 0 AND up.kind = %d %s
	`, SQLFieldsForTag(), userId, kind, permission)
	if len(startDate) > 0 && len(endDate) > 0 {
		query += fmt.Sprintf("AND up.addedOn BETWEEN (TIMESTAMP '%s') AND (TIMESTAMP '%s') ", startDate, endDate)
	} else if len(startDate) > 0 {
		query += fmt.Sprintf("AND up.addedOn > (TIMESTAMP '%s') ", startDate)
	} else if len(endDate) > 0 {
		query += fmt.Sprintf("AND up.addedOn < (TIMESTAMP '%s') ", endDate)
	}
	query += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	rows, e := db.Query(query)
	result := []Tag{}
	if DidFail(e, "get user content") {
		return result
	}
	result = ScanTags(rows, false, false, false)

	return result
}
