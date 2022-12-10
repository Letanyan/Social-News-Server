package main

import (
	"database/sql"
	"fmt"
	"strings"
)

func SQLGetItems(db *sql.DB, table string, voteTable string, aliasFields string, returnedFields string, joins string,
	popularIn string, cond []string, usingVotes bool, upvotes int64, downvotes int64,
	sortOrder SortOrder, limit int64, offset int64,
	startDate string, endDate string, forUser int64, search string) string {

	ratioFunc := "Ratio"
	weightedRatioFunc := "WeightRatio"
	if usingVotes || forUser > 0 {
		ratioFunc = "SumRatio"
		weightedRatioFunc = "SumWeightedRatio"
	}

	scoreField := fmt.Sprintf("%s(%s.upvotes, %s.upvotes, %s.downvotes)", weightedRatioFunc, voteTable, voteTable, voteTable)
	if table == "Users p" && !usingVotes {
		scoreField = "WeightRatio(p.upvotes, p.upvotes + p.investment, p.downvotes + p.investment)"
	}
	withTable := ""
	if forUser > 0 {
		withTable = fmt.Sprintf(`
		WITH 
		UserPrefs AS (
			SELECT * 
			FROM UserPref
			WHERE uid=%d
		), UserConts AS (
			SELECT *
			FROM UserCont
			WHERE uid=%d
		),

		TagTotal AS (
			SELECT SUM(upvotes) up, SUM(downvotes) down
			FROM UserPrefs
			WHERE kind=3 OR kind=4 -- liked tags or watched tags
		), TagScores AS (
			WITH TEx AS (
				SELECT pid id, (upvotes - downvotes) / (t.up + t.down) * (1.1 - dateFrac(updatedOn, now() at time zone 'utc', 60*60*24*30)) AS value
				FROM UserPrefs, TagTotal t
				WHERE kind=3 OR kind=4 -- liked tags or watched tags
			), TImp AS (
				SELECT t.id id, 0.2 AS value
				FROM UserConts uc
				JOIN Tags t ON t.id=uc.pid
				WHERE uc.kind=6 -- favourite tag
			)
			SELECT id, SUM(value) AS Value
			FROM (
				SELECT id, value FROM TEx
				UNION ALL
				SELECT id, value FROM TImp
			) T GROUP BY id
		),
		
		
		UserTotal AS (
			SELECT SUM(upvotes) up, SUM(downvotes) down
			FROM UserPrefs
			WHERE kind=0 -- liked users
		), UserScores AS (
			WITH USEx AS (
				SELECT pid id, (upvotes - downvotes) / (t.up + t.down) * (1.1 - dateFrac(updatedOn, now() at time zone 'utc', 60*60*24*30)) AS value
				FROM UserPrefs, UserTotal t
				WHERE kind=0 -- liked users
			), USImp AS (
				SELECT uc.pid id, 0.2 AS value
				FROM UserConts uc
				WHERE uc.kind=3 -- following user
			) 
			SELECT id, SUM(value) AS Value
			
			FROM (
				SELECT id, value FROM USEx
				UNION ALL
				SELECT id, value FROM USImp
			) T GROUP BY id
		),
		
		Ignored AS (
			SELECT pid
			FROM UserConts
			WHERE kind=4 -- ignored user
		), Viewed AS (
			(
				SELECT pid
				FROM UserConts
				WHERE kind=5 OR kind=1 -- already recommended or viewed
			) 
			UNION 
			(
				SELECT pid
				FROM UserPrefs
				WHERE kind=2 -- voted for posts
			)
		)
		`, forUser, forUser)

		joins += "JOIN TagScores ts ON ts.id = ANY(p.tags)\n"
		joins += "JOIN UserScores us ON us.id = p.userId\n"
		cond = append(cond, "p.userId NOT IN (SELECT * FROM Ignored)")
		cond = append(cond, "p.id NOT IN (SELECT * FROM Viewed)")
		scoreField = "scoreValueFactor(ts.value, us.value, 1.5 - dateFrac(p.createdAt, now() at time zone 'utc', 60*60*12))"
	}

	result := fmt.Sprintf(`%s
	SELECT %s{agg}, %s(%s.upvotes, %s.downvotes) AS cred, {score} AS score{rank}
	FROM %s
	%s
	`, withTable, aliasFields, ratioFunc, voteTable, voteTable, table, joins)

	result = strings.ReplaceAll(result, "{score}", scoreField)

	if upvotes > 0 {
		cond = append(cond, fmt.Sprintf("%s.upvotes >= %d", voteTable, upvotes))
	} else if upvotes < 0 {
		cond = append(cond, fmt.Sprintf("%s.upvotes < %d", voteTable, -upvotes))
	}
	if downvotes > 0 {
		cond = append(cond, fmt.Sprintf("%s.downvotes >= %d", voteTable, downvotes))
	} else if downvotes < 0 {
		cond = append(cond, fmt.Sprintf("%s.downvotes < %d", voteTable, -downvotes))
	}
	// we use 2 because the empty array '{}' is 2 characters
	if len(popularIn) > 2 {
		cond = append(cond, fmt.Sprintf("v.location @> %s", popularIn))
	}
	if len(startDate) > 0 && len(endDate) > 0 {
		cond = append(cond, fmt.Sprintf("v.updatedAt BETWEEN (TIMESTAMP '%s') AND (TIMESTAMP '%s')\n", startDate, endDate))
	} else if len(startDate) > 0 {
		cond = append(cond, fmt.Sprintf("TIMESTAMP '%s' <= v.updatedAt\n", startDate))
	} else if len(endDate) > 0 {
		cond = append(cond, fmt.Sprintf("TIMESTAMP '%s' > v.updatedAt\n", endDate))
	}
	if len(search) > 0 {
		if table == "Posts p" || table == "Comments p" {
			search, altTags := DBPrepareSearchString(db, search)
			if table == "Posts p" && len(altTags) > 0 {
				queryTags := SQLFormattedIndexArray(altTags)
				cond = append(cond, fmt.Sprintf("%s && p.tags", queryTags))
			}
			if len(search) > 0 {
				cond = append(cond, fmt.Sprintf("p.contentVector @@ to_tsquery('%s')", search))
			}
			if sortOrder == soRank {
				result = strings.ReplaceAll(result, "{rank}", fmt.Sprintf(", ts_rank(p.contentVector, to_tsquery('%s')) AS rank", search))
			} else {
				result = strings.ReplaceAll(result, "{rank}", "")
			}
		} else if table == "Users p" || table == "Tags p" {
			search, _ := DBPrepareSearchString(db, search)
			cond = append(cond, fmt.Sprintf("p.name @@ to_tsquery('%s')", search))
			if sortOrder == soRank {
				result = strings.ReplaceAll(result, "{rank}", fmt.Sprintf(", ts_rank(to_tsvector(p.name), to_tsquery('%s')) AS rank", search))
			} else {
				result = strings.ReplaceAll(result, "{rank}", "")
			}
		}
	} else {
		result = strings.ReplaceAll(result, "{rank}", "")
	}

	if len(cond) > 0 {
		result += "WHERE " + JoinStrings(cond, " AND ") + "\n"
	}

	if usingVotes || forUser > 0 {
		result += fmt.Sprintf("GROUP BY %s\n", returnedFields)
	}

	if usingVotes {
		result = strings.ReplaceAll(result, "{agg}", ", SUM(v.upvotes) AS sec_up, SUM(v.downvotes) AS sec_down")
	} else {
		result = strings.ReplaceAll(result, "{agg}", "")
	}

	result += SQLSortOrder(sortOrder, usingVotes)
	result += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	return result
}
