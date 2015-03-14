ascender
========

# Overview
A lightweight, plaintext TCP to message queue gateway (currently AWS SQS only). Ascender is intended to make it easy to send arbitrary data into message queues without having to write boilerplate code for simple tasks.

For instance, if Ascender were listening on `localhost:6030`, it would be valid to do the following:
<pre>
$ echo $(grep some-info some.log) | nc localhost 6030
</pre>
or
<pre>
$ echo '{ "@timestamp": "'$(date +%s)'", "@type": "pakages", "@hostname": "'$(hostname)'", '$(echo $(dpkg -l | awk '/^ii/ {print "\""$2"\": " "\""$3"\", "}' | head -30) | sed 's/,$//')'}' | nc localhost 6030
</pre>

If the above were fed into [Langolier](https://github.com/jamiealquiza/langolier) and indexed into ElasticSearch, you could visualize the data as:

![ScreenShot](http://us-east.manta.joyent.com/jalquiza/public/github/ascender-packages.png)

This makes it easy to index, search on and visualize arbitrary information from thousands of hosts using simple one-liners.

# Mechanics
Ascender starts up a TCP listener and accepts linefeed delimited messages up to 256KB (the current AWS SQS max per-message size). Messages larger than this are truncated to 256K. Message are aggregated into 10 count message batches (the current AWS max batch size) and pushed into an internal queue (default 100, defined by `--send-queue` directive). A configurable number of workers (default 3, defined by `-workers` directive) pop and send message batches as fast as possible from the queue. SQS throughput is heavily determined by latency; Ascender maximizes the total throughput by allowing a deep queue and many concurrent workers (each with a seperate connection to SQS).

Performance note: Ascender is intended to be ran as a lightweight daemon on every node in your fleet, and by default will use only a single core (which is more than enough for most cases). If you run a dedicated Ascender box that will process high event rates, simply raise the '-workers' directive and set the GOMAXPROCS environment variable to n CPUs to maximize parallelism.

# Install

Assuming Go is installed (tested up to version 1.4.1) and $GOPATH is set:
- `go get github.com/jamiealquiza/ascender`
- `go build github.com/jamiealquiza/ascender`

Binary will be found at: `$GOPATH/bin/ascender`

Example Upstart:
<pre>
% cat /etc/init/ascender.conf
description "Ascender - SQS Gateway Service"

start on runlevel [2345]
stop on runlevel [!2345]

console log

respawn
respawn limit 5 30

setuid ascender
setgid nogroup

script
  chdir /opt/ascender
  export ASCENDER_ACCESS_KEY="xxx"
  export ASCENDER_SECRET_KEY="xxxxxx"
  export ASCENDER_SQS_REGION="us-west-2"
  export ASCENDER_SQS_QUEUE="somequeue"
  exec ./ascender
end script
</pre>
# Usage

Requires several AWS settings (see `./ascender -h` output) which can optionally be applied as environment variables*.

Usage / help:
<pre>
Usage of ./ascender:
  -aws-access-key="": Required: AWS access key
  -aws-secret-key="": Required: AWS secret key
  -aws-sqs-queue="": Required: SQS queue name
  -aws-sqs-region="": Required: SQS queue region
  -listen-addr="localhost": bind address
  -listen-port="6030": bind port
  -queue-cap=100: In-flight message queue capacity
  -workers=3: queue workers
</pre>

*env vars:
<pre>
ASCENDER_ACCESS_KEY
ASCENDER_SECRET_KEY
ASCENDER_SQS_REGION
ASCENDER_SQS_QUEUE
</pre>

Server:
<pre>
% ./ascender
2014/10/24 15:53:58 Ascender listening on localhost:6030
2014/10/24 15:53:59 Connected to queue: https://sqs.us-west-2.amazonaws.com/000/testing
2014/10/24 15:53:59 Connected to queue: https://sqs.us-west-2.amazonaws.com/000/testing
2014/10/24 15:53:59 Connected to queue: https://sqs.us-west-2.amazonaws.com/000/testing
2014/10/24 15:54:06 Last 5s: sent 1 messages | Avg: 0.20 messages/sec. | Send queue length: 0
</pre>

Client:
<pre>
% echo '{ "@timestamp": "'$(date +%s)'", "@type": "mytype", "hello": "world" }' | nc localhost 6030
200|68|received
</pre>

# Response Codes:
Pending refinement. Format:
<pre>
code|bytes received|info
</pre>

Examples:

### 200 received
32 byte message received: `200|32|received`

### 204 received empty message
No message body, no action taken: `204|0|received empty message`

### 400 exceeds message size limit
Message is larger than 256K, truncated and queued for sending: `400|266240|exceeds message size limit`

### 503 message queue full
Messages in-flight exceeds `-queue-cap`, new messages are dropped until queue has open slots: `503|-1|message queue full`

# Admin / Stats API
WIP. Ascender runs an instance of [Ghostats](https://github.com/jamiealquiza/ghostats) that exposes Go runtime data over TCP.

<pre>
% echo stats | nc localhost 6040
{
  "runtime-meminfo": {
    "Alloc": 1422664,
    "BuckHashSys": 1441776,
    "Frees": 67398,
    "GCSys": 268436,
    "HeapAlloc": 1422664,
    "HeapIdle": 1343488,
    "HeapInuse": 2441216,
    "HeapObjects": 13561,
    "HeapReleased": 0,
    "HeapSys": 3784704,
    "LastGC": 1424214667658042851,
    "Lookups": 13,
    "MCacheInuse": 1200,
    "MCacheSys": 16384,
    "MSpanInuse": 31304,
    "MSpanSys": 32768,
    "Mallocs": 80959,
    "NextGC": 2745168,
    "NumGC": 11,
    "OtherSys": 274548,
    "PauseTotalNs": 4307280,
    "StackInuse": 409600,
    "StackSys": 409600,
    "Sys": 6228216,
    "TotalAlloc": 4620208
  },
  "service": {
    "start-time": "2015-02-17T16:11:06-07:00",
    "uptime-seconds": 3
  }
}
</pre>

# To Do
- Close issues ;)
- More thorough documentation
- Lots more
