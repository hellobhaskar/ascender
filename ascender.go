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
	"github.com/jamiealquiza/ghostats"
)

type ascenderConfig struct {
	addr     string
	port     string
	workers  int
	queuecap int
}

var (
	// Channel that listeners pass received messages to
	// for consumption by messageHandler.
	// Limits number of in-flight message and subsequently is 
	// a large dictator of Ascender memory usage.
	messageIncomingQueue = make(chan []byte, config.queuecap)
	// Queue that messageHandler loads message batches into.
	// Output workers read batches and send to destinations.
	messageOutgoingQueue = make(chan []string, config.queuecap)
	// Timeout to force the current messageHandler batch to the messageOutgoingQueue.
	flushTimeout = time.Tick(5 * time.Second)

	sig_chan = make(chan os.Signal)
	config = ascenderConfig{}
)

func init() {
	flag.StringVar(&config.addr, "listen-addr", "localhost", "bind address")
	flag.StringVar(&config.port, "listen-port", "6030", "bind port")
	flag.IntVar(&config.workers, "workers", 3, "queue workers")
	flag.IntVar(&config.queuecap, "queue-cap", 100, "In-flight message queue capacity")
	flag.Parse()
	// Update vars that depend on flag inputs.
	messageIncomingQueue = make(chan []byte, config.queuecap)
	messageOutgoingQueue = make(chan []string, config.queuecap)
}

// Receives messages on messageIncomingQueue, batches into message groups
// and flushes into the 'messageOutgoingQueue' channel when the batch
// hits either the configured batchSize or flushTimeout treshold.
func messageHandler() {
	// AWS SQS max batch size is currently 10.
	batchSize := 10
	messages := []string{}
	for {
		select {
		case <-flushTimeout:
			// We hit the flush timeout, load the current batch if present.
			if len(messages) > 0 {
				messageOutgoingQueue <- messages
				messages = []string{}
			}
			messages = []string{}
		case msg := <-messageIncomingQueue:
			// If this puts us at the batchSize threshold, enqueue
			// into the messageOutgoingQueue.
			if len(messages) == batchSize {
				messageOutgoingQueue <- messages
				messages = []string{}
			}
			// Otherwise, just append message to current batch.
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
	// Start internals.
	go listenTcp()
	go messageHandler()

	// Start stat services.	
	sentCnt := NewStatser()
	go statsTracker(sentCnt)
	go ghostats.Start("localhost", "6031", nil)

	// Start outputs
	for i := 0; i < config.workers; i++ {
		go sqs.Sender(messageOutgoingQueue, sentCnt)
	}

	runControl()
}