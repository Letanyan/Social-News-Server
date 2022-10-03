package main

import (
	"database/sql"
	"fmt"

	"github.com/lib/pq"
)

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

func DBVoteTags(db *sql.DB, userId int64, tags []string, isUpvote bool, location []string) {
	if len(tags) <= 0 {
		return
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
	upsertTags := fmt.Sprintf(`INSERT INTO tags (name, updatedAt, location)
	VALUES %s ON CONFLICT (name, location) DO NOTHING;
	UPDATE tags 
	SET %s = cooldown(%s, updatedAt, '%s', 31536000) + 1,
	%s = cooldown(%s, updatedAt, '%s', 31536000),
	updatedAt = '%s'
	WHERE name = ANY(%s)
	RETURNING id
	`, tagRows, updatedField, updatedField, nowTime, otherField, otherField, nowTime, nowTime, tagArray)
	rows, e := db.Query(upsertTags)
	if DidFail(e, "insert and update tags") {
		return
	}

	tagIndices := []int64{}
	for rows.Next() {
		var tag int64
		rows.Scan(&tag)
		if DidFail(e, "read tag index") {
			continue
		}
		tagIndices = append(tagIndices, tag)
	}

	tagIndexRows := SQLFormattedIndexRows(tagIndices, func(i int64) string { return "-1, 4" })
	tagIndexArray := SQLFormattedIndexArray(tagIndices)
	upsertUserTags := fmt.Sprintf(`INSERT INTO User%dPref (pid, sid, kind)
	VALUES %s ON CONFLICT (kind, pid, sid) DO NOTHING;
	UPDATE User%dPref 
	SET %s = cooldown(%s, updatedAt, '%s', 31536000) + 1,
	%s = cooldown(%s, updatedAt, '%s', 31536000)
	WHERE kind=4 AND pid = ANY(%s)
	`, userId, tagIndexRows, userId, updatedField, updatedField, nowTime, otherField, otherField, nowTime, tagIndexArray)
	_, e = db.Exec(upsertUserTags)
	DidFail(e, "insert and update tags")
}

func DBGetTags(db *sql.DB, tags []string, location []string, sortOrder SortOrder, limit int, offset int) []string {
	getTags := `SELECT id, name, location, upvotes, downvotes, cred, upvotes * cred AS score 
	FROM (SELECT id, name, location, upvotes, downvotes, COALESCE(upvotes / NULLIF(upvotes + downvotes, 0), 0.0) AS cred FROM tags) compute 
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

	switch sortOrder {
	case soScore:
		getTags += "ORDER BY score DESC\n"
	case soCred:
		getTags += "ORDER BY score DESC\n"
	case soUpvotes:
		getTags += "ORDER BY score DESC\n"
	case soDownvotes:
		getTags += "ORDER BY score DESC\n"
	case soControversial:
		getTags += "ORDER BY COALESCE(1 / NULLIF(ABS(cred - 0.5), 0), 9e90) DESC\n"
	case soCreatedAt:
		getTags += "ORDER BY createdAt DESC\n"
	}

	getTags += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	rows, e := db.Query(getTags)
	if DidFail(e, "get tags") {
		return []string{}
	}
	defer rows.Close()

	result := []string{}
	for rows.Next() {
		var id int64
		var name string
		var locs []string
		var upvotes float64
		var downvotes float64
		var cred float64
		var score float64

		e = rows.Scan(&id, &name, pq.Array(&locs), &upvotes, &downvotes, &cred, &score)
		if DidFail(e, "read row") {
			continue
		}
		result = append(result, fmt.Sprint("(", id, ") ", name, " ", fmt.Sprintf("%v", locs), " {", cred, "}[", score, "]"))
	}

	return result
}
