package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/facubozzi/fyne-writer/internal/app"
)

func main() {
	checkAI := flag.Bool("check-ai", false, "send a tiny prompt to the active provider and exit")
	flag.Parse()

	a, err := app.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		os.Exit(1)
	}

	if *checkAI {
		out, err := a.CheckAI(context.Background())
		if err != nil {
			fmt.Fprintln(os.Stderr, "check-ai:", err)
			os.Exit(1)
		}
		fmt.Println(out)
		return
	}

	a.Run()
}
