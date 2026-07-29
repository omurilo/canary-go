package main

import (
	"fmt"
	"os"
	"strings"

	"google.golang.org/protobuf/proto"

	"github.com/opentibiabr/canary-go/internal/appproto"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run cmd/lookup_outfit/main.go <path/to/appearances.dat> [search_term]")
		os.Exit(1)
	}

	path := os.Args[1]
	search := ""
	if len(os.Args) >= 3 {
		search = strings.ToLower(os.Args[2])
	}

	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading %s: %v\n", path, err)
		os.Exit(1)
	}

	var app appproto.Appearances
	if err := proto.Unmarshal(data, &app); err != nil {
		fmt.Fprintf(os.Stderr, "error unmarshalling: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Loaded %d objects, %d outfits, %d effects\n",
		len(app.GetObject()), len(app.GetOutfit()), len(app.GetEffect()))

	for _, outfit := range app.GetOutfit() {
		name := string(outfit.GetName())
		id := outfit.GetId()

		if search != "" {
			lower := strings.ToLower(name)
			searchLower := strings.ToLower(search)
			if strings.Contains(lower, searchLower) || lower == searchLower {
				fmt.Printf("  lookType=%-5d  name=%q\n", id, name)
			}
		} else {
			fmt.Printf("  lookType=%-5d  name=%q\n", id, name)
		}
	}
}
