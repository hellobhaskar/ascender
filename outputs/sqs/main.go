// 2014, 1015 Jamie Alquiza
package sqs

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/jamiealquiza/ascender/vendor/github.com/AdRoll/goamz/aws"
	"github.com/jamiealquiza/ascender/vendor/github.com/AdRoll/goamz/sqs"
)

var (
	accessKey = flag.String("aws-access-key",
		os.Getenv("ASCENDER_ACCESS_KEY"),
		"AWS access key")
	secretKey = flag.String("aws-secret-key",
		os.Getenv("ASCENDER_SECRET_KEY"),
		"AWS secret key")
	queueName = flag.String("aws-sqs-queue",
		os.Getenv("ASCENDER_SQS_QUEUE"),
		"SQS queue name")
	regionString = flag.String("aws-sqs-region",
		os.Getenv("ASCENDER_SQS_REGION"),
		"SQS queue region")
	region aws.Region
)

func init() {
	flag.Parse()
	region = awsFormatRegion(regionString)
}

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

func init() {
	flag.Parse()
	region = awsFormatRegion(regionString)
}

type Statser interface {
	IncrSent(int64)
	FetchSent() int64
}

// Worker that reads message batches from the messageOutgoingQueue
// and writes to SQS.
func Sender(messageOutgoingQueue <-chan []string, s Statser) {
	sqsConn := estabSqs(*accessKey, *secretKey, region, *queueName)
	for m := range messageOutgoingQueue {
		_, err := sqsConn.SendMessageBatchString(m)
		if err != nil {
			fmt.Printf("SQS batch error: %s\n", err)
		}
		s.IncrSent(int64(len(m)))
	}
}

// estabSqs establishes a connection to SQS.
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
