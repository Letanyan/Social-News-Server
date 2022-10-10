package main

import (
	"database/sql"
	"fmt"
)

type Tag struct {
	ID        int64
	Name      string
	Upvotes   float64
	Downvotes float64
}

func SQLFieldsForTag() string {
	return "id, name, upvotes, downvotes"
}

func SQLFieldsForTagAlias() string {
	return "id, name, upvotes AS item_up, downvotes AS item_down"
}

func ScanTags(rows *sql.Rows, includeScore bool) []Tag {
	result := []Tag{}
	var e error
	for rows.Next() {
		var cred float64
		var score float64
		tag := Tag{}
		if includeScore {
			e = rows.Scan(&tag.ID, &tag.Name, &tag.Upvotes, &tag.Downvotes, &cred, &score)
		} else {
			e = rows.Scan(&tag.ID, &tag.Name, &tag.Upvotes, &tag.Downvotes)
		}
		if DidFail(e, "read row") {
			continue
		}
		result = append(result, tag)
	}
	return result
}

func DBCreateTags(db *sql.DB, tags []string) []Tag {
	if len(tags) <= 0 {
		return []Tag{}
	}
	// nowTime := formatNow()
	tagRows := SQLFormattedRows(tags, func(s string) string {
		return ""
	})
	upsertTags := fmt.Sprintf(`
	INSERT INTO tags (name)
	VALUES %s ON CONFLICT (name) DO NOTHING 
	RETURNING %s;
	`, tagRows, SQLFieldsForTag())
	rows, e := db.Query(upsertTags)
	if DidFail(e, "create tags") {
		return []Tag{}
	}
	result := ScanTags(rows, false)

	return result
}

func DBVoteTags(db *sql.DB, userId int64, tags []int64, upvoteAmount int64, location []string, date string) ([]Tag, []UserPref) {
	if len(tags) <= 0 {
		return []Tag{}, []UserPref{}
	}
	// tagRows := SQLFormattedRows(tags, func(s string) string {
	// 	return ""
	// })
	tagArray := SQLFormattedIndexArray(tags)
	updatedField := ""
	locArray := SQLFormattedArray(location)
	isUpvote := upvoteAmount > 0
	if isUpvote {
		updatedField = "upvotes"
	} else {
		updatedField = "downvotes"
		upvoteAmount = -upvoteAmount
	}
	// INSERT INTO tags (name)
	// VALUES %s ON CONFLICT (name) DO NOTHING;
	upsertTags := fmt.Sprintf(`
	UPDATE tags SET 
	%s = %s + %d
	WHERE id = ANY(%s)
	RETURNING %s;
	`, //tagRows,
		updatedField, updatedField, upvoteAmount,
		tagArray, SQLFieldsForTag(),
	)
	rows, e := db.Query(upsertTags)
	if DidFail(e, "insert and update tags") {
		return []Tag{}, []UserPref{}
	}
	tagResult := ScanTags(rows, false)

	tagIndices := []int64{}
	for _, tag := range tagResult {
		tagIndices = append(tagIndices, tag.ID)
	}

	tagIndexRows := SQLFormattedIndexList(tagIndices, func(i int64) string {
		return fmt.Sprintf("(%d, 4, %d, -1)", userId, i)
	})
	tagVoteRows := SQLFormattedIndexList(tagIndices, func(i int64) string {
		return fmt.Sprintf("(4, %d, -1, %s{date_value})", i, locArray)
	})
	tagIndexArray := SQLFormattedIndexArray(tagIndices)
	upsertUserTags := fmt.Sprintf(`
	INSERT INTO votes(kind, pid, sid, location{date}) 
	VALUES %s ON CONFLICT (kind, pid, sid, location, updatedAt) DO NOTHING;
	UPDATE votes SET
	%s = %s + %d
	WHERE kind=4 AND pid=ANY(%s) AND location=%s AND updatedAt={date_value_res};

	INSERT INTO UserPref (uid, pid, sid, kind)
	VALUES %s ON CONFLICT (uid, kind, pid, sid) DO NOTHING;
	UPDATE UserPref SET 
	%s = %s + %d
	WHERE kind=4 AND uid=%d AND pid = ANY(%s)
	RETURNING %s, 0.0, 0.0
	`, tagVoteRows,
		updatedField, updatedField, upvoteAmount,
		tagIndexArray, locArray,
		tagIndexRows,
		updatedField, updatedField, upvoteAmount,
		userId, tagIndexArray, SQLFieldsForUserPref())

	upsertUserTags = ReplaceDateValues(upsertUserTags, date)
	rows, e = db.Query(upsertUserTags)
	if DidFail(e, "insert and update tags") {
		return tagResult, []UserPref{}
	}

	tagPrefs := ScanUserPrefRows(rows)
	if DidFail(e, "insert and update tags") {
		return tagResult, []UserPref{}
	}

	return tagResult, tagPrefs
}

func DBGetTags(db *sql.DB, id int64, tags []string, popularIn []string, upvotes int64, downvotes int64, sortOrder SortOrder, limit int64, offset int64, startDate string, endDate string) []Tag {
	voteTable := "v"
	if len(popularIn) > 0 {
		voteTable = "t"
	}

	usingVotesTable := len(popularIn) > 0 || len(startDate) > 0 || len(endDate) > 0
	cond := []string{}
	joins := ""
	if id != 0 {
		cond = append(cond, fmt.Sprintf("t.id = %d\n", id))
	} else {
		if len(tags) > 0 {
			tagArray := SQLFormattedArray(tags)
			cond = append(cond, fmt.Sprintf("ARRAY[t.name] <@ %s", tagArray))
		}

		if usingVotesTable {
			joins += "JOIN votes v ON v.pid = t.id\n"
			cond = append(cond, "kind=4")
		}
	}

	getTags := SQLGetItems("tags t", voteTable, SQLFieldsForTagAlias(),
		SQLFieldsForTag(), joins, popularIn, cond, usingVotesTable,
		upvotes, downvotes,
		sortOrder, limit, offset, startDate, endDate)

	rows, e := db.Query(getTags)
	if DidFail(e, "get tags") {
		return []Tag{}
	}
	defer rows.Close()

	result := ScanTags(rows, true)

	return result
}
