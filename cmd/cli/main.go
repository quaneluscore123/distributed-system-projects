package main

import (
	"fmt"
	"os"
)

func printUsage() {
	fmt.Println("RoseDB CLI")
	fmt.Println("Usage:")
	fmt.Println("  get <key>")
	fmt.Println("  put <key> <value>")
	fmt.Println("  delete <key>")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	switch os.Args[1] {

	case "get":
		if len(os.Args) < 3 {
			fmt.Println("Usage: get <key>")
			return
		}
		fmt.Printf("GET %s\n", os.Args[2])

	case "put":
		if len(os.Args) < 4 {
			fmt.Println("Usage: put <key> <value>")
			return
		}
		fmt.Printf("PUT %s = %s\n", os.Args[2], os.Args[3])

	case "delete":
		if len(os.Args) < 3 {
			fmt.Println("Usage: delete <key>")
			return
		}
		fmt.Printf("DELETE %s\n", os.Args[2])

	default:
		fmt.Println("Unknown command")
		printUsage()
	}
}