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
	"sync"
)

type Client struct {
	Conn net.Conn
	IP   net.IP
	Room string
}

var rooms = make(map[string][]*Client)
var mu sync.Mutex

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

	// userIp := conn.RemoteAddr().String()

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

	parserUserIPandStoreInMapWithGrpId(conn, url[0])

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
	var client *Client

	// Find the client object from connection
	mu.Lock()
	for _, clients := range rooms {
		for _, c := range clients {
			if c.Conn == conn {
				client = c
				break
			}
		}
		if client != nil {
			break
		}
	}
	mu.Unlock()

	if client == nil {
		fmt.Println("Client not found in room list")
		return
	}

	for {
		msg, err := readWebSocketFrame(conn)
		if err != nil {
			fmt.Println("WebSocket read error:", err)
			removeClient(client)
			return
		}

		fmt.Println("Broadcasting message:", msg)
		broadcastToRoom(client.Room, msg, client)
	}
}

func readWebSocketFrame(conn net.Conn) (string, error) {
	header := make([]byte, 2)

	n, err := io.ReadFull(conn, header)

	fmt.Printf("Read %d bytes, err: %v\n", n, err)
	if err != nil {
		return "", err
	}

	fin := header[0]&0x80 != 0
	// fin := header[0]&128 != 0

	opcode := header[0] & 0x0F
	// opcode := header[0] & 15 // 15 = 0x0F

	mask := header[1]&0x80 != 0

	//mask := header[0]&128 != 0
	payloadLen := int(header[1] & 0x7F)
	// payloadLen := int(header[1] & 127)

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
	payloadLen := len(payload)

	header := []byte{0x81} // FIN=1, text frame
	// header := []byte{129}

	// Set length — do NOT mask
	if payloadLen < 126 {
		header = append(header, byte(payloadLen))
	} else if payloadLen < 65536 {
		header = append(header, 126)
		header = append(header, byte(payloadLen>>8), byte(payloadLen&0xff))
	} else {
		header = append(header, 127)
		for i := 7; i >= 0; i-- {
			header = append(header, byte(payloadLen>>(8*i)))
		}
	}

	// No mask, just append payload
	frame := append(header, payload...)
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

func parserUserIPandStoreInMapWithGrpId(conn net.Conn, roomKey string) {

	client := &Client{
		Conn: conn,
		IP:   conn.RemoteAddr().(*net.TCPAddr).IP,
		Room: roomKey,
	}

	mu.Lock()
	rooms[roomKey] = append(rooms[roomKey], client)
	mu.Unlock()

}

func broadcastToRoom(room string, message string, sender *Client) {
	mu.Lock()
	defer mu.Unlock()

	for _, client := range rooms[room] {
		if client.Conn != sender.Conn {
			err := writeWebSocketText(client.Conn, message)
			if err != nil {
				fmt.Println("Error sending message:", err)
			}
		}
	}
}

func removeClient(client *Client) {
	mu.Lock()
	defer mu.Unlock()

	clients := rooms[client.Room]
	for i, c := range clients {
		if c.Conn == client.Conn {
			rooms[client.Room] = append(clients[:i], clients[i+1:]...)
			break
		}
	}

	client.Conn.Close()
}
