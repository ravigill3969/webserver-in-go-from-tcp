package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
)

type Res struct {
	message string
	addr    string
	file    string
}

var (
	maxActiveIps int
)

var (
	mu        sync.Mutex
	activeIPs = make(map[string]struct{})
)

func main() {
	maxActiveIps = 3
	l, err := net.Listen("tcp", ":8081")
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

		mu.Lock()
		if len(activeIPs) >= maxActiveIps {
			mu.Unlock()

			busyMsg := "HTTP/1.1 503 Service Unavailable\r\n" +
				"Content-Type: text/plain\r\n" +
				"Content-Length: 11\r\n\r\n" +
				"Server busy"

			c.Write([]byte(busyMsg))
			fmt.Println("Rejected connection from", c.RemoteAddr(), "server busy")
			c.Close()
			continue
		}

		activeIPs[c.RemoteAddr().String()] = struct{}{}
		mu.Unlock()

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

			mu.Lock()
			delete(activeIPs, conn.RemoteAddr().String())
			mu.Unlock()
			return
		}
		message := string(buffer[:n])
		addr := conn.RemoteAddr().String()

		requestData, err := urlMethodHandler(message)
		daat, err := headersInRequest(message)

		if err != nil {
			fmt.Println(err)
			return
		}

		fmt.Println(requestData, daat)

		resCh <- &Res{
			message: message,
			addr:    addr,
		}

	}
}

func sendResponse(conn net.Conn, resCh <-chan *Res) {
	defer func() {
		conn.Close()
		mu.Lock()
		delete(activeIPs, conn.RemoteAddr().String())
		fmt.Println("Connection removed:", conn.RemoteAddr().String())
		fmt.Println("Active connections:", len(activeIPs))
		mu.Unlock()
	}()

	for res := range resCh {
		if res.file != "" {
			body := res.file
			response := fmt.Sprintf("HTTP/1.1 200 OK\r\n"+
				"Content-Type: text/html\r\n"+
				"Content-Length: %d\r\n"+
				"\r\n%s", len(body), body)

			conn.Write([]byte(response))
			return 
		} else {
			msg := fmt.Sprintf("HTTP/1.1 200 OK\r\n"+
				"Content-Type: text/plain\r\n"+
				"Content-Length: %d\r\n"+
				"\r\nThanks: %s", len(res.message)+8, res.message)

			conn.Write([]byte(msg))
			return
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

type urlMethodHandlerT struct {
	Method string
	Path   string
}

func urlMethodHandler(message string) (urlMethodHandlerT, error) {
	reqLines := strings.Split(message, "\r\n")
	if len(reqLines) == 0 || strings.TrimSpace(reqLines[0]) == "" {
		return urlMethodHandlerT{}, fmt.Errorf("empty or malformed request")
	}

	requestLine := reqLines[0]
	parts := strings.Fields(requestLine)
	if len(parts) < 2 {
		return urlMethodHandlerT{}, fmt.Errorf("incomplete request line: %q", requestLine)
	}

	return urlMethodHandlerT{
		Method: parts[0],
		Path:   parts[1],
	}, nil
}

func headersInRequest(message string) (map[string]string, error) {
	headers := make(map[string]string)

	if message == "" {
		return nil, fmt.Errorf("empty request message")
	}

	reqLines := strings.Split(message, "\r\n")
	if len(reqLines) == 0 {
		return nil, fmt.Errorf("request message contains no lines")
	}

	for _, line := range reqLines[1:] {
		if line == "" {
			break
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("malformed header line: %q", line)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "" {
			return nil, fmt.Errorf("header key empty in line: %q", line)
		}

		headers[key] = value
	}

	return headers, nil
}
