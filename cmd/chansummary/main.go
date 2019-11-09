package main

import (
	"fmt"
	"os"
	
	"github.com/guggero/chansummary"
)

func main() {
	if err := chansummary.Main(); err != nil {
		fmt.Printf("Error running chansummary: %v\n", err)
	}

	os.Exit(0)
}
