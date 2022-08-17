# Reliable, Efficient and Cancellable Distributed Task Queue in Go

[![Go Report Card](https://goreportcard.com/badge/github.com/dumbmachine/nq)](https://goreportcard.com/report/github.com/dumbmachine/nq)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](https://opensource.org/licenses/MIT)
[![GoDoc](https://godoc.org/github.com/dumbmachine/nq?status.svg)](https://godoc.org/github.com/dumbmachine/nq)

NQ ( Nats Queue ) is Go package to queuing and processing jobs in background with workers. Backend by nats-server and with a focus on cancel-ability of enqueued jobs.

Overview of how NQ works:

- Client puts tasks into streams ( nats jetstream )
- Server pulls those tasks and executes them in a goroutine worker
- Processed concurrently by multiple workers
- Tasks states are stored in [nats key-value store](https://docs.nats.io/nats-concepts/jetstream/key-value-store). ( An `interface` can be implemented to support other stores )

Task streams can be used to distribute work across multiple machines, where machines can run server ( worker server ) and brokers, for high availability and horizontal scaling.

NQ requires nats-server with jetstream support. Feedback is appreciated.

**How does it work?:**
![Task Queue Figure](/docs//assets/nq.drawio.svg)
This package was designed such that a task should always be cancellable by client. Upon network partision ( eg. disconnect from nats-server ), workers can be configured to cancel and quit instantly.

For scalable task execution, client can submit `task` to queues which are _load-balanced_ across servers ( subscribed to said queues ). When a task is to be `cancelled`, client issues `cancel` request to all servers subsribed to the `queue`, think multicast, and responsible server `cancels` executing task via go-context. A `task` in `state ∈ {Completed, Failed, Cancelled}` cannot be cancelled. While a `task` still in queue, pending for it's execution, will be removed from the queue and a `task` in execution will be cancelled by calling `cancel` method on it's context.
For successful cancellations it is important that `ProcessingFunc`, the function executing said task, should respect `context` provided to it.

To learn more about stream, subjects and other nats related terms refer

## Features

<!-- - Retries of failed tasks // todo -->

- Multiple task queues
- Deadline and Timeout for tasks
- CLI to inspect queues
- Tasks can be cancelled via context
- Auto-shutdown of worker server if at any time server is incapable of respecting a cancel request. Eg. losing connection to nats-server
- Reconnection to nats-server for automatic failover
- Cancel tasks at any time by id.
- Horizontally scalable workers

## Something

- Cancel tasks
- Fetch task status
- Automatic failover
- Monitoring and Alerting
<!-- - TODO: Nats cluster -->

## Cancel Tasks

Tasks that are either waiting for execution or being executed on any worker, can be `cancelled`. Cancellation of a task requires it's `taskID`.

```go
// Cancel a task by ID
ack, err := client.Enqueue(taskSignature);
client.Cancel(ack.ID)
```

```go
// If queue name is known, faster way to issue cancel request
ack, err := client.Enqueue(taskSignature);
client.CancelInQueue(ack.ID, "<queue-name>")
```

## Fetch task status

```go
msg, _ := client.Fetch(ack.ID)
msgStatus := msg.GetStatus() // string formatted status
```

## Automatic Failover

`ShutdownOnNatsDisconnect` option will shutdown workers and server is connection to nats-server is broken. Useful when tasks being `cancellable` at all times is of utmost importance. Note: When disconnect is observed, workers would stop processing instantly and if any remaining task/s will stay in the message queue ( available to other instances of worker-servers, in multi-deployment )

```go
srv := nq.NewServer(nq.NatsClientOpt{
	Addr: "nats://127.0.0.1:4222",
}, nq.Config{
	ServerName:  nq.GenerateServerName(),
	Concurrency: 2,
	LogLevel:    nq.InfoLevel,
}, nq.NoAuthentcation(), nq.ShutdownOnNatsDisconnect(),
)
```

Server can configured to not shutdown and instead try to reconnect to nats.

```go
srv := nq.NewServer(nq.NatsClientOpt{
		Addr:          "nats://127.0.0.1:4222",
		ReconnectWait: time.Second * 5,
		MaxReconnects: 100,
	}, nq.Config{ServerName:  "local-serv-1"})

```

## Monitoring and Alerting

Refer [nats monitoring section](https://docs.nats.io/running-a-nats-service/configuration/monitoring) and [monitoring tool by nats-io](https://github.com/nats-io/nats-surveyor)

## Quickstart

Install NQ library

```bash
go get -u github.com/dumbmachine/nq
```

Make sure you have nats-server running locally or in a container. Example:

```bash
docker run --rm -p 4222:4222 --name nats-server -ti nats:latest -js
```

Now create a client to publish jobs.

```go

package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"

	"net/http"
	"time"

	"github.com/dumbmachine/nq"
	"github.com/dumbmachine/nq/base"
)

type UrlPayload struct {
	Url string `json:"url"`
}

// Stream / Subject name
const (
	StreamName      = "scrap-url"
	SubjectNameDev  = "scrap-url-dev"
	SubjectNameProd = "scrap-url-prod"
)

func main() {
	log.Info("Startup publisher ...")
	client := NewPublishClient(NatsClientOpt{
		Addr: "nats://127.0.0.1:4222",
	}, config.NoAuthentcation(),
	// see godoc for more options
	)

	defer client.Close()

	stream, err := client.CreateTaskStream(StreamName, []string{
		SubjectNameDev, SubjectNameProd,
	})
	if err != nil {
		panic(err)
	}

	defer stream.Delete()

	b, err := json.Marshal(UrlPayload{Url: "https://httpstat.us/200?sleep=5000"})
	if err != nil {
		log.Println(err)
	}

	task1 := config.NewTask(SubjectNameDev, b)
	if ack, err := stream.Publish(task1); err == nil {
		log.Println("Acknowledgement: ", ack.ID)
	} else {
		log.Println(err)
	}

}

```

Now create a worker server

```go
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"

	"net/http"
	"time"

	"github.com/dumbmachine/nq"
	"github.com/dumbmachine/nq/base"
)

type UrlPayload struct {
	Url string `json:"url"`
}

// An example function
func fetchHTML(ctx context.Context, task *TaskPayload) error {
	var payload UrlPayload
	if err := json.Unmarshal(task.Payload, &payload); err != nil {
		return errors.New("invalid payload")
	}
	client := &http.Client{Timeout: 30 * time.Second}
	req, _ := http.NewRequest("GET", payload.Url, nil)
	req = req.WithContext(ctx)
	if _, err := client.Do(req); err != nil {
		return err
	}
	return nil
}

// Stream / Subject name
const (
	StreamName      = "scrap-url"
	SubjectNameDev  = "scrap-url-dev"
	SubjectNameProd = "scrap-url-prod"
)

func main() {
	log.Println("Startup subscriber ...")
	srv := config.NewServer(config.NatsClientOpt{
		Addr: "nats://127.0.0.1:4222",
	}, config.Config{
		ServerName:  "local",
		Concurrency: 1,
		LogLevel:    log.InfoLevel,
	}, config.NoAuthentcation(), config.ShutdownOnNatsDisconnect())

	srv.Register(StreamName, SubjectNameDev, fetchHTML)

	if err := srv.Run(); err != nil {
		panic(err)
	}
}

```

Note: New messages are fetched from queue in sequencial order of their registration. NQ does not implement any custom priority order for registered queue yet.

For more checkout [Getting Started](/link/to/wiki)
To learn more about `nq` APIs, see [godoc](/link/to/godoc)

## Acknowledgements

[Async](https://github.com/hibiken/asynq) : Many of the design ideas are taken from async

## License

NQ is released under the MIT license. See LICENSE.
