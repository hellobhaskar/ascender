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
Ascender starts up a TCP listener and accepts individual messages up to 256KB (the current AWS SQS max per-message size). Messages larger than this are truncated to 256K. Message are aggregated into 10 count message batches (the current AWS max batch size) and queued into a buffer (default 100, defined by '-send-queue' directive). A configurable number of workers (default 3, defined by '-workers' directive) dequeue and fire off message batches as fast as possible from the buffer. SQS throughput is heavily determined by latency; Ascender maximizes the total throughput by allowing a deep queue and many concurrent workers (each with a seperate connection to SQS).

Performance note: Ascender is intended to be ran as a lightweight daemon on every node in your fleet, and by default will use only a single core (which is more than enough for most cases). If you run a dedicated Ascender box that will process high event rates, simply raise the '-workers' directive and set the GOMAXPROCS environment variable to n CPUs to maximize parallelism.

# Usage

Clone repo, build, run. Requires several AWS settings (see `./ascender -h` output) which can optionally be applied as environment variables*.

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
  -v=false: verbose output
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

### 503 send queue full
Messages in-flight exceeds `--send-queue`, new messages are dropped: `503|-1|send queue full`

# To Do
- Close issues ;)
- More thorough documentation
- Lots more
