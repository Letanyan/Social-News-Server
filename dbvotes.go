package main

import "fmt"

func SQLMakeVote(kind UserPrefKind, pid int64, sid int64, location []string, upvoteAmount int64, isUpvote bool) string {
	locArray := SQLFormattedArray(location)
	var updateField string
	if isUpvote {
		updateField = "upvotes"
	} else {
		updateField = "downvotes"
	}

	query := fmt.Sprintf(`
	INSERT INTO Votes(kind, pid, sid, location)
	VALUES (%d, %d, %d, %s)
	ON CONFLICT (kind, pid, sid, location, updatedAt) DO NOTHING;
	UPDATE Votes SET
	%s = %s + %d
	WHERE kind=%d AND pid=%d AND sid=%d AND location=%s AND updatedAt='%s';
	`, kind, pid, sid, locArray, updateField, updateField, upvoteAmount,
		kind, pid, sid, locArray, utc().Format("2006-01-02"))

	return query
}
