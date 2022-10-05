package main

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/lib/pq"
)

type Tag struct {
	ID        int64
	Name      string
	UpdatedAt time.Time
	Location  []string
	Upvotes   float64
	Downvotes float64
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
		tagIndices = append(tagIndices, tag.ID)
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

func DBGetTags(db *sql.DB, id int64, tags []string, location []string, upvotes int64, downvotes int64, sortOrder SortOrder, limit int64, offset int64) []Tag {
	getTags := `SELECT id, name, updatedAt, location, upvotes, downvotes, RATIO(upvotes, downvotes) AS cred, upvotes * RATIO(upvotes, downvotes) AS score 
	FROM tags 
	`

	if id != 0 {
		getTags += fmt.Sprintf("WHERE id = %d\n", id)
	} else {
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
		setCondition := ""
		if len(tagClause) > 0 && len(locClause) > 0 {
			setCondition = tagClause + " AND " + locClause
		} else if len(tagClause) > 0 {
			setCondition = tagClause
		} else if len(locClause) > 0 {
			setCondition = locClause
		}

		upClause := ""
		if upvotes > 0 {
			upClause = fmt.Sprintf("upvotes > %d", upvotes)
		} else if upvotes < 0 {
			upClause = fmt.Sprintf("upvotes < %d", -upvotes)
		}
		downClause := ""
		if downvotes > 0 {
			downClause = fmt.Sprintf("downvotes > %d", downvotes)
		} else if upvotes < 0 {
			downClause = fmt.Sprintf("downvotes < %d", -downvotes)
		}
		voteCondition := ""
		if len(upClause) > 0 && len(downClause) > 0 {
			voteCondition = upClause + " AND " + downClause
		} else if len(upClause) > 0 {
			voteCondition = upClause
		} else if len(downClause) > 0 {
			voteCondition = downClause
		}

		condition := ""
		if len(voteCondition) > 0 && len(setCondition) > 0 {
			condition = voteCondition + " AND " + setCondition
		} else if len(voteCondition) > 0 {
			condition = voteCondition
		} else if len(setCondition) > 0 {
			condition = setCondition
		}

		if len(condition) > 0 {
			getTags += "WHERE " + condition + "\n"
		}
	}
	if id != 0 {
		getTags += SQLSortOrder(sortOrder)
		getTags += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)
	}

	fmt.Println(getTags)
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
