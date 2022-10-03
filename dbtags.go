package main

import (
	"database/sql"
	"fmt"
)

func DBVoteTags(db *sql.DB, userId int64, tags []string, isUpvote bool) {
	if len(tags) <= 0 {
		return
	}
	nowTime := formatNow()
	tagRows := SQLFormattedRows(tags, func(s string) string { return "'" + nowTime + "'" })
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
	upsertTags := fmt.Sprintf(`INSERT INTO tags (name, updatedAt)
	VALUES %s ON CONFLICT (name) DO NOTHING;
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
