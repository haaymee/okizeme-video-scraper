package main

import (
	"fmt"
	"okiscraper/internal/app"
	"okiscraper/internal/cli"
	"os"
	"time"
)

func main() {
	character, actions, err := cli.ConfigureApp()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	start := time.Now()
	if err := app.New(character, actions).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Total Time: %s", time.Since(start))

	fmt.Println("Press any key to exit...")
	fmt.Scanln()
}
