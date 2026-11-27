package main

import (

	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: my-tool [daemon | create | start]")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "daemon":
		runDaemon()
	
	case "create", "start":
		runClientCommand(os.Args[1:])

	default:
		fmt.Println("Unknown command")
	}
}
