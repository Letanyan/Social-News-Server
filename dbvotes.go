package main

import "fmt"

func SQLMakeVote(kind UserPrefKind, pid int64, sid int64, location []string, upvoteAmount int64, date string) string {
	locArray := SQLFormattedArray(location)
	var updateField string
	isUpvote := upvoteAmount > 0
	if isUpvote {
		updateField = "upvotes"
	} else {
		updateField = "downvotes"
		upvoteAmount = -upvoteAmount
	}

	query := fmt.Sprintf(`
	INSERT INTO Votes(kind, pid, sid, location{date})
	VALUES (%d, %d, %d, %s{date_value})
	ON CONFLICT (kind, pid, sid, location, updatedAt) DO NOTHING;
	UPDATE Votes SET
	%s = %s + %d
	WHERE kind=%d AND pid=%d AND sid=%d AND location=%s AND updatedAt={date_value_res};
	`, kind, pid, sid, locArray, updateField, updateField, upvoteAmount,
		kind, pid, sid, locArray)

	query = ReplaceDateValues(query, date)

	return query
}
