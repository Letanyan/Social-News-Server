package main

import (
	"database/sql"
	"fmt"
)

type Tag struct {
	ID        int64
	Name      string
	Upvotes   int64
	Downvotes int64

	Score float64
	Cred  float64
	Rank  float64
}

func SQLFieldsForTag() string {
	return "p.id, p.name, p.upvotes, p.downvotes"
}

func SQLFieldsForTagAlias() string {
	return "p.id, p.name, p.upvotes AS item_up, p.downvotes AS item_down"
}

func ScanTags(rows *sql.Rows, includeScore bool, hasVotes bool, hasRank bool) []Tag {
	result := []Tag{}
	var e error
	for rows.Next() {
		var up int64
		var down int64
		t := Tag{}
		if hasRank {
			if hasVotes {
				if includeScore {
					e = rows.Scan(&t.ID, &t.Name, &t.Upvotes, &t.Downvotes, &up, &down, &t.Cred, &t.Score, &t.Rank)
				} else {
					e = rows.Scan(&t.ID, &t.Name, &t.Upvotes, &t.Downvotes, &up, &down, &t.Rank)
				}
			} else {
				if includeScore {
					e = rows.Scan(&t.ID, &t.Name, &t.Upvotes, &t.Downvotes, &t.Cred, &t.Score, &t.Rank)
				} else {
					e = rows.Scan(&t.ID, &t.Name, &t.Upvotes, &t.Downvotes, &t.Rank)
				}
			}
		} else {
			if hasVotes {
				if includeScore {
					e = rows.Scan(&t.ID, &t.Name, &t.Upvotes, &t.Downvotes, &up, &down, &t.Cred, &t.Score)
				} else {
					e = rows.Scan(&t.ID, &t.Name, &t.Upvotes, &t.Downvotes, &up, &down)
				}
			} else {
				if includeScore {
					e = rows.Scan(&t.ID, &t.Name, &t.Upvotes, &t.Downvotes, &t.Cred, &t.Score)
				} else {
					e = rows.Scan(&t.ID, &t.Name, &t.Upvotes, &t.Downvotes)
				}
			}
		}
		if DidFail(e, "read row") {
			continue
		}
		result = append(result, t)
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
	tagArray := SQLFormattedArray(tags)

	upsertTags := fmt.Sprintf(`
	INSERT INTO tags (name)
	VALUES %s ON CONFLICT (name) DO NOTHING;
	SELECT %s
	FROM Tags p
	WHERE ARRAY[Name] <@ %s;
	`, tagRows, SQLFieldsForTag(), tagArray)
	rows, e := db.Query(upsertTags)
	if DidFail(e, "create tags") {
		return []Tag{}
	}
	result := ScanTags(rows, false, false, false)

	return result
}

func DBVoteTags(db *sql.DB, userId int64, tags []int64, upvoteAmount int64, location []string, isPublic bool) ([]Tag, []UserPref) {
	if len(tags) <= 0 {
		return []Tag{}, []UserPref{}
	}
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
	upsertTags := fmt.Sprintf(`
	UPDATE tags p SET 
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
	tagResult := ScanTags(rows, false, false, false)

	tagIndices := []int64{}
	for _, tag := range tagResult {
		tagIndices = append(tagIndices, tag.ID)
	}

	voteKind := upTag
	if !isPublic {
		voteKind = upWatchTag
	}
	tagIndexRows := SQLFormattedIndexList(tagIndices, func(i int64) string {
		return fmt.Sprintf("(%d, %d, %d, -1)", userId, voteKind, i)
	})
	tagVoteRows := SQLFormattedIndexList(tagIndices, func(i int64) string {
		return fmt.Sprintf("(%d, %d, -1, %s)", voteKind, i, locArray)
	})
	tagIndexArray := SQLFormattedIndexArray(tagIndices)
	upsertUserTags := fmt.Sprintf(`
	INSERT INTO votes(kind, pid, sid, location) 
	VALUES %s ON CONFLICT (kind, pid, sid, location, updatedAt) DO NOTHING;
	UPDATE votes SET
	%s = %s + %d
	WHERE kind=%d AND pid=ANY(%s) AND location=%s AND updatedAt='%s';

	INSERT INTO UserPref (uid, kind, pid, sid)
	VALUES %s ON CONFLICT (uid, kind, pid, sid) DO NOTHING;
	UPDATE UserPref SET 
	%s = %s + %d,
	updatedOn = (now() at time zone 'utc')
	WHERE kind=%d AND uid=%d AND pid = ANY(%s)
	RETURNING %s, 0.0, 0.0
	`, tagVoteRows,
		updatedField, updatedField, upvoteAmount,
		voteKind, tagIndexArray, locArray, utc().Format("2006-01-02"),
		tagIndexRows,
		updatedField, updatedField, upvoteAmount,
		voteKind, userId, tagIndexArray, SQLFieldsForUserPref())

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

func DBGetTagsFromIDs(db *sql.DB, ids []int64) []Tag {
	query := fmt.Sprintf(`
	SELECT %s
	FROM Tags p
	WHERE ARRAY[p.id] <@ %s
	`, SQLFieldsForTagAlias(), SQLFormattedIndexArray(ids))
	rows, e := db.Query(query)
	result := []Tag{}
	if DidFail(e, "get tags by id") {
		return result
	}
	result = ScanTags(rows, false, false, false)
	return result
}

func DBGetTags(db *sql.DB, id int64, tags []string, popularIn []string,
	upvotes int64, downvotes int64, sortOrder SortOrder, limit int64, offset int64,
	startDate string, endDate string, forUser int64, search string) []Tag {
	voteTable := "p"
	if len(popularIn) > 0 {
		voteTable = "v"
	}

	usingVotesTable := len(popularIn) > 0 || len(startDate) > 0 || len(endDate) > 0
	cond := []string{}
	joins := ""
	if id > 0 {
		cond = append(cond, fmt.Sprintf("p.id = %d\n", id))
	} else {
		if len(tags) > 0 {
			tagArray := SQLFormattedArray(tags)
			cond = append(cond, fmt.Sprintf("ARRAY[p.name] <@ %s", tagArray))
		}

		if usingVotesTable {
			joins += "JOIN votes v ON v.pid = p.id\n"
			cond = append(cond, "kind=3")
		}
	}

	getTags := SQLGetItems("Tags p", voteTable, SQLFieldsForTagAlias(),
		SQLFieldsForTag(), joins, popularIn, cond, usingVotesTable,
		upvotes, downvotes,
		sortOrder, limit, offset, startDate, endDate, forUser, search)

	rows, e := db.Query(getTags)
	if DidFail(e, "get tags", getTags) {
		return []Tag{}
	}
	defer rows.Close()

	result := ScanTags(rows, true, usingVotesTable, len(search) > 0 && sortOrder == soRank)

	return result
}
