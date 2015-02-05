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

// Generate stats response.
func buildStats(meta interface{}) []byte {
	// Get current MemStats.
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	// We swipe the stats we want.
	// Reference: http://golang.org/pkg/runtime/#ReadMemStats
	memInfo := make(map[string]interface{})
	memInfo["Alloc"] = mem.Alloc
	memInfo["TotalAlloc"] = mem.TotalAlloc
	memInfo["Sys"] = mem.Sys
	memInfo["Lookups"] = mem.Lookups
	memInfo["Mallocs"] = mem.Mallocs
	memInfo["Frees"] = mem.Frees
	memInfo["HeapAlloc"] = mem.HeapAlloc
	memInfo["HeapSys"] = mem.HeapSys
	memInfo["HeapIdle"] = mem.HeapIdle
	memInfo["HeapInuse"] = mem.HeapInuse
	memInfo["HeapReleased"] = mem.HeapReleased
	memInfo["HeapObjects"] = mem.HeapObjects
	memInfo["StackInuse"] = mem.StackInuse
	memInfo["StackSys"] = mem.StackSys
	memInfo["MSpanInuse"] = mem.MSpanInuse
	memInfo["MSpanSys"] = mem.MSpanSys
	memInfo["MCacheInuse"] = mem.MCacheInuse
	memInfo["MCacheSys"] = mem.MCacheSys
	memInfo["BuckHashSys"] = mem.BuckHashSys
	memInfo["GCSys"] = mem.GCSys
	memInfo["OtherSys"] = mem.OtherSys
	memInfo["NextGC"] = mem.NextGC
	memInfo["LastGC"] = mem.LastGC
	memInfo["PauseTotalNs"] = mem.PauseTotalNs
	memInfo["NumGC"] = mem.NumGC

	response, err := json.MarshalIndent(memInfo, "", "  ")
	if err != nil {
		log.Printf("Error parsing: %s", err)
	}
	return response
}