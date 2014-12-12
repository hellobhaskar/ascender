// The MIT License (MIT)
//
// Copyright (c) 2014 Jamie Alquiza
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.
package main

import (
	"flag"
	"fmt"
	"github.com/crowdmob/goamz/aws"
	"github.com/crowdmob/goamz/sqs"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

// AWS vars.
var (
	accessKey = flag.String("aws-access-key",
		os.Getenv("ASCENDER_ACCESS_KEY"),
		"Required: AWS access key")
	secretKey = flag.String("aws-secret-key",
		os.Getenv("ASCENDER_SECRET_KEY"),
		"Required: AWS secret key")
	queueName = flag.String("aws-sqs-queue",
		os.Getenv("ASCENDER_SQS_QUEUE"),
		"Required: SQS queue name")
	regionString = flag.String("aws-sqs-region",
		os.Getenv("ASCENDER_SQS_REGION"),
		"Required: SQS queue region")
	region aws.Region
)

// Convert region human input to type 'aws.Region'.
func awsFormatRegion(r *string) aws.Region {
	var region aws.Region
	switch *r {
	case "us-gov-west-1":
		region = aws.USGovWest
	case "us-east-1":
		region = aws.USEast
	case "us-west-1":
		region = aws.USWest
	case "us-west-2":
		region = aws.USWest2
	case "eu-west-1":
		region = aws.EUWest
	case "ap-southeast-1":
		region = aws.APSoutheast
	case "ap-southeast-2":
		region = aws.APSoutheast2
	case "ap-northeast-1":
		region = aws.APNortheast
	case "sa-east-1":
		region = aws.SAEast
	case "":
		region = aws.USEast
	default:
		log.Fatalf("Invalid Region: %s\n", *r)
	}
	return region
}

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
	// Counters.
	sentCntr = make(chan int64, 1)
	// Graceful shutdown notification chans.
	sig_chan  = make(chan os.Signal)
	kill_chan = make(chan bool, 1)
	// Configs.
	ascender = ascenderConfig{}
	verbose  = flag.Bool("v", false, "verbose output")
)

func init() {
	// Init sent count at 0.
	sentCntr <- 0
	// Parse flags.
	flag.StringVar(&ascender.addr, "listen-addr", "localhost", "bind address")
	flag.StringVar(&ascender.port, "listen-port", "6030", "bind port")
	flag.IntVar(&ascender.workers, "workers", 3, "queue workers")
	flag.IntVar(&ascender.queuecap, "queue-cap", 100, "In-flight message queue capacity")
	flag.Parse()
	// Reassign vars.
	region = awsFormatRegion(regionString)
	sendQueue = make(chan []byte, ascender.queuecap)
	batchBuffer = make(chan []string, ascender.queuecap)
}

// Thread-safe global counter function via buffered channel.
func incrSent(v int64) {
	i := <-sentCntr
	sentCntr <- i + v
}

// Fetch counter val from channel then reload into buffer.
func fetchSent() int64 {
	i := <-sentCntr
	sentCntr <- i
	return i
}

// Generate response codes.
func response(code int, bytes int, info string) []byte {
	message := fmt.Sprintf("%d|%d|%s", code, bytes-1, info)
	return []byte(message)
}

// Receives messages from 'listener' & sends over 'sendQueue'.
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
	if len(sendQueue) >= 1 {
		status := response(503, 0, "send queue full")
		conn.Write(status)
		conn.Close()
	} else {
		// Queue message and send response back to client.
		switch {
		case mlen > maxMsgSize:
			status := response(400, mlen, "exceeds message size limit")
			conn.Write(status)
			conn.Close()
			sendQueue <- reqBuf[:maxMsgSize]
		case string(reqBuf[:mlen]) == "\n":
			status := response(204, mlen, "received empty message")
			conn.Write(status)
			conn.Close()
		default:
			status := response(200, mlen, "received")
			conn.Write(status)
			conn.Close()
			sendQueue <- reqBuf[:mlen]
		}
	}
}

// Listens for events, 'reqHandler' goroutine dispatched for each.
func listener() {
	log.Printf("Ascender listening on %s:%s\n",
		ascender.addr,
		ascender.port)
	server, err := net.Listen("tcp", ascender.addr+":"+ascender.port)
	if err != nil {
		log.Fatalf("Listener error: %s\n", err)
	}
	defer server.Close()
	// Connection handler loop.
	for {
		conn, err := server.Accept()
		if err != nil {
			fmt.Printf("Listener down: %s\n", err)
			continue
		}
		go reqHandler(conn)
	}
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

// Func to establish 'batchSender' connection to SQS.
func estabSqs(accessKey string, secretKey string, region aws.Region, queueName string) *sqs.Queue {
	auth := aws.Auth{AccessKey: accessKey, SecretKey: secretKey}
	client := sqs.New(auth, region)
	queue, err := client.GetQueue(queueName)
	if err != nil {
		log.Fatalf("SQS connection error: %s\n", err)
	}
	log.Printf("Connected to queue: %s\n", queue.Url)
	return queue
}

// 'batchBuffer' worker that reads message batches
// from the channel and sends into SQS.
func batchSender(c <-chan []string) {
	sqsConn := estabSqs(*accessKey, *secretKey, region, *queueName)
	for m := range c {
		_, err := sqsConn.SendMessageBatchString(m)
		if err != nil {
			fmt.Printf("SQS batch error: %s\n", err)
		}
		if *verbose {
			log.Printf("Sent %d message(s) into SQS\n",
				len(m))
		}
		incrSent(int64(len(m)))
	}
}

// Outputs periodic info summary.
func statsTracker() {
	tick := time.Tick(5 * time.Second)
	var currCnt, lastCnt int64
	for {
		select {
		case <-tick:
			lastCnt = currCnt
			currCnt = fetchSent()
			deltaCnt := currCnt - lastCnt
			if deltaCnt > 0 {
				log.Printf("Last 5s: sent %d messages | Avg: %.2f messages/sec. | Send queue length: %d\n",
					deltaCnt,
					float64(deltaCnt)/5,
					len(sendQueue))
			}
		}
	}
}

// Handles signal events.
// Currently just kills service. 'kill_chan' will eventually signal
// the listener to stop and any in-flight messages to complete.
func signalHandler() {
	signal.Notify(sig_chan, syscall.SIGINT)
	for {
		runtime.Gosched()
		select {
		case <-sig_chan:
			kill_chan <- true
			// Do things, then:
			os.Exit(0)
		}
	}
}

func main() {
	// Start signal handler.
	go signalHandler()
	// Start the listener server.
	go listener()
	// Start info goroutine.
	go statsTracker()
	// Start workers.
	for i := 0; i < ascender.workers; i++ {
		go batchSender(batchBuffer)
	}
	// Start the message handler.
	go msgHandler()
	// Start signal handler.
	signalHandler()
}
