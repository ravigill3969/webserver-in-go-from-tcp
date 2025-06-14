package main

import (
	"fmt"
	"io"
	"net"
)

func main() {
	l, err := net.Listen("tcp", ":8080")

	if err != nil {
		fmt.Println("error in statrting server", err)
		return
	}

	fmt.Println("server stated on port 8080")

	defer l.Close()

	for {
		c, err := l.Accept()

		if err != nil {
			fmt.Println("error accepting", err)
			continue
		}

		fmt.Println("got a user connected ")

		go acceptConnection(c)
	}

}

func acceptConnection(conn net.Conn) {
	defer conn.Close()
	buffer := make([]byte, 1024)

	for {
		n, err := conn.Read(buffer)
		if err != nil {
			if err == io.EOF {
				fmt.Println("Connection closed by client")
			} else {
				fmt.Println("Read error:", err)
			}
			return
		}

		addr := conn.RemoteAddr()

		msg := fmt.Sprintf("Thanks for your message  %s\n", addr)
		conn.Write([]byte(msg))
		fmt.Printf("Received: %s\n", string(buffer[:n]))
	}
}
