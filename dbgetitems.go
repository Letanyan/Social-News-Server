package main

import (
	"fmt"
	"strings"
)

func SQLGetItems(table string, voteTable string, aliasFields string, returnedFields string, joins string,
	popularIn []string, cond []string, usingVotes bool, upvotes int64, downvotes int64,
	sortOrder SortOrder, limit int64, offset int64,
	startDate string, endDate string, forUser int64, search string) string {

	scoreField := fmt.Sprintf("%s.upvotes * RATIO(%s.upvotes, %s.downvotes)", voteTable, voteTable, voteTable)
	withTable := ""
	if forUser > 0 {
		withTable = fmt.Sprintf(`
		WITH 
		UserPrefs AS (
			SELECT * 
			FROM UserPref
			WHERE uid=%d
		), Total AS (
			SELECT SUM(upvotes) up, SUM(downvotes) down
			FROM UserPrefs
			WHERE kind=3 OR kind=4
		), Scores AS (
			SELECT pid, (upvotes - downvotes) / (total.up + total.down) * (1.1 * dateFrac(updatedOn, now() at time zone 'utc', 2592000)) AS value
			FROM UserPrefs, Total
			WHERE kind=3 OR kind=4
		), UserConts AS (
			SELECT *
			FROM UserCont
			WHERE uid=%d
		), Ignored AS (
			SELECT pid
			FROM UserConts
			WHERE kind=4
		), Viewed AS (
			(
				SELECT pid
				FROM UserConts
				WHERE kind=5
			) 
			UNION 
			(
				SELECT pid
				FROM UserPrefs
				WHERE kind=2
			)
		)
		`, forUser, forUser)

		joins += "JOIN Scores s ON s.pid = ANY(p.tags)"
		cond = append(cond, "p.userId NOT IN (SELECT * FROM Ignored)")
		cond = append(cond, "p.id NOT IN (SELECT * FROM Viewed)")
		scoreField = "SUM(s.value * 1.5 - dateFrac(p.createdAt, now() at time zone 'utc', 2592000))"
	}

	result := fmt.Sprintf(`%s
	SELECT %s{agg}, RATIO(p.upvotes, p.downvotes) AS cred, {score} AS score{rank}
	FROM %s
	%s
	`, withTable, aliasFields, table, joins)

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
	if len(popularIn) > 0 {
		locArray := SQLFormattedArray(popularIn)
		cond = append(cond, fmt.Sprintf("v.location @> %s", locArray))
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
			search, altTags := DBPrepareSearchString(search)
			if table == "Posts p" && len(altTags) > 0 {
				queryTags := SQLFormattedIndexArray(altTags)
				cond = append(cond, fmt.Sprintf("%s && p.tags", queryTags))
			}
			if len(search) > 0 {
				cond = append(cond, fmt.Sprintf("p.content @@ websearch_to_tsquery('%s')", search))
			}
			if sortOrder == soRank {
				result = strings.ReplaceAll(result, "{rank}", fmt.Sprintf(", ts_rank(to_tsvector(p.content), websearch_to_tsquery('%s')) AS rank", search))
			} else {
				result = strings.ReplaceAll(result, "{rank}", "")
			}
		} else if table == "Users p" || table == "Tags p" {
			search, _ := DBPrepareSearchString(search)
			cond = append(cond, fmt.Sprintf("p.name @@ websearch_to_tsquery('%s')", search))
			if sortOrder == soRank {
				result = strings.ReplaceAll(result, "{rank}", fmt.Sprintf(", ts_rank(to_tsvector(p.name), websearch_to_tsquery('%s')) AS rank", search))
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

	if usingVotes {
		result += fmt.Sprintf("GROUP BY %s, cred, score\n", returnedFields)
	} else if forUser > 0 {
		result += fmt.Sprintf("GROUP BY %s, cred\n", returnedFields)
	}

	if usingVotes {
		result = strings.ReplaceAll(result, "{agg}", ", SUM(v.upvotes) AS sec_up, SUM(v.downvotes) AS sec_down")
	} else {
		result = strings.ReplaceAll(result, "{agg}", "")
	}

	result += SQLSortOrder(sortOrder)
	result += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	return result
}
