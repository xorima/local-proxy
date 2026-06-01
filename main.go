package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		fmt.Println("usage: local-proxy <command> [flags]")
		fmt.Println("commands: serve, hashgen, magic")
		os.Exit(1)
	}

	switch args[0] {
	case "serve":
		os.Args = append([]string{"cmd/serve/main.go"}, args[1:]...)
		// In production, cmd/serve is built as a separate binary
		fmt.Println("run: go run ./cmd/serve or build the cmd/serve binary")
		os.Exit(0)
	case "hashgen":
		os.Args = append([]string{"cmd/hashgen/main.go"}, args[1:]...)
		fmt.Println("run: go run ./cmd/hashgen or build the cmd/hashgen binary")
		os.Exit(0)
	case "magic":
		os.Args = append([]string{"cmd/magic/main.go"}, args[1:]...)
		fmt.Println("run: go run ./cmd/magic or build the cmd/magic binary")
		os.Exit(0)
	default:
		fmt.Printf("unknown command: %s\n", args[0])
		os.Exit(1)
	}
}
