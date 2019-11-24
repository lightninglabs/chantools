package main

import (
	"fmt"
	"os"
	
	"github.com/guggero/chantools"
)

func main() {
	if err := chantools.Main(); err != nil {
		fmt.Printf("Error running chantools: %v\n", err)
	}

	os.Exit(0)
}
