package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"
)

func main() {
	l, err := net.Listen("tcp", ":8081")
	if err != nil {
		fmt.Println("Error in starting server:", err)
		return
	}
	fmt.Println("Server started on port 8081")
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting:", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		if err != io.EOF {
			fmt.Println("Read error:", err)
		}
		return
	}

	request := string(buffer[:n])
	fmt.Println("Request from", conn.RemoteAddr())

	lines := strings.Split(request, "\n")
	if len(lines) == 0 {
		sendResponse(conn, "HTTP/1.1 400 Bad Request\r\n\r\n")
		return
	}

	parts := strings.Fields(lines[0])
	if len(parts) < 2 {
		sendResponse(conn, "HTTP/1.1 400 Bad Request\r\n\r\n")
		return
	}

	path := parts[1]

	// Serve static files
	switch {
	case path == "/":
		serveFile(conn, "index.html", "text/html")
	case path == "/about":
		serveFile(conn, "./about/about.html", "text/html")
	case path == "/about/about.css":
		serveFile(conn, "./about/about.css", "text/css")
	case path == "/about/about.js":
		serveFile(conn, "./about/about.js", "text/javascript")
	case strings.HasSuffix(path, ".css"):
		serveFile(conn, "."+path, "text/css")
	case strings.HasSuffix(path, ".js"):
		serveFile(conn, "."+path, "text/javascript")
	default:
		sendResponse(conn, "HTTP/1.1 404 Not Found\r\n\r\n404 Not Found")
	}
}

func serveFile(conn net.Conn, filename, contentType string) {
	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Println("Error reading file:", err)
		sendResponse(conn, "HTTP/1.1 404 Not Found\r\n\r\n404 File Not Found")
		return
	}

	response := fmt.Sprintf("HTTP/1.1 200 OK\r\n"+
		"Content-Type: %s\r\n"+
		"Content-Length: %d\r\n\r\n%s",
		contentType, len(content), string(content))

	sendResponse(conn, response)
}

func sendResponse(conn net.Conn, response string) {
	conn.Write([]byte(response))
}
