package main

import "database/sql"

func DBAgentPathExists(db *sql.DB, agentId int64, path string) bool {
	query := `
	SELECT agentId 
	FROM Agents
	WHERE agentId=$1 AND path=$2
	`
	rows, e := db.Query(query, agentId, path)
	if DidFail(e, "query iap exists", query) {
		return false
	}
	var id int64
	found := false
	for rows.Next() {
		rows.Scan(&id)
		found = true
	}
	return found
}

func DBAgentPathInsert(db *sql.DB, agentId int64, path string) bool {
	query := `
	INSERT INTO Agents(agentId, path)
	VALUES ($1, $2)
	ON CONFLICT DO NOTHING;
	`
	_, e := db.Exec(query, agentId, path)
	return !DidFail(e, "insert iap for user", query)
}
