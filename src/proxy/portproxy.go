package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

func doForward(conn net.Conn) {
	client, err := net.Dial("tcp", os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Dial failed: %v", err)
	}
	go func() {
		defer client.Close()
		defer conn.Close()
		io.Copy(client, conn)
	}()
	go func() {
		defer client.Close()
		defer conn.Close()
		io.Copy(conn, client)
	}()
}

func check_backend() {
	for {
		client, err := net.Dial("tcp", os.Args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Dial failed: %v", err)
			time.Sleep(100 * time.Millisecond)
		} else {
			client.Close()
			break
		}
	}
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage %s listen:port forward:port\n", os.Args[0])
		return
	}

	check_backend()
	listener, err := net.Listen("tcp", os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup listener: %v", err)
		return
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Fprintf(os.Stdout, "ERROR: failed to accept listener: %v", err)
		}
		go doForward(conn)
	}
}
