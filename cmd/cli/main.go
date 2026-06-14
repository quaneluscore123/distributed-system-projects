package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
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

		resp, err := http.Get(
			"http://localhost:8080/get?key=" + os.Args[2],
		)

		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		fmt.Println(string(body))

	case "put":
		if len(os.Args) < 4 {
			fmt.Println("Usage: put <key> <value>")
			return
		}

		resp, err := http.PostForm(
			"http://localhost:8080/put",
			url.Values{
				"key":   {os.Args[2]},
				"value": {os.Args[3]},
			},
		)

		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		fmt.Println(string(body))

	case "delete":
		if len(os.Args) < 3 {
			fmt.Println("Usage: delete <key>")
			return
		}

		req, _ := http.NewRequest(
			http.MethodDelete,
			"http://localhost:8080/delete?key="+os.Args[2],
			nil,
		)

		client := &http.Client{}

		resp, err := client.Do(req)

		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)

		fmt.Println(string(body))

	default:
		fmt.Println("Unknown command")
		printUsage()
	}
}
