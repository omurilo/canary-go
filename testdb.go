package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	db, err := sql.Open("mysql", "canary:canary@tcp(127.0.0.1:3306)/canary")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query("SELECT player_id, pid, sid, itemtype, count, LENGTH(attributes) FROM player_items")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	hasRows := false
	for rows.Next() {
		hasRows = true
		var player_id, pid, sid, itemtype, count, attrLen int
		rows.Scan(&player_id, &pid, &sid, &itemtype, &count, &attrLen)
		fmt.Printf("Player: %d, pid: %d, sid: %d, itemtype: %d, count: %d, attrLen: %d\n", player_id, pid, sid, itemtype, count, attrLen)
	}
	if !hasRows {
		fmt.Println("Table is empty")
	}
}
