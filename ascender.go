// 2014, 2015 Jamie Alquiza
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jamiealquiza/ascender/outputs/console"
	"github.com/jamiealquiza/ascender/outputs/sqs"
	"github.com/jamiealquiza/ghostats"
)

var (
	// Channel that listeners pass received messages to
	// for consumption by messageHandler.
	// Limits number of in-flight message and subsequently is
	// a large dictator of Ascender memory usage.
	messageIncomingQueue = make(chan string, options.queuecap)
	// Queue that messageHandler loads message batches into.
	// Output handlers read batches and send to destinations.
	messageOutgoingQueue = make(chan []string, options.queuecap)
	// Timeout to force the current messageHandler batch to the messageOutgoingQueue.
	flushTimeout = time.Tick(5 * time.Second)

	options struct {
		addr     string
		port     string
		handlers int
		queuecap int
		console  bool
	}

	config struct {
		batchSize    int
		flushTimeout int
	}

	sig_chan = make(chan os.Signal)
)

func init() {
	flag.StringVar(&options.addr, "listen-addr", "localhost", "bind address")
	flag.StringVar(&options.port, "listen-port", "6030", "bind port")
	flag.IntVar(&options.handlers, "handlers", 3, "Queue handlers")
	flag.IntVar(&options.queuecap, "queue-cap", 1000, "In-flight message queue capacity")
	flag.BoolVar(&options.console, "console-out", false, "Dump output to console")
	flag.Parse()
	// Update vars that depend on flag inputs.
	messageIncomingQueue = make(chan string, options.queuecap)
	messageOutgoingQueue = make(chan []string, options.queuecap)
}

// Receives messages on messageIncomingQueue, batches into message groups
// and flushes into the 'messageOutgoingQueue' channel when the batch
// hits either the configured config.batchSize or flushTimeout treshold.
func messageHandler() {
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
			if len(messages)+1 >= config.batchSize {
				messages = append(messages, msg)
				messageOutgoingQueue <- messages
				messages = []string{}
			} else {
				// Otherwise, just append message to current batch.
				messages = append(messages, msg)
			}
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
	go ghostats.Start("localhost", "6040", nil)

	// Start outputs
	for i := 0; i < options.handlers; i++ {
		if options.console {
			config.batchSize = 1
			go console.Handler(messageOutgoingQueue)
		} else {
			// AWS SQS max batch size is currently 10.
			config.batchSize = 10
			go sqs.Handler(messageOutgoingQueue, sentCnt)
		}
	}

	runControl()
}
