// 2015 Jamie Alquiza
package ghostats

import (
	"runtime"
	"encoding/json"
	"net"
	"fmt"
	"log"
	"io"
	"strings"
)

func Start(address, port string, config interface{}) {
	log.Printf("Stats API started: %s:%s\n",
		address,
		port)
	server, err := net.Listen("tcp", address+":"+port)
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
		go reqHandler(conn, config)
	}
}

func reqHandler(conn net.Conn, meta interface{}) {
	reqBuf := make([]byte, 8)
	mlen, err := conn.Read(reqBuf)
	if err != nil && err != io.EOF {
		fmt.Println(err.Error())
	}

	req := strings.TrimSpace(string(reqBuf[:mlen]))
	switch req {
	case "stats":
		r := buildStats(meta)
		conn.Write(r)
		conn.Close()
	default:
		m := fmt.Sprintf("Not a command: %s\n", req)
		conn.Write([]byte(m))
		conn.Close()
	}	
}
// http://golang.org/pkg/runtime/#ReadMemStats
// Generate response codes.
func buildStats(meta interface{}) []byte {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	memStats, err := json.MarshalIndent(mem, "", "  ")
	if err != nil {
		log.Printf("Error parsing: %s", err)
	}
	m := fmt.Sprintf("%s\n%s\n", meta, memStats)
	return []byte(m)
}