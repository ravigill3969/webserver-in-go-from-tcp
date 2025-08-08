package response

import "net"

func SendResponse(conn net.Conn, response string) {
	conn.Write([]byte(response))
}
