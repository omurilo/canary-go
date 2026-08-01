package main

import (
	"fmt"
	"github.com/omurilo/canary-go/internal/items"
)

func main() {
	cat, err := items.Load("../data/items/appearances.dat")
	if err != nil {
		panic(err)
	}
	err = cat.LoadXML("../data/items/items.xml")
	if err != nil {
		panic(err)
	}

	h := cat.Get(3405)
	if h != nil {
		fmt.Printf("Helmet: %+v\n", h)
	}
}
