// 2014, 2015 Jamie Alquiza
package main

import (
	"fmt"
	"io"
	"log"
	"net"
)

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
	// We match SQS max message size since it's
	// the reference message queue (for now).
	maxMsgSize := 256 * 1024
	reqBuf := make([]byte, maxMsgSize)
	// Get size of received message.
	mlen, err := conn.Read(reqBuf)
	if err != nil && err != io.EOF {
		fmt.Println(err.Error())
	}
	// Drop message and respond if the 'batchBuffer' is at capacity.
	if len(messageIncomingQueue) >= config.queuecap {
		status := response(503, 0, "message queue full")
		conn.Write(status)
		conn.Close()
	} else {
		// Queue message and send response back to client.
		switch {
		case mlen > maxMsgSize:
			status := response(400, mlen, "exceeds message size limit")
			conn.Write(status)
			conn.Close()
			messageIncomingQueue <- reqBuf[:maxMsgSize]
		case string(reqBuf[:mlen]) == "\n":
			status := response(204, mlen, "received empty message")
			conn.Write(status)
			conn.Close()
		default:
			status := response(200, mlen, "received")
			conn.Write(status)
			conn.Close()
			messageIncomingQueue <- reqBuf[:mlen]
		}
	}
}

// Generate response codes.
func response(code int, bytes int, info string) []byte {
	message := fmt.Sprintf("%d|%d|%s\n", code, bytes-1, info)
	return []byte(message)
}
