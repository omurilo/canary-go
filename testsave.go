package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	_ "github.com/go-sql-driver/mysql"
	"github.com/opentibiabr/canary-go/internal/db"
	"github.com/opentibiabr/canary-go/internal/game"
)

func main() {
	sqldb, err := sql.Open("mysql", "canary:canary@tcp(127.0.0.1:3306)/canary")
	if err != nil {
		log.Fatal(err)
	}
	defer sqldb.Close()

	d := &db.DB{SQL: sqldb}

	player := &game.Player{
		DBID: 1, // Assume player 1
	}
	
	// mock inventory
	player.Inventory[3] = &game.Item{
		ID:    2854,
		Count: 1,
	}
	player.Inventory[3].Contents = append(player.Inventory[3].Contents, &game.Item{
		ID:    2120,
		Count: 1,
	})

	err = d.SavePlayerItems(context.Background(), player)
	if err != nil {
		fmt.Println("SAVE ERROR:", err)
	} else {
		fmt.Println("SAVE SUCCESS")
	}
}
