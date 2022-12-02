package main

import (
	"database/sql"
	"fmt"
)

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

func DBAgentPathRemoveOld(db *sql.DB, monthAgo int) {
	query := fmt.Sprintf(`
	DELETE FROM Agents
	WHERE date < ((now() at time zone 'utc') - interval '%d month')
	`, monthAgo)
	_, e := db.Exec(query)
	DidFail(e, "delete old agent paths", query)
}

func DBDeleteDuplicateAgentPosts(db *sql.DB) {
	agentIds := []int64{}
	for _, agent := range agents {
		agentIds = append(agentIds, agent.ID)
	}
	agentArray := SQLFormattedIndexArray(agentIds)

	query := fmt.Sprintf(`
	DELETE FROM Posts a 
	USING Posts b 
	WHERE a.userId=ANY(%s) AND a.userId=b.userId AND a.id < b.id AND a.Content = b.Content; 
	`, agentArray)
	_, e := db.Exec(query)
	DidFail(e, "remove duplicate agent posts")
}
