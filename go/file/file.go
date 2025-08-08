package fileOperations

import (
	"fmt"
	"net"
	"os"

	"github.com/ravigill3969/tcp-websocket/go/response"
)

func ServeFile(conn net.Conn, filename, contentType string) {
	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Println("Error reading file:", err)
		response.SendResponse(conn, "HTTP/1.1 404 Not Found\r\n\r\n404 File Not Found")
		return
	}

	responseData := fmt.Sprintf("HTTP/1.1 200 OK\r\n"+
		"Content-Type: %s\r\n"+
		"Content-Length: %d\r\n\r\n%s",
		contentType, len(content), string(content))

	response.SendResponse(conn, responseData)
}
