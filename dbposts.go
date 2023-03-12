package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

type Post struct {
	ID           int64 `json:"ID,string"`
	UserID       int64
	Content      string
	Tags         []int64
	CreatedAt    time.Time
	Location     []string
	Upvotes      int64
	Downvotes    int64
	CommentCount int32
	Trashed      bool
	Edited       time.Time
	FlagCount    int64 `json:"FlagCount,string"`
	Views        int64

	Score float64
	Cred  float64
	Rank  float64
}

type PostResult struct {
	ID           int64 `json:"ID,string"`
	Author       UserProfile
	Content      string
	Tags         []string
	CreatedAt    time.Time
	Location     []string
	Upvotes      int64
	Downvotes    int64
	CommentCount int32
	Trashed      bool
	Edited       time.Time
	FlagCount    int64 `json:"FlagCount,string"`
	Views        int64

	Score float64
	Cred  float64
	Rank  float64
}

func SQLFieldsForPost() string {
	return "id, userId, content, tags, createdAt, location, upvotes, downvotes, commentCount, trashed, edited, flagCount, views"
}

func SQLFieldsForPostResult() string {
	return "p.id, p.userId, p.content, p.tags, p.createdAt, p.location, p.upvotes, p.downvotes, p.commentCount, p.trashed, p.edited, p.flagCount, p.views, u.id, u.name, u.registerDate, u.upvotes, u.downvotes, u.email"
}

func SQLFieldsForPostResultAlias() string {
	return "p.id, p.userId, p.content, p.tags, p.createdAt, p.location, p.upvotes AS item_up, p.downvotes AS item_down, p.commentCount, p.trashed, p.edited, p.flagCount, p.views, u.id, u.name, u.registerDate, u.upvotes, u.downvotes, u.email"
}

func ScanPost(row *sql.Row) (Post, error) {
	p := Post{}
	e := row.Scan(&p.ID, &p.UserID, &p.Content, pq.Array(&p.Tags), &p.CreatedAt, pq.Array(&p.Location), &p.Upvotes, &p.Downvotes, &p.CommentCount, &p.Trashed, &p.Edited, &p.FlagCount, &p.Views)
	return p, e
}

func ScanPostResult(row *sql.Row) (PostResult, error) {
	p := PostResult{}
	u := UserProfile{}
	var userID int64
	var email string
	e := row.Scan(&p.ID, &userID, &p.Content, pq.Array(&p.Tags), &p.CreatedAt, pq.Array(&p.Location), &p.Upvotes,
		&p.Downvotes, &p.CommentCount, &p.Trashed, &p.Edited, &p.FlagCount, &p.Views, &u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Downvotes, &email)
	u.IsAgent = len(email) == 0
	p.Author = u
	return p, e
}

func ScanPosts(rows *sql.Rows) []Post {
	result := []Post{}
	var e error
	defer rows.Close()
	for rows.Next() {
		p := Post{}
		e = rows.Scan(&p.ID, &p.UserID, &p.Content, pq.Array(&p.Tags), &p.CreatedAt, pq.Array(&p.Location), &p.Upvotes, &p.Downvotes, &p.CommentCount, &p.Trashed, &p.Edited, &p.FlagCount, &p.Views, &p.Cred, &p.Score)
		if DidFail(e, "scan post") {
			continue
		}
		result = append(result, p)
	}

	return result
}

func ScanPostResults(rows *sql.Rows, hasVotes bool, hasRank bool) []PostResult {
	result := []PostResult{}
	var e error
	defer rows.Close()
	for rows.Next() {
		p := PostResult{}
		u := UserProfile{}
		var userID int64
		var up int64
		var down int64
		var email string
		if hasRank {
			if hasVotes {
				e = rows.Scan(&p.ID, &userID, &p.Content, pq.Array(&p.Tags), &p.CreatedAt, pq.Array(&p.Location), &p.Upvotes,
					&p.Downvotes, &p.CommentCount, &p.Trashed, &p.Edited, &p.FlagCount, &p.Views, &u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Downvotes, &email, &up, &down, &p.Cred, &p.Score, &p.Rank)
			} else {
				e = rows.Scan(&p.ID, &userID, &p.Content, pq.Array(&p.Tags), &p.CreatedAt, pq.Array(&p.Location), &p.Upvotes,
					&p.Downvotes, &p.CommentCount, &p.Trashed, &p.Edited, &p.FlagCount, &p.Views, &u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Downvotes, &email, &p.Cred, &p.Score, &p.Rank)
			}
		} else {
			if hasVotes {
				e = rows.Scan(&p.ID, &userID, &p.Content, pq.Array(&p.Tags), &p.CreatedAt, pq.Array(&p.Location), &p.Upvotes,
					&p.Downvotes, &p.CommentCount, &p.Trashed, &p.Edited, &p.FlagCount, &p.Views, &u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Downvotes, &email, &up, &down, &p.Cred, &p.Score)
			} else {
				e = rows.Scan(&p.ID, &userID, &p.Content, pq.Array(&p.Tags), &p.CreatedAt, pq.Array(&p.Location), &p.Upvotes,
					&p.Downvotes, &p.CommentCount, &p.Trashed, &p.Edited, &p.FlagCount, &p.Views, &u.ID, &u.Name, &u.RegisterDate, &u.Upvotes, &u.Downvotes, &email, &p.Cred, &p.Score)
			}
		}

		if DidFail(e, "scan post") {
			continue
		}
		u.IsAgent = len(email) == 0
		p.Author = u
		result = append(result, p)
	}

	return result
}

func DBCreatePost(db *sql.DB, userId int64, content string, createdAt time.Time, tags []string, location []string, locale string) PostResult {
	t := utc()
	nowTime := formatTime(t)
	if !createdAt.IsZero() && createdAt.Year() > 1900 && createdAt.Year() < 2100 {
		nowTime = formatTime(createdAt)
	}

	tagObjects := DBCreateTags(db, tags)
	tagIndices := []int64{}
	for _, t := range tagObjects {
		tagIndices = append(tagIndices, t.ID)
	}
	locIndex := DBCreateLocation(db, location)
	locArray := SQLFormattedIndexArray(locIndex)
	lang := DBLocaleToLanguageConfig(locale)

	insertPost := fmt.Sprintf(`
	INSERT INTO posts(userId, content, tags, createdAt, edited, location, language, contentLocaleVector) 
	VALUES ($1, $2, %s, '%s', '%s', %s, $3, to_tsvector('%s', $2)) 
	RETURNING %s
	`, SQLFormattedIndexArray(tagIndices), nowTime, nowTime, locArray, lang, SQLFieldsForPost())
	row := db.QueryRow(insertPost, userId, content, lang)

	post, e := ScanPost(row)
	if DidFail(e, "create post") {
		return PostResult{}
	}

	DBCreatePostTags(db, post.ID, post.Tags)

	cont := DBCreateUserCont(db, ucpCreated, post.UserID, post.ID, -1)
	if cont.pid == 0 {
		return PostResult{}
	}

	result := DBGetPost(db, post.ID)

	return result
}

func DBUpdatePost(db *sql.DB, postId int64, content string) Post {
	t := utc()
	nowTime := formatTime(t)

	updatePost := fmt.Sprintf(`
	UPDATE Posts 
	SET content=$1, Edited='%s', flagCount=0, contentLocaleVector=to_tsvector(language::regconfig, $1)
	WHERE id=$2
	RETURNING %s
	`, nowTime, SQLFieldsForPost())
	row := db.QueryRow(updatePost, content, postId)
	post, e := ScanPost(row)
	if DidFail(e, "update post ", postId) {
		return Post{}
	}
	return post
}

func DBCreatePostTags(db *sql.DB, postId int64, tags []int64) {
	if len(tags) == 0 {
		return
	}
	tagRows := ""
	for i := range tags {
		comma := ""
		if i != len(tags)-1 {
			comma = ","
		}
		tagRows += fmt.Sprintf("(%d, %d)%s", postId, tags[i], comma)
	}
	query := fmt.Sprintf(`
	INSERT INTO PostTags (postId, tagId)
	VALUES %s ON CONFLICT (postId, tagId) DO NOTHING;
	`, tagRows)
	_, e := db.Exec(query)
	DidFail(e, "insert post tags")
}

func DBVotePost(db *sql.DB, userId int64, postId int64, upvoteAmount int64, location []string) (Post, UserProfile, []Tag, []UserPref) {
	var updateField string
	isUpvote := upvoteAmount > 0
	if isUpvote {
		updateField = "upvotes"
	} else {
		updateField = "downvotes"
		upvoteAmount = -upvoteAmount
	}

	locIndex := DBCreateLocation(db, location)
	locArray := SQLFormattedIndexArray(locIndex)
	voteQuery := SQLMakeVote(upPost, postId, -1, locIndex, upvoteAmount, isUpvote)
	updateVoteForPost := fmt.Sprintf(`
	%s
	UPDATE posts SET
	%s = %s + %d
	WHERE id = %d
	RETURNING %s
	`, voteQuery,
		updateField, updateField, upvoteAmount,
		postId, SQLFieldsForPost())

	row := db.QueryRow(updateVoteForPost)
	post, e := ScanPost(row)
	if DidFail(e, "vote for post ", postId) {
		return Post{}, UserProfile{}, []Tag{}, []UserPref{}
	}

	tagResult, tagPrefs := DBVoteTags(db, userId, post.Tags, upvoteAmount*sign(isUpvote), locArray, true)
	user, userPref := DBVoteForUser(db, userId, post.UserID, upvoteAmount*sign(isUpvote), location)
	userPrefForPost := DBCreateUserPref(db, userId, upPost, postId, -1, upvoteAmount*sign(isUpvote))

	prefs := []UserPref{userPref, userPrefForPost}
	prefs = append(prefs, tagPrefs...)

	return post, user, tagResult, prefs
}

func DBDeletePost(db *sql.DB, postId int64) {
	deleteFromPosts := `UPDATE posts SET trashed=true WHERE id=$1`
	_, e := db.Exec(deleteFromPosts, postId)
	if DidFail(e, "trash post ", postId) {
		return
	}

	// deleteComments := fmt.Sprintf(`UPDATE Comments SET trashed WHERE postId = %d`, postId)
	// _, e = db.Exec(deleteComments)
	// DidFail(e, "delete post table")
}

type SortOrder int

const (
	soScore SortOrder = iota
	soCred
	soUpvotes
	soDownvotes
	soControversial
	soCreatedAt
	soUpdatedAt
	soUpdatedOn
	soAddedOn
	soRank
)

func SortOrderFromString(text string) (SortOrder, error) {
	switch strings.ToLower(text) {
	case "score":
		return soScore, nil
	case "cred":
		return soCred, nil
	case "upvotes":
		return soUpvotes, nil
	case "downvotes":
		return soDownvotes, nil
	case "controversial":
		return soControversial, nil
	case "createdat":
		return soCreatedAt, nil
	case "updatedat":
		return soUpdatedAt, nil
	case "updatedon":
		return soUpdatedOn, nil
	case "addedon":
		return soAddedOn, nil
	case "rank":
		return soRank, nil
	}
	return soUpvotes, errors.New("no known order for " + text)
}

func SQLSortOrder(so SortOrder, usingVotes bool) string {
	switch so {
	case soScore:
		return "ORDER BY score DESC, p.id DESC\n"
	case soCred:
		return "ORDER BY cred DESC, p.upvotes DESC, p.downvotes, p.id DESC\n"
	case soUpvotes:
		if usingVotes {
			return "ORDER BY sec_up DESC, p.id DESC\n"
		} else {
			return "ORDER BY item_up DESC, p.id DESC\n"
		}
	case soDownvotes:
		if usingVotes {
			return "ORDER BY sec_down DESC, p.id DESC\n"
		} else {
			return "ORDER BY item_down DESC, p.id DESC\n"
		}
	case soControversial:
		return "ORDER BY COALESCE(1 / NULLIF(ABS(RATIO(p.upvotes, p.downvotes) - 0.5), 0), 9e90) DESC, p.id DESC\n"
	case soCreatedAt:
		return "ORDER BY createdAt DESC, p.id DESC\n"
	case soUpdatedAt:
		return "ORDER BY updatedAt DESC, p.id DESC\n"
	case soUpdatedOn:
		return "ORDER BY updatedOn DESC, p.id DESC\n"
	case soAddedOn:
		return "ORDER BY addedOn DESC, p.id DESC\n"
	case soRank:
		return "ORDER BY rank, p.id DESC\n"
	}
	return ""
}

func DBGetPost(db *sql.DB, id int64) PostResult {
	getPosts := fmt.Sprintf(`
	SELECT %s
	FROM posts p JOIN users u ON p.userId = u.id 
	WHERE p.id = $1
	`, SQLFieldsForPostResult())

	row := db.QueryRow(getPosts, id)
	post, e := ScanPostResult(row)
	if DidFail(e, "read row from post id ", id, getPosts) {
		return PostResult{}
	}
	return post
}

// ignore userId if equals 0. ignore id if equals 0. ignore tags if empty. ignore location if empty.
func DBGetPosts(db *sql.DB, userId int64, tags []int64, origin []string, popularIn []string,
	upvotes int64, downvotes int64, sortOrder SortOrder, limit int64, offset int64,
	start string, end string, startDate string, endDate string, forUser int64, search string) []PostResult {
	voteTable := "p"
	usingVotesTable := len(popularIn) > 0 || len(startDate) > 0 || len(endDate) > 0
	if usingVotesTable {
		voteTable = "v"
	}

	joins := "JOIN users u ON p.userId = u.id\n"
	cond := []string{"p.trashed = false"}
	if len(start) > 0 && len(end) > 0 {
		cond = append(cond, fmt.Sprintf("p.createdAt BETWEEN (TIMESTAMP '%s') AND (TIMESTAMP '%s')", start, end))
	} else if len(start) > 0 {
		cond = append(cond, fmt.Sprintf("(TIMESTAMP '%s') < p.createdAt", start))
	} else if len(end) > 0 {
		cond = append(cond, fmt.Sprintf("p.createdAt < (TIMESTAMP '%s')", end))
	}
	if userId > 0 {
		cond = append(cond, fmt.Sprintf("p.userId = %d", userId))
	}
	if len(tags) > 0 {
		queryTags := SQLFormattedIndexArray(tags)
		cond = append(cond, fmt.Sprintf("%s && p.tags", queryTags))
	}
	if len(origin) > 0 {
		queryLoc := DBGetLocationIndex(db, origin)
		cond = append(cond, fmt.Sprintf("p.location @> %s", queryLoc))
	}
	if usingVotesTable {
		joins += "JOIN Votes v ON v.pid = p.id\n"
		cond = append(cond, "v.kind=2")
	}

	locArray := ""
	if len(popularIn) > 0 {
		locArray = DBGetLocationIndex(db, popularIn)
	}

	result := []PostResult{}
	// increase total posts to consider by months as more items requested
	for i := 1; i <= 12; i += 1 {
		cond = append(cond, fmt.Sprintf("p.createdAt > ((now() at time zone 'utc') - interval '%d month')", i))
		getPosts := SQLGetItems(db, "Posts p", voteTable, SQLFieldsForPostResultAlias(),
			SQLFieldsForPostResult(), joins, locArray, cond, usingVotesTable,
			upvotes, downvotes,
			sortOrder, limit, offset, startDate, endDate, forUser, search)

		rows, e := db.Query(getPosts)
		if DidFail(e, "get posts\n", getPosts) {
			return result
		}

		result = ScanPostResults(rows, usingVotesTable, len(search) > 0 && sortOrder == soRank)
		if len(result) != 0 {
			break
		}
	}

	if forUser > 0 && len(result) == 0 { // if no more recommended show 2nd degree recommended
		result = DBGetSimilarPosts(db, forUser, 0, sortOrder, limit, offset)
		if len(result) == 0 { // show trending if no recommended
			today := utc()
			lastWeek := today.AddDate(0, 0, -7)

			result = DBGetPosts(db, 0, []int64{}, []string{}, []string{},
				0, 0, soScore, limit, offset, "", "",
				formatTime(lastWeek), formatTime(today), 0, "")

			if len(result) == 0 { // show new post if no trending
				result = DBGetPosts(db, 0, []int64{}, []string{}, []string{},
					0, 0, soCreatedAt, limit, offset, "", "",
					"", "", 0, "")
			}
		}
	}

	return result
}

func DBGetSimilarPosts(db *sql.DB, userId int64, postId int64, sortOrder SortOrder, limit int64, offset int64) []PostResult {
	singlePost := ""
	userPrefsTable := "UserPrefs"
	if postId > 0 {
		userPrefsTable = "UserPref"
		singlePost = fmt.Sprintf("AND pid = %d", postId)
	}

	sourceTable := ""
	if postId > 0 {
		sourceTable = fmt.Sprintf(`
		SELECT t.id id
		FROM Posts p
		JOIN Tags t ON t.id = ANY(p.tags) 
		WHERE p.id=%d
		`, postId)
	} else {
		sourceTable = `
		SELECT t.id id
		FROM UserPrefs up
		JOIN Posts p ON p.id=up.pid
		JOIN Tags t ON t.id=ANY(p.tags)
		WHERE up.kind=2
		`
	}

	getPosts := fmt.Sprintf(`
	WITH
	UserPrefs AS (
		SELECT * 
		FROM UserPref
		WHERE uid=%d
	), Source AS (
		%s
	), SourceVotes AS (
		SELECT src.id AS id, COALESCE(up.upvotes, 0) AS up, COALESCE(up.downvotes, 0) AS down
		FROM Source src
		LEFT JOIN userPref up ON up.pid=src.id
		WHERE up.kind=3 OR up.kind IS NULL
	), Total AS (
		SELECT SUM(up) up, SUM(down) down
		FROM SourceVotes
	), TagScores AS (
		SELECT sv.id AS id, (sv.up - sv.down) / (t.up + t.down) AS value
		FROM SourceVotes sv, Total t
	), Viewed AS (
		SELECT pid, upvotes, downvotes
		FROM %s
		WHERE kind=2 %s
	), UserScores AS (
		SELECT uid, SUM(InverseNumber(ABS(v.upvotes - up.upvotes + v.downvotes - up.downvotes), 1)) AS value
		FROM UserPref up
		JOIN Viewed v ON up.pid=v.pid
		WHERE kind=2
		GROUP BY uid
	)
	SELECT %s, AVG(RATIO(p.upvotes, p.downvotes)) AS cred, SUM(COALESCE(us.value, 0) + COALESCE(ts.value, 0)) AS score
	FROM Posts p
	JOIN UserPref up ON up.pid=p.id
	JOIN Users u ON p.userId=u.id
	LEFT JOIN UserScores us ON us.uid=up.uid
	LEFT JOIN TagScores ts ON ts.id = ANY(p.tags)
	WHERE p.id NOT IN (SELECT pid FROM Viewed)
	GROUP BY %s
	`, userId, sourceTable, userPrefsTable, singlePost, SQLFieldsForPostResultAlias(), SQLFieldsForPostResult())

	getPosts += SQLSortOrder(sortOrder, false)
	getPosts += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	rows, e := db.Query(getPosts)
	result := []PostResult{}
	if DidFail(e, "get posts\n", getPosts) {
		return result
	}

	result = ScanPostResults(rows, false, false)
	return result
}
