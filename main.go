package main

import (
	"fmt"
	"io"
	"net"
	"strings"

	fileOperations "github.com/ravigill3969/tcp-websocket/go/file"
	"github.com/ravigill3969/tcp-websocket/go/response"
	"github.com/ravigill3969/tcp-websocket/go/socket"
	"github.com/ravigill3969/tcp-websocket/go/utils"
)

func main() {
	l, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Println("Error in starting server:", err)
		return
	}
	fmt.Println("Server started on port 8080")
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
			fmt.Println("Read error:wrk5m3n8busy", err)
		}
		return
	}

	request := string(buffer[:n])

	headers := utils.HeaderToMap(request)

	lines := strings.Split(request, "\r\n")

	if len(lines) == 0 {
		response.SendResponse(conn, "HTTP/1.1 400 Bad Request\r\n\r\n")
		return
	}

	parts := strings.Fields(lines[0])

	if len(parts) < 2 {
		response.SendResponse(conn, "HTTP/1.1 400 Bad Request\r\n\r\n")
		return
	}

	path := parts[1]

	url := strings.Split(path, "/")

	if url[1] == "ws" && strings.Contains(request, "Upgrade: websocket") {

		socket.ParserUserIPandStoreInMapWithGrpId(conn, url[2])
		socket.HandleWebSocketHandshake(conn, headers)

		socket.HandleWebSocketEcho(conn)
		return
	}

	switch {
	case path == "/":
		fileOperations.ServeFile(conn, "index.html", "text/html")
	case path == "/about":
		fileOperations.ServeFile(conn, "./about/about.html", "text/html")
	case path == "/about/about.css":
		fileOperations.ServeFile(conn, "./about/about.css", "text/css")
	case path == "/about/about.js":
		fileOperations.ServeFile(conn, "./about/about.js", "text/javascript")
	case path == "/chat/chat.html":
		fileOperations.ServeFile(conn, "./chat/chat.html", "text/html")
	case strings.HasSuffix(path, ".css"):
		fileOperations.ServeFile(conn, "."+path, "text/css")
	case strings.HasSuffix(path, ".js"):
		fileOperations.ServeFile(conn, "."+path, "text/javascript")
	default:
		response.SendResponse(conn, "HTTP/1.1 404 Not Found\r\n\r\n404 Not Found")
	}
}
