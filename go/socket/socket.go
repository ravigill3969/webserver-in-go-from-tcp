package socket

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
)

type Client struct {
	Conn     net.Conn
	IP       net.IP
	Room     string
	SendChan chan string
}

var rooms = make(map[string][]*Client)
var mu sync.Mutex

//-------------------------------------------------------------------------//

func HandleWebSocketHandshake(conn net.Conn, headers map[string]string) error {
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

//-------------------------------------------------------------------------//

func HandleWebSocketEcho(conn net.Conn) {
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

	// Reader loop
	for {
		msg, err := ReadWebSocketFrame(conn)
		if err != nil {
			fmt.Println("WebSocket read error:", err)
			return
		}
		fmt.Println("Broadcasting message:", msg)
		broadcastToRoom(client.Room, msg, client)
	}
}

//-------------------------------------------------------------------------//

func ReadWebSocketFrame(conn net.Conn) (string, error) {
	header := make([]byte, 2)

	_, err := io.ReadFull(conn, header)
	if err != nil {
		return "", err
	}

	fin := header[0]&0x80 != 0
	opcode := header[0] & 0x0F
	mask := header[1]&0x80 != 0
	payloadLen := int(header[1] & 0x7F)

	switch payloadLen {
	case 126:
		extended := make([]byte, 2)
		if _, err := io.ReadFull(conn, extended); err != nil {
			return "", err
		}
		payloadLen = int(binary.BigEndian.Uint16(extended))
	case 127:
		extended := make([]byte, 8)
		if _, err := io.ReadFull(conn, extended); err != nil {
			return "", err
		}
		payloadLen64 := binary.BigEndian.Uint64(extended)
		if payloadLen64 > (1 << 31) {
			return "", fmt.Errorf("payload too large")
		}
		payloadLen = int(payloadLen64)
	}

	if opcode == 0x8 {
		return "", io.EOF
	}

	if !fin || opcode != 0x1 || !mask {
		return "", fmt.Errorf("unsupported frame")
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

	return string(payload), nil
}

//-------------------------------------------------------------------------//

func writeWebSocketText(conn net.Conn, message string) error {
	payload := []byte(message)
	payloadLen := len(payload)

	header := []byte{0x81}

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

	frame := append(header, payload...)
	_, err := conn.Write(frame)
	return err
}

//-------------------------------------------------------------------------//

func broadcastToRoom(room string, message string, sender *Client) {
	mu.Lock()
	defer mu.Unlock()

	for _, client := range rooms[room] {
		if client.Conn != sender.Conn {
			select {
			case client.SendChan <- message:

			default:
				fmt.Println("SendChan full for", client.IP)
			}
		}
	}
}

//-------------------------------------------------------------------------//

func ParserUserIPandStoreInMapWithGrpId(conn net.Conn, roomKey string) {
	client := &Client{
		Conn:     conn,
		IP:       conn.RemoteAddr().(*net.TCPAddr).IP,
		Room:     roomKey,
		SendChan: make(chan string, 256),
	}

	// Writer goroutine for this client
	go func(c *Client) {
		for msg := range c.SendChan {
			if err := writeWebSocketText(c.Conn, msg); err != nil {
				removeClient(c)
				return
			}
		}
	}(client)

	mu.Lock()
	rooms[roomKey] = append(rooms[roomKey], client)
	mu.Unlock()
}

func removeClient(c *Client) {
	mu.Lock()
	defer mu.Unlock()

	clients := rooms[c.Room]
	for i := 0; i < len(clients); i++ {
		if clients[i] == c {
			rooms[c.Room] = append(clients[:i], clients[i+1:]...)
			fmt.Println(c.Room)
			break
		}
	}

	close(c.SendChan)
	c.Conn.Close()
}
