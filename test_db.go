package main

import (
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type ClientTraffic struct {
	Id        int    `gorm:"primaryKey"`
	InboundId int
	Enable    bool
	Email     string
	Up        int64
	Down      int64
	AllTime   int64
}

func main() {
	db, err := gorm.Open(sqlite.Open("db/x-ui.db"), &gorm.Config{})
	if err != nil {
		fmt.Println("failed to connect database:", err)
		return
	}

	var traffics []ClientTraffic
	db.Find(&traffics)
	
	fmt.Printf("%-5s %-10s %-8s %-20s %-10s %-10s %-10s\n", "ID", "InboundID", "Enable", "Email", "Up", "Down", "AllTime")
	for _, t := range traffics {
		fmt.Printf("%-5d %-10d %-8v %-20s %-10d %-10d %-10d\n", t.Id, t.InboundId, t.Enable, t.Email, t.Up, t.Down, t.AllTime)
	}
}
