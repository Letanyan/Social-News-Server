package main

import "database/sql"

func DBIapExists(db *sql.DB, userId int64, platform string, productId string, data string) bool {
	query := `
	SELECT userId 
	FROM Iap
	WHERE userId=$1 AND platform=$2 AND productId=$3 AND data=$4 
	`
	rows, e := db.Query(query, userId, platform, productId, data)
	if DidFail(e, "query iap exists", query) {
		return false
	}
	defer rows.Close()
	var id int64
	found := false
	for rows.Next() {
		rows.Scan(&id)
		found = true
	}
	return found
}

func DBIapInsert(db *sql.DB, userId int64, platform string, productId string, data string) bool {
	if DBIapExists(db, userId, platform, productId, data) {
		return false
	}

	query := `
	INSERT INTO Iap(userId, platform, productId, data)
	VALUES ($1, $2, $3, $4)
	ON CONFLICT DO NOTHING;
	`
	_, e := db.Exec(query, userId, platform, productId, data)
	return !DidFail(e, "insert iap for user", query)
}
