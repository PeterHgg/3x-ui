package main

import (
	"fmt"
	"log"
	"os"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/xray"
)

func main() {
	dbPath := "bin/3x-ui.db"
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		dbPath = "3x-ui.db"
	}

	err := database.InitDB(dbPath)
	if err != nil {
		log.Fatalf("InitDB failed: %v", err)
	}

	db := database.GetDB()

	fmt.Println("--- ClientTraffic Schema Check ---")
	var rows []struct {
		Name string
		SQL  string
	}
	db.Raw("SELECT name, sql FROM sqlite_master WHERE type='index' AND tbl_name='client_traffics'").Scan(&rows)
	for _, r := range rows {
		fmt.Printf("Index: %s, SQL: %s\n", r.Name, r.SQL)
	}

	fmt.Println("\n--- Recent Client Traffic Data ---")
	var traffics []xray.ClientTraffic
	db.Order("id desc").Limit(10).Find(&traffics)
	for _, t := range traffics {
		fmt.Printf("ID: %d, InboundID: %d, Email: %s, Up: %d, Down: %d\n", t.Id, t.InboundId, t.Email, t.Up, t.Down)
	}
}
