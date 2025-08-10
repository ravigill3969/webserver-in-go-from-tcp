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
	Conn net.Conn
	IP   net.IP
	Room string
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

// -------------------------------------------------------------------------//

func HandleWebSocketEcho(conn net.Conn) {
	var client *Client

	// Find the client object from connection
	mu.Lock()
	for _, clients := range rooms {
		fmt.Println(len(rooms))
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

//-------------------------------------------------------------------------//

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

//-------------------------------------------------------------------------//

func broadcastToRoom(room string, message string, sender *Client) {
	mu.Lock()
	defer mu.Unlock()

	for _, client := range rooms[room] {
		if client.Conn != sender.Conn {
			err := writeWebSocketText(client.Conn, message)
			fmt.Println(client.Conn.RemoteAddr().String(), sender.Conn.RemoteAddr().String())
			if err != nil {
				fmt.Println("Error sending message:", err)

			}
		}
	}
}

//-------------------------------------------------------------------------//

func ParserUserIPandStoreInMapWithGrpId(conn net.Conn, roomKey string) {

	client := &Client{
		Conn: conn,
		IP:   conn.RemoteAddr().(*net.TCPAddr).IP,
		Room: roomKey,
	}

	mu.Lock()

	fmt.Println(roomKey)
	rooms[roomKey] = append(rooms[roomKey], client)

	for roomKey, clients := range rooms {
		fmt.Printf("Room: %s\n", roomKey)
		for i, client := range clients {
			fmt.Printf("  Client %d: IP=%s, Room=%s\n", i, client.IP.String(), client.Room)
		}
	}

	mu.Unlock()

}

//-------------------------------------------------------------------------//
