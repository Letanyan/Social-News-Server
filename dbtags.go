package main

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
)

type Tag struct {
	id        int64
	name      string
	updatedAt time.Time
	location  []string
	upvotes   float64
	downvotes float64
}

func DBCreateTags(db *sql.DB, tags []string, location []string) {
	if len(tags) <= 0 {
		return
	}
	nowTime := formatNow()
	tagRows := SQLFormattedRows(tags, func(s string) string {
		return "'" + nowTime + "'," + SQLFormattedArray(location)
	})
	upsertTags := fmt.Sprintf(`INSERT INTO tags (name, updatedAt, location)
	VALUES %s ON CONFLICT (name, location) DO NOTHING;
	`, tagRows)
	_, e := db.Exec(upsertTags)
	if DidFail(e, "create tags") {
		return
	}
}

func DBVoteTags(db *sql.DB, userId int64, tags []string, isUpvote bool, location []string) ([]Tag, []UserPref) {
	if len(tags) <= 0 {
		return []Tag{}, []UserPref{}
	}
	nowTime := formatNow()
	tagRows := SQLFormattedRows(tags, func(s string) string {
		return "'" + nowTime + "'," + SQLFormattedArray(location)
	})
	tagArray := SQLFormattedArray(tags)
	updatedField := ""
	otherField := ""
	if isUpvote {
		updatedField = "upvotes"
		otherField = "downvotes"
	} else {
		updatedField = "downvotes"
		otherField = "upvotes"
	}
	locArray := SQLFormattedArray(location)
	upsertTags := fmt.Sprintf(`INSERT INTO tags (name, updatedAt, location)
	VALUES %s ON CONFLICT (name, location) DO NOTHING;
	UPDATE tags 
	SET %s = cooldown(%s, updatedAt, '%s', 31536000) + 1,
	%s = cooldown(%s, updatedAt, '%s', 31536000),
	updatedAt = '%s'
	WHERE name = ANY(%s) AND location @> %s
	RETURNING id, name, updatedAt, location, upvotes, downvotes
	`, tagRows, updatedField, updatedField, nowTime, otherField, otherField, nowTime, nowTime, tagArray, locArray)
	rows, e := db.Query(upsertTags)
	if DidFail(e, "insert and update tags") {
		return []Tag{}, []UserPref{}
	}
	tagResult, e := ScanTagRows(rows, false)
	if DidFail(e, "insert and update tags") {
		return []Tag{}, []UserPref{}
	}

	tagIndices := []int64{}
	for _, tag := range tagResult {
		tagIndices = append(tagIndices, tag.id)
	}

	tagIndexRows := SQLFormattedIndexRows(tagIndices, func(i int64) string { return "-1, 4" })
	tagIndexArray := SQLFormattedIndexArray(tagIndices)
	upsertUserTags := fmt.Sprintf(`INSERT INTO User%dPref (pid, sid, kind)
	VALUES %s ON CONFLICT (kind, pid, sid) DO NOTHING;
	UPDATE User%dPref 
	SET %s = cooldown(%s, updatedAt, '%s', 31536000) + 1,
	%s = cooldown(%s, updatedAt, '%s', 31536000)
	WHERE kind=4 AND pid = ANY(%s)
	RETURNING kind, pid, sid, upvotes, downvotes
	`, userId, tagIndexRows, userId, updatedField, updatedField, nowTime, otherField, otherField, nowTime, tagIndexArray)
	rows, e = db.Query(upsertUserTags)
	if DidFail(e, "insert and update tags") {
		return tagResult, []UserPref{}
	}

	tagPrefs, e := ScanUserPrefRows(rows, false)
	if DidFail(e, "insert and update tags") {
		return tagResult, []UserPref{}
	}

	return tagResult, tagPrefs
}

func DBGetTags(db *sql.DB, tags []string, location []string, sortOrder SortOrder, limit int, offset int) []Tag {
	getTags := `SELECT id, name, location, upvotes, downvotes, RATIO(upvotes, downvotes) AS cred, upvotes * RATIO(upvotes, downvotes) AS score 
	FROM tags 
	`
	tagClause := ""
	if len(tags) > 0 {
		tagArray := SQLFormattedArray(tags)
		tagClause = fmt.Sprintf("ARRAY[name] <@ %s", tagArray)
	}

	locClause := ""
	if len(location) > 0 {
		locArray := SQLFormattedArray(location)
		locClause = fmt.Sprintf("location @> %s", locArray)
	}

	condition := ""
	if len(tagClause) > 0 && len(locClause) > 0 {
		condition = tagClause + " AND " + locClause
	} else if len(tagClause) > 0 {
		condition = tagClause
	} else if len(locClause) > 0 {
		condition = locClause
	}

	if len(condition) > 0 {
		getTags += "WHERE " + condition + "\n"
	}

	getTags += SQLSortOrder(sortOrder)

	getTags += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	rows, e := db.Query(getTags)
	if DidFail(e, "get tags") {
		return []Tag{}
	}
	defer rows.Close()

	result, e := ScanTagRows(rows, true)
	if DidFail(e, "scan tags") {
		return result
	}

	return result
}

func ScanTagRows(rows *sql.Rows, includeScore bool) ([]Tag, error) {
	result := []Tag{}
	var e error
	for rows.Next() {
		var id int64
		var name string
		var updatedAt time.Time
		var locs []string
		var upvotes float64
		var downvotes float64
		var cred float64
		var score float64

		if includeScore {
			e = rows.Scan(&id, &name, &updatedAt, pq.Array(&locs), &upvotes, &downvotes, &cred, &score)
		} else {
			e = rows.Scan(&id, &name, &updatedAt, pq.Array(&locs), &upvotes, &downvotes)
		}
		if DidFail(e, "read row") {
			continue
		}
		tag := Tag{id, name, updatedAt, locs, upvotes, downvotes}
		result = append(result, tag)
	}
	return result, e
}
