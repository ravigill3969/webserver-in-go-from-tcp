package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"sync"
)

type Res struct {
	message string
	addr    string
	file    string
}

var (
	mu        sync.Mutex
	activeIPs = make(map[string]struct{})
)

func storeIPs(ip string) {
	mu.Lock()
	defer mu.Lock()
	activeIPs[ip] = struct{}{}
}

func deleteIP(ip string) {
	mu.Lock()
	defer mu.Lock()
	delete(activeIPs, ip)
}

func main() {
	l, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Println("Error in starting server:", err)
		return
	}
	fmt.Println("Server started on port 8080")
	defer l.Close()

	for {
		c, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting:", err)
			continue
		}
		storeIPs(c.RemoteAddr().String())
		fmt.Println("Got a user connected")

		resCh := make(chan *Res)

		go acceptConnection(c, resCh)
		go sendResponse(c, resCh)
		go readFile(resCh, "index.html")

	}
}

func acceptConnection(conn net.Conn, resCh chan<- *Res) {
	buffer := make([]byte, 1024)

	for {
		n, err := conn.Read(buffer)
		if err != nil {
			if err == io.EOF {
				fmt.Println("Connection closed by client")
			} else {
				fmt.Println("Read error:", err)
			}
			close(resCh)
			deleteIP(conn.RemoteAddr().String())
			return
		}
		message := string(buffer[:n])
		addr := conn.RemoteAddr().String()

		fmt.Printf("Received: %s\n", message)

		resCh <- &Res{
			message: message,
			addr:    addr,
		}

	}
}

func sendResponse(conn net.Conn, resCh <-chan *Res) {
	defer conn.Close()
	for res := range resCh {
		if res.message == "PING" {
			conn.Write([]byte("PONG"))
		} else if res.file != "" {
			body := res.file
			response := fmt.Sprintf("HTTP/1.1 200 OK\r\n"+
				"Content-Type: text/html\r\n"+
				"Content-Length: %d\r\n"+
				"\r\n%s", len(body), body)

			conn.Write([]byte(response))
			continue
		} else {

			msg := fmt.Sprintf("HTTP/1.1 200 OK\r\n"+
				"Content-Type: text/plain\r\n"+
				"Content-Length: %d\r\n"+
				"\r\nThanks: %s", len(res.message)+8, res.message)

			conn.Write([]byte(msg))
		}
	}
}

func readFile(resCh chan<- *Res, fileName string) {
	f, err := os.ReadFile(fileName)
	if err != nil {
		fmt.Println("Error reading file:", err)
		return
	}
	resCh <- &Res{
		file: string(f),
	}

}

// func readUrlPath(message string) {
// 	for {
// 		if
// 	}
// }
