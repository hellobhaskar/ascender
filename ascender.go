// 2014, 2015 Jamie Alquiza
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jamiealquiza/ascender/outputs/sqs"
	"github.com/jamiealquiza/ascender/util/ghostats"
)

// General server config struct.
type ascenderConfig struct {
	addr     string
	port     string
	workers  int
	queuecap int
}

var (
	// Comm. for 'reqHandler' to pass messages to the 'msgHandler'.
	// Limits number of in-flight message and subsequently a
	// is a large dictator of Ascender memory limits.
	sendQueue = make(chan []byte, ascender.queuecap)
	// Queue that 'msgHandler' produces to and 'batchSender'
	// workers consume from.
	batchBuffer = make(chan []string, ascender.queuecap)
	// Timeout to force current 'msgHandler' batch to the 'batchBuffer'.
	flushTimeout = time.Tick(5 * time.Second)

	sig_chan = make(chan os.Signal)
	ascender = ascenderConfig{}
)

func init() {
	// Parse flags.
	flag.StringVar(&ascender.addr, "listen-addr", "localhost", "bind address")
	flag.StringVar(&ascender.port, "listen-port", "6030", "bind port")
	flag.IntVar(&ascender.workers, "workers", 3, "queue workers")
	flag.IntVar(&ascender.queuecap, "queue-cap", 100, "In-flight message queue capacity")
	flag.Parse()
	// Update vars that dep on flags.
	sendQueue = make(chan []byte, ascender.queuecap)
	batchBuffer = make(chan []string, ascender.queuecap)
}

// Receives messages on 'sendQueue', batches into message groups
// and flushes into the 'batchBuffer' channel.
func msgHandler() {
	// AWS SQS max batch size is currently 10.
	batchSize := 10
	messages := []string{}
	for {
		select {
		case <-flushTimeout:
			// We hit the flush timeout, see if there's messages to ship.
			if len(messages) > 0 {
				batchBuffer <- messages
				messages = []string{}
			}
			messages = []string{}
		case msg := <-sendQueue:
			// Enqueue and reset message batch if we're at 'batchSize'.
			if len(messages) == batchSize {
				batchBuffer <- messages
				messages = []string{}
			}
			// Otherwise, append message to batch.
			messages = append(messages, string(msg))
		}
	}
}

// Handles signal events.
// Currently just kills service, will eventually perform graceful shutdown.
func runControl() {
	signal.Notify(sig_chan, syscall.SIGINT)
	<-sig_chan
	log.Printf("Ascender shutting down")
	os.Exit(0)
}

func main() {
	go listenTcp()
	go msgHandler()
	go ghostats.Start("localhost", "6031", ascender)

	sentCnt := NewStatser()
	go statsTracker(sentCnt)

	for i := 0; i < ascender.workers; i++ {
		go sqs.BatchSender(batchBuffer, sentCnt)
	}

	runControl()
}
