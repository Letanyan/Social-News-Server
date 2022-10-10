package main

import (
	"fmt"
	"strings"
)

func SQLGetItems(table string, voteTable string, aliasFields string, returnedFields string, joins string,
	popularIn []string, cond []string, usingVotes bool, upvotes int64, downvotes int64,
	sortOrder SortOrder, limit int64, offset int64, startDate string, endDate string) string {
	result := fmt.Sprintf(`
	SELECT %s{agg}, RATIO(p.upvotes, p.downvotes) AS cred, p.upvotes * RATIO(p.upvotes, p.downvotes) AS score
	FROM %s
	%s
	`, aliasFields, table, joins)

	if upvotes > 0 {
		cond = append(cond, fmt.Sprintf("%s.upvotes > %d", voteTable, upvotes))
	} else if upvotes < 0 {
		cond = append(cond, fmt.Sprintf("%s.upvotes < %d", voteTable, -upvotes))
	}
	if downvotes > 0 {
		cond = append(cond, fmt.Sprintf("%s.downvotes > %d", voteTable, downvotes))
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

	if len(cond) > 0 {
		result += "WHERE " + JoinStrings(cond, " AND ") + "\n"
	}

	if usingVotes {
		result += fmt.Sprintf("GROUP BY %s, cred, score\n", returnedFields)
		result = strings.ReplaceAll(result, "{agg}", ", SUM(v.upvotes) AS sec_up, SUM(v.downvotes) AS sec_down")
	} else {
		result = strings.ReplaceAll(result, "{agg}", "")
	}

	result += SQLSortOrder(sortOrder)
	result += fmt.Sprintf("LIMIT %d OFFSET %d", limit, offset)

	return result
}
