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
	if isUpvote {
		updatedField = "upvotes"
	} else {
		updatedField = "downvotes"
	}
	upsertTags := fmt.Sprintf(`INSERT INTO tags (name, updatedAt)
	VALUES %s ON CONFLICT (name) DO NOTHING;
	UPDATE tags 
	SET %s = %s + 1 - LEAST(TRUNC(EXTRACT(EPOCH FROM TIMESTAMP '%s')) - TRUNC(EXTRACT(EPOCH FROM updatedAt)), 604800.0) / 604800.0
	WHERE name = ANY(%s)
	RETURNING id
	`, tagRows, updatedField, updatedField, nowTime, tagArray)
	rows, e := db.Query(upsertTags)
	Failed("insert and update tags", e)

	tagIndices := []int64{}
	for rows.Next() {
		var tag int64
		rows.Scan(&tag)
		if Failed("read tag index", e) {
			continue
		}
		tagIndices = append(tagIndices, tag)
	}

	tagIndexRows := SQLFormattedIndexRows(tagIndices, func(i int64) string { return "-1, 4" })
	tagIndexArray := SQLFormattedIndexArray(tagIndices)
	upsertUserTags := fmt.Sprintf(`INSERT INTO User%dPref (pid, sid, kind)
	VALUES %s ON CONFLICT (kind, pid, sid) DO NOTHING;
	UPDATE User%dPref 
	SET %s = %s + 1
	WHERE kind=4 AND pid = ANY(%s)
	`, userId, tagIndexRows, userId, updatedField, updatedField, tagIndexArray)
	_, e = db.Exec(upsertUserTags)
	Failed("insert and update tags", e)
}
