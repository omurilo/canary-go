package main

import (
	"fmt"
	"log"

	"github.com/opentibiabr/canary-go/internal/items"
)

func main() {
	cat, err := items.Load("../data/items/appearances.dat")
	if err != nil {
		log.Fatal("appearances.dat:", err)
	}

	err = cat.LoadXML("../data/items/items.xml")
	if err != nil {
		log.Fatal("items.xml:", err)
	}

	// 1386 is ladder server ID? Or what is backpack? Backpack is 2854.
	bp := cat.Get(2854)
	if bp != nil {
		fmt.Printf("Backpack: %+v\n", bp)
	} else {
		fmt.Println("Backpack (2854) not found in catalog!")
	}

	// 2120 is rope server ID
	rope := cat.Get(2120)
	if rope != nil {
		fmt.Printf("Rope: %+v\n", rope)
	} else {
		fmt.Println("Rope (2120) not found in catalog!")
	}
	
	// Ladder (Client ID 411/412? Server ID 1386?)
	ladder := cat.Get(1386)
	if ladder != nil {
		fmt.Printf("1386: %+v\n", ladder)
	} else {
		fmt.Println("1386 not found in catalog!")
	}
}
