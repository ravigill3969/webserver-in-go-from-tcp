package main

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
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
			fmt.Println("Read error:", err)
		}
		return
	}

	request := string(buffer[:n])

	headers := HeaderToMap(request)

	lines := strings.Split(request, "\r\n")

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

	url := strings.Split(path, "/")

	if url[1] == "ws" && strings.Contains(request, "Upgrade: websocket") {
		handleWebSocketHandshake(conn, headers)
		handleWebSocketEcho(conn)
	}

	switch {
	case path == "/":
		serveFile(conn, "index.html", "text/html")
	case path == "/about":
		serveFile(conn, "./about/about.html", "text/html")
	case path == "/about/about.css":
		serveFile(conn, "./about/about.css", "text/css")
	case path == "/about/about.js":
		serveFile(conn, "./about/about.js", "text/javascript")
	case path == "/chat/chat.html":
		serveFile(conn, "./chat/chat.html", "text/html")
	case strings.HasSuffix(path, ".css"):
		serveFile(conn, "."+path, "text/css")
	case strings.HasSuffix(path, ".js"):
		serveFile(conn, "."+path, "text/javascript")
	default:
		sendResponse(conn, "HTTP/1.1 404 Not Found\r\n\r\n404 Not Found")
	}
}

func handleWebSocketHandshake(conn net.Conn, headers map[string]string) error {
	key, ok := headers["Sec-WebSocket-Key"]
	if !ok {
		return fmt.Errorf("missing Sec-WebSocket-Key")
	}

	const magicGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	sha1sum := sha1.Sum([]byte(key + magicGUID))
	acceptKey := base64.StdEncoding.EncodeToString(sha1sum[:])

	response := fmt.Sprintf("HTTP/1.1 101 Switching Protocols\r\n"+
		"Upgrade: websocket\r\n"+
		"Connection: Upgrade\r\n"+
		"Sec-WebSocket-Accept: %s\r\n\r\n", acceptKey)

	_, err := conn.Write([]byte(response))
	return err
}

func handleWebSocketEcho(conn net.Conn) {
	for {
		msg, err := readWebSocketFrame(conn)
		fmt.Println(msg, err)
		if err != nil {
			fmt.Println("Read error:", err)
			return
		}
		fmt.Println("Received:", msg)
		writeWebSocketText(conn, "Echo: "+msg)
	}
}
func readWebSocketFrame(conn net.Conn) (string, error) {
	header := make([]byte, 2)
	fmt.Println("inside read websocket frame")
	fmt.Println("Waiting to read 2 bytes header...")
	n, err := io.ReadFull(conn, header)
	
	fmt.Printf("Read %d bytes, err: %v\n", n, err)
	if err != nil {
		return "", err
	}

	fin := header[0]&0x80 != 0
	opcode := header[0] & 0x0F
	mask := header[1]&0x80 != 0
	payloadLen := int(header[1] & 0x7F)

	// **Extended payload length handling starts here**
	switch payloadLen {
	case 126:
		extended := make([]byte, 2)
		if _, err := io.ReadFull(conn, extended); err != nil {
			fmt.Println("Error reading extended payload length:", err)
			return "", err
		}
		payloadLen = int(binary.BigEndian.Uint16(extended))
	case 127:
		extended := make([]byte, 8)
		if _, err := io.ReadFull(conn, extended); err != nil {
			fmt.Println("Error reading extended payload length:", err)
			return "", err
		}
		payloadLen64 := binary.BigEndian.Uint64(extended)
		if payloadLen64 > (1 << 31) {
			return "", fmt.Errorf("payload too large")
		}
		payloadLen = int(payloadLen64)
	}

	fmt.Printf("FIN: %v\n", fin)
	fmt.Printf("Opcode: %d\n", opcode)
	fmt.Printf("MASK: %v\n", mask)
	fmt.Printf("Payload Length: %d\n", payloadLen)

	if opcode == 0x8 {
		fmt.Println("Received close frame")
		return "", io.EOF
	}

	if !fin || opcode != 0x1 || !mask {
		return "", fmt.Errorf("unsupported frame (FIN=%v, opcode=0x%x, mask=%v)", fin, opcode, mask)
	}

	maskKey := make([]byte, 4)
	if _, err := io.ReadFull(conn, maskKey); err != nil {
		return "", err
	}

	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return "", err
	}

	for i := 0; i < payloadLen; i++ {
		payload[i] ^= maskKey[i%4]
	}

	fmt.Printf("Payload data: %s\n", string(payload))

	return string(payload), nil
}

func writeWebSocketText(conn net.Conn, message string) error {
	payload := []byte(message)
	frame := []byte{
		0x81,               // FIN=1, opcode=1 (text)
		byte(len(payload)), // assuming len < 126
	}
	frame = append(frame, payload...)
	_, err := conn.Write(frame)
	return err
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

func HeaderToMap(request string) map[string]string {
	headersMap := map[string]string{}
	splitRequestInfoRN := strings.Split(request, "\r\n")

	for i := 1; i < len(splitRequestInfoRN); i++ {
		currentLine := splitRequestInfoRN[i]
		if currentLine == "" {
			break
		}
		for j := 0; j < len(currentLine); j++ {
			if currentLine[j] == ':' {
				key := strings.TrimSpace(currentLine[:j])
				value := strings.TrimSpace(currentLine[j+1:])

				headersMap[key] = value
				break
			}
		}

	}

	return headersMap

}
