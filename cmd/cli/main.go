package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

func printUsage() {
	fmt.Println("RoseDB CLI")
	fmt.Println("Usage:")
	fmt.Println("  get <key>")
	fmt.Println("  put <key> <value>")
	fmt.Println("  delete <key>")
	fmt.Println("  health")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  go run cmd/cli/main.go get name")
	fmt.Println("  go run cmd/cli/main.go put name Duy")
	fmt.Println("  go run cmd/cli/main.go delete name")
	fmt.Println("  go run cmd/cli/main.go health")
	fmt.Println("  go run cmd/cli/main.go -server http://localhost:8081 health")
}

func main() {
	serverAddr := flag.String(
		"server",
		"http://localhost:8080",
		"RoseDB server address",
	)

	flag.Parse()

	args := flag.Args()

	if len(args) < 1 {
		printUsage()
		return
	}

	switch args[0] {

	case "get":
		if len(args) < 2 {
			fmt.Println("Usage: get <key>")
			return
		}

		resp, err := http.Get(
			*serverAddr + "/get?key=" + args[1],
		)

		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		fmt.Println(string(body))

	case "put":
		if len(args) < 3 {
			fmt.Println("Usage: put <key> <value>")
			return
		}

		resp, err := http.PostForm(
			*serverAddr+"/put",
			url.Values{
				"key":   {args[1]},
				"value": {args[2]},
			},
		)

		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		fmt.Println(string(body))

	case "delete":
		if len(args) < 2 {
			fmt.Println("Usage: delete <key>")
			return
		}

		req, err := http.NewRequest(
			http.MethodDelete,
			*serverAddr+"/delete?key="+args[1],
			nil,
		)

		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		client := &http.Client{}

		resp, err := client.Do(req)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		fmt.Println(string(body))

	case "health":

		resp, err := http.Get(
			*serverAddr + "/health",
		)

		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		fmt.Println(string(body))

	default:
		fmt.Println("Unknown command")
		printUsage()
	}
}
