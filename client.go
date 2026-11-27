package main

import (
	"fmt"
	"net"
	"os"
	"encoding/json"
)

type ContainerRequest struct {
	Action string `json:"action"`	
	ID	   string `json:"id,omitempty"`
	Error  string `json:"error,omitempty"`
}

func sendToSocket(jsonData []byte) {
	
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		fmt.Printf("Error: Cannot connect to daemon at %s:%v\n", socketPath, err)
		fmt.Println("Ensure the daemon background process is running.")
		os.Exit(1)
	}

	defer conn.Close()
	
	// Client writing request to daemon
	_, err = conn.Write(jsonData)
	if err != nil {
		fmt.Printf("Error. Failed to write data to socket.\n")
		os.Exit(1)
	}

	// allocating a buffer chunk to capture the daemon's reply
	replyBuff := make([]byte, 1024)
	n, err := conn.Read(replyBuff)
	if err != nil {
		fmt.Println("Error: Failed to read response from daemon socket")
		os.Exit(1)
	}

	// Decoding the returned JSON payload to unpack the ID string
	var resp ContainerRequest 

	if err := json.Unmarshal(replyBuff[:n], &resp); err != nil {
		fmt.Println("Error: Failed to parse daemon response")
		os.Exit(1)
	}

	if resp.Action == "create_response" {
		fmt.Printf("Container created successfully ID:%s\n", resp.ID)
	}	else if resp.Action == "start_response" {
		fmt.Printf("Container started successfully ID %s\n", resp.ID)
	} else {
		fmt.Printf("Action completed successfully ID: %s\n", resp.ID)
	}
}

func runClientCommand (args []string) {
	if len(args) < 1 {
		fmt.Println("Error: Missing subcommand, Choose 'create' or 'start'.")
		os.Exit(1)
	}

	subcommand := args[0]

	switch subcommand {
	case "create":
		req := ContainerRequest{Action: subcommand}
		jsonData, _ := json.Marshal(req)
		sendToSocket(jsonData)

	case "start":
		if len(args) < 2 {
			fmt.Println("Error. 'start' requires a container ID. Usage: my-tool start <id>")
			os.Exit(1)
		}

		containerID := args[1]
		//fmt.Printf("Valid client request: action=%s;id=%s", subcommand, containerID)
		req := ContainerRequest{Action: subcommand, ID: containerID}
		jsonData, _ := json.Marshal(req)
		sendToSocket(jsonData)
			
	default:
		fmt.Printf("Error. Unknown subcommand '%s'. Use 'create' or 'start'.\n", subcommand)
		os.Exit(1)
	}
} 
