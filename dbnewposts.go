package main

import "database/sql"

func DBGetNewPosts(db *sql.DB, userId int64, origin []string, popularIn []string, limit int64, offset int64, start string, end string) []PostResult {
	return []PostResult{}
}
