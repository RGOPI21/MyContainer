package main

import (
	"fmt"
	"net"
	"log"
	"os"
	"os/exec"
	"encoding/json"
	"encoding/hex"
	"crypto/rand"
	"path/filepath"
	"io"
	"os/signal"
	"syscall"
)

const socketPath = "/tmp/mycontainer.sock"

type ContainerState struct {
	ID		string 
	Status	string
}

type ContainerConfig struct {
	Args	[]string `json:"args"`
	Root	string `json:"rootfs"`
}
var containerRegistry map[string]ContainerState

var containerIsActive bool

func generateContainerID() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func runDaemon() {
	
	// Catch SIGINT/SIGTERM for clean exit.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		for{	
			<-sigChan
			if containerIsActive {
				continue
			}

			fmt.Println("\n[Daemon] Shutdown signal received, Cleaning up the socket...")
			os.Remove(socketPath)
			os.Exit(0)
		}
	}()

	// Initialize global metadata storage map.
	containerRegistry = make(map[string]ContainerState)

	// Remove old socket file if it exists on the disk.
	if err:= os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		log.Fatalf("Failed to remove old socket: %v", err)	
	}

	// Bind and listen to the unix socket domain path
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("Critical Error: Unable to listen to listen on Unix socket:%v\n", err)
	}
	
	defer listener.Close()
	
	fmt.Println("Listening on socket /tmp/mycontainer.sock...")

	// Establishing infinite listener loop
	for {
		c, err := listener.Accept()
		if err != nil {
			fmt.Println("Accept error:",err)
			continue
		}

		unixConn, ok := c.(*net.UnixConn) 
		if !ok { 
			fmt.Println("Error: Not a unix connection") 
			c.Close() 
			continue 
		} 

		go handleDaemonConnection(unixConn) } }

func handleDaemonConnection(c *net.UnixConn) {
	
	defer c.Close()
	fmt.Println("New connection accepted.")

	// allocate data reading buffer chunk
	buff := make([]byte, 1024)
	oob	:= make([]byte, 1024)

	n, _, _, _, err := c.ReadMsgUnix(buff, oob) 
	if err != nil { 
		fmt.Println("Read error:", err) 
		return 
	} 

	// Converting the absolute byte array into readable text string var req ContainerRequest
	var req ContainerRequest

	err = json.Unmarshal(buff[:n], &req)
	if err != nil {
		fmt.Println("JSON unmarshal error:", err)
		return
	}

	switch req.Action {
	case "create":
		// generating a brand new unique container ID here!
		id, err := generateContainerID()
		if err != nil {
			fmt.Println("Error generating Container ID:", err)
			return
		}

		// Adding metadata record to containerRegistry map with "created" status
		state := ContainerState {ID: id, Status: "created"}
		containerRegistry[id] = state
		fmt.Println("Container successfully allocated:", id)

		// writing a success message back to socket stream to client.
		/*Packing the ID into JSON response payload */
		resp := ContainerRequest{Action: "create_response", ID: id}
		respData, err := json.Marshal(resp)
		if err != nil {
			fmt.Println("Failed to Marshal response:", err)
			return
		}
		/*writing the JSON back into the active socket connection */
		_, err = c.Write(respData)
		if err != nil {
			fmt.Println("Failed to write response back to client:", err)
			return 
		}

	case "start":
		//verifies req.ID exists inside containerRegistry map 
		state, exists := containerRegistry[req.ID]
		if !exists {
			fmt.Println("Error, Container ID not found in registry")

			// prepare an error JSON response payload for client.
			resp := ContainerRequest{Action: "start_response", ID: "Error: Invalid ID"}
			respData, _ := json.Marshal(resp)
			c.Write(respData)
			return
		}
		
		state.Status = "starting"
		containerRegistry[req.ID] = state

		// Bundle
		basePath := "/tmp/my-containers/"
		bundlePath := filepath.Join(basePath, req.ID)
		rootfsPath := filepath.Join(bundlePath, "rootfs")

		/*Creating bundle and empty rootfs using a single nested command
		os.MkdirAll() creates all the target folder and all missing parent folders at once.*/
		err = os.MkdirAll(rootfsPath, 0755)
		if err != nil {
			fmt.Println("Failed to create nested directories",err)
			return
		}

		// Instantiating containerConfig
		config := ContainerConfig {
			Args: []string{"/bin/bash"},
			Root: rootfsPath,
		}

		// convert JSON byte array and save it in json (marshal)
		configData, err := json.MarshalIndent(config, "", " ")
		if err != nil {
			fmt.Println("Failed to marshal config",err)
			return
		}

		configFilePath := filepath.Join(bundlePath, "config.json")
		err = os.WriteFile(configFilePath, configData, 0644)
		if err != nil {
			fmt.Println("Failed to write config.json", err)
		}

		// Copy a basic shell binary from host system into rootfs
		/* creating /bin directory inside root first*/
		rootfsBinDir := filepath.Join(rootfsPath, "bin")
		if err := os.MkdirAll(rootfsBinDir, 0755); err != nil {
			fmt.Println("Failed to create rootfs/bin directory", err)
			return
		}

		// open source binary on host
		srcFile, err := os.Open("/bin/bash")
		if err != nil {
			// /bin/sh if bash is missing
			srcFile, err = os.Open("/bin/sh")
			if err != nil {
				fmt.Println("Failed to open host shell binary", err)	
				return 
			}
		}
		defer srcFile.Close()

		// create target binary location inside rootfs/bin/bash
		targetBinPath := filepath.Join(rootfsBinDir, "bash")
		destFile, err := os.OpenFile(targetBinPath, os.O_CREATE | os.O_WRONLY, 0755)
		if err != nil {
			fmt.Println("Failed to create target binary file", err)
			return 
		}


		// copying the binary content
		if _, err  = io.Copy(destFile, srcFile); err != nil {
			fmt.Println("Failed to copy binary data", err)
			destFile.Close()
			return
		}

		destFile.Close()

		// copying /bi/ls utility into container environment
		srcLs, err := os.Open("/bin/ls")
		if err != nil {
			fmt.Println("Failed to Open host host ls utility:", err)
			return 
		}

		defer srcLs.Close()

		targetLsPath := filepath.Join(rootfsBinDir, "ls")
		destLs, err := os.OpenFile(targetLsPath, os.O_CREATE | os.O_WRONLY, 0755)
		if err != nil {
			fmt.Println("Failed to create target ls binary file", err)
			return
		}

		if _, err = io.Copy(destLs, srcLs); err != nil {
			fmt.Println("Failed to copy ls binary data", err)
			destLs.Close()
			return 
		}
		destLs.Close()


		// Create the /lib64 folder inside rootfs 
		rootfsLibDir := filepath.Join(rootfsPath, "lib64")
		if err := os.MkdirAll(rootfsLibDir, 0755); err != nil {
			fmt.Println("Failed to create rootfs/lib64 directory", err)
			return 
		}

		// ldd /bin/bash listed host files for bash to run.
		libsToCopy := []string {
			"/lib64/ld-linux-x86-64.so.2",
			"/lib64/libc.so.6",
			"/lib64/libtinfo.so.6",
			"/lib64/libselinux.so.1",
			"/lib64/libcap.so.2",
			"/lib64/libpcre2-8.so.0",
		}

		for _, libPath := range libsToCopy {
			srcLib, err := os.Open(libPath)
			if err != nil {
				fmt.Printf("Failed to open host library %s:%v\n", libPath, err)
				return 
			}

			// Determine target Path like, /tmp/my-containers/<id>/rootfs/lib64/libc.so.6
			targetLibPath := filepath.Join(rootfsLibDir, filepath.Base(libPath))
			destLib, err := os.OpenFile(targetLibPath, os.O_CREATE | os.O_WRONLY, 0755)
			if err != nil {
				fmt.Printf("Failed to create target library path %s: %v\n", targetLibPath, err)
				srcLib.Close()
				return
			}

			if _, err = io.Copy(destLib, srcLib); err != nil {
				fmt.Printf("Failed to copy library data for %s: %v\n", libPath,err)
				srcLib.Close()
				destLib.Close()
				return
			}

			srcLib.Close()
			destLib.Close()
		}

		fmt.Printf("Bundle environment successfully created at: %s\n", bundlePath)

		// success response to client so it exits without any error.
		resp := ContainerRequest {Action: "start_response", ID: req.ID}
		respData, _ := json.Marshal(resp)
		_, _ = c.Write(respData)

		// if it does, change it's status and prepare to execute the runtime c.
		cmd := exec.Command("./my-container-runtime", bundlePath)
		
		// I/O streams
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		containerIsActive = true

		// Launching process asynchronously
		if err := cmd.Start(); err != nil {
			fmt.Println("Failed to launch the shim process:", err)
			return
		}

		fmt.Println("Container shell active. Waiting for exit...")

		if err := cmd.Wait(); err != nil {
			fmt.Printf("Container exited with error state: %v\n", err)
		} else {
			fmt.Println("Container Process terminated cleanly. Returning to daemon loop.")
		}

		containerIsActive = false

	default:
		fmt.Printf("Unknown action received:%s\n", req.Action)
	}
}


