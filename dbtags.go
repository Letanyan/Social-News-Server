package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type Tag struct {
	ID        int64
	Name      string
	UpdatedAt time.Time
	Upvotes   float64
	Downvotes float64
}

func SQLFieldsForTag() string {
	return "id, name, updatedAt, upvotes, downvotes"
}

func SQLFieldsForTagAlias() string {
	return "id, name, updatedAt, upvotes AS item_up, downvotes AS item_down"
}

func ScanTags(rows *sql.Rows, includeScore bool) []Tag {
	result := []Tag{}
	var e error
	for rows.Next() {
		var cred float64
		var score float64
		tag := Tag{}
		if includeScore {
			e = rows.Scan(&tag.ID, &tag.Name, &tag.UpdatedAt, &tag.Upvotes, &tag.Downvotes, &cred, &score)
		} else {
			e = rows.Scan(&tag.ID, &tag.Name, &tag.UpdatedAt, &tag.Upvotes, &tag.Downvotes)
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

func DBVoteTags(db *sql.DB, userId int64, tags []int64, upvoteAmount int64, location []string) ([]Tag, []UserPref) {
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
		return fmt.Sprintf("(4, %d, -1, %s)", i, locArray)
	})
	tagIndexArray := SQLFormattedIndexArray(tagIndices)
	upsertUserTags := fmt.Sprintf(`
	INSERT INTO votes(kind, pid, sid, location) 
	VALUES %s ON CONFLICT (kind, pid, sid, location, updatedAt) DO NOTHING;
	UPDATE votes SET
	%s = %s + %d
	WHERE kind=4 AND pid=ANY(%s) AND location=%s;

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

func DBGetTags(db *sql.DB, id int64, tags []string, location []string, upvotes int64, downvotes int64, sortOrder SortOrder, limit int64, offset int64) []Tag {
	voteTable := "v"
	if len(location) > 0 {
		voteTable = "t"
	}
	getTags := fmt.Sprintf(`
	SELECT %s{agg}, RATIO(t.upvotes, t.downvotes) AS cred, t.upvotes * RATIO(t.upvotes, t.downvotes) AS score 
	FROM tags t
	`, SQLFieldsForTagAlias())

	if id != 0 {
		getTags += fmt.Sprintf("WHERE t.id = %d\n", id)
	} else {
		tagClause := ""
		if len(tags) > 0 {
			tagArray := SQLFormattedArray(tags)
			tagClause = fmt.Sprintf("ARRAY[t.name] <@ %s", tagArray)
		}
		locClause := ""
		if len(location) > 0 {
			locArray := SQLFormattedArray(location)
			locClause = fmt.Sprintf("v.kind=4 AND v.location @> %s", locArray)
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
			upClause = fmt.Sprintf("%s.upvotes > %d", voteTable, upvotes)
		} else if upvotes < 0 {
			upClause = fmt.Sprintf("%s.upvotes < %d", voteTable, -upvotes)
		}
		downClause := ""
		if downvotes > 0 {
			downClause = fmt.Sprintf("%s.downvotes > %d", voteTable, downvotes)
		} else if downvotes < 0 {
			downClause = fmt.Sprintf("%s.downvotes < %d", voteTable, -downvotes)
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

		if len(location) > 0 {
			getTags += "JOIN votes v ON v.pid = t.id\n"
		}
		if len(condition) > 0 {
			getTags += "WHERE " + condition + "\n"
		}

		if len(location) > 0 {
			getTags += fmt.Sprintf("GROUP BY %s, cred, score\n", SQLFieldsForUserProfile())
			getTags = strings.ReplaceAll(getTags, "{agg}", ", SUM(v.upvotes) AS sec_up, SUM(v.downvotes) AS sec_down")
		} else {
			getTags = strings.ReplaceAll(getTags, "{agg}", "")
		}
		getTags += SQLSortOrder(sortOrder)
		getTags += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)
	}

	rows, e := db.Query(getTags)
	if DidFail(e, "get tags") {
		return []Tag{}
	}
	defer rows.Close()

	result := ScanTags(rows, true)

	return result
}
