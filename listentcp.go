// 2014, 2015 Jamie Alquiza
package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
)

// We match SQS max message size since it's
// the reference message queue (for now).
var maxMsgSize = 256 * 1024

// Listens for messages, reqHandler goroutine dispatched for each.
func listenTcp() {
	log.Printf("Ascender TCP listener started: %s:%s\n",
		config.addr,
		config.port)
	server, err := net.Listen("tcp", config.addr+":"+config.port)
	if err != nil {
		log.Fatalf("Listener error: %s\n", err)
	}
	defer server.Close()
	// Connection handler loop.
	for {
		conn, err := server.Accept()
		if err != nil {
			log.Printf("Listener down: %s\n", err)
			continue
		}
		go reqHandler(conn)
	}
}

// Receives messages from 'listener' & sends over 'messageIncomingQueue'.
func reqHandler(conn net.Conn) {
	defer conn.Close()
	messages := bufio.NewScanner(conn)

	for messages.Scan() {
		m := messages.Text()

		// Drop message and respond if the 'batchBuffer' is at capacity.
		if len(messageIncomingQueue) >= config.queuecap {
			status := response(503, 0, "message queue full")
			conn.Write(status)
		} else {
			// Queue message and send response back to client.
			switch {
			case len(m) > maxMsgSize:
				status := response(400, len(m), "exceeds message size limit")
				conn.Write(status)
				messageIncomingQueue <- m[:maxMsgSize]
				conn.Close()
			case m == "\n":
				status := response(204, len(m), "received empty message")
				conn.Write(status)
			default:
				status := response(200, len(m), "received")
				conn.Write(status)
				messageIncomingQueue <- m
			}
		}
	}

}

// Generate response codes.
func response(code int, bytes int, info string) []byte {
	message := fmt.Sprintf("%d|%d|%s\n", code, bytes, info)
	return []byte(message)
}
