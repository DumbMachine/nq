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
![Task Queue Figure](/docs//assets/1.svg)
![Task Queue Figure](/docs//assets/2.svg)
This package was designed such that a task should always be cancellable by client. Upon network partision ( eg. disconnect from nats-server ), workers can be configured to cancel and quit instantly.

For scalable task execution, client can submit `task` to queues which are _load-balanced_ across servers ( subscribed to said queues ). When a task is to be `cancelled`, client issues `cancel` request to all servers subsribed to the `queue`, think multicast, and responsible server `cancels` executing task via go-context. A `task` in `state ∈ {Completed, Failed, Cancelled, Deleted}` cannot be cancelled. While a `task` still in queue, pending for it's execution, will be removed from the queue ( marked as `deleted` ) and a `task` in execution will marked be `cancelled` by calling `cancel` method on it's context.
For successful cancellations it is important that `ProcessingFunc`, the function executing said task, should respect `context` provided to it.

## Features

<!-- - Retries of failed tasks // todo -->

- Multiple task queues
- [Deadline and Timeout for tasks](#deadline--timeout-for-tasks)
- [Tasks can be cancelled via context](#task-cancellations)
- Horizontally scalable workers
- Reconnection to nats-server for automatic failover
- [Automatic failover](#automatic-failover)
- [Monitoring and Alerting](#monitoring-and-alerting)
- [CLI](#cli-usage)
<!-- - TODO: Nats cluster -->

# Deadline / Timeout for tasks

```go
// a task that executes till time.Now() + 1 hour
taskWithDeadline := nq.NewTask("my-queue", bytesPayload, nq.Deadline(time.Now().Add(time.Hour)), nq.TaskID("deadlineTaskID"))

// a task that executes for 10 minutes
taskWithTimeout := nq.NewTask("my-queue", bytesPayload, nq.Timeout(time.Minute * 10), nq.TaskID("timeoutTaskID"))
```

# Task cancellations

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

A **Task** can handle cancel like so:

```go
func longRunningOperation(ctx context.Context, task *nq.TaskPayload) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	for i := 0; i < 1000; i++ {
		timeout := time.Millisecond * 20
		println("sleeping for: ",timeout)
		time.Sleep(timeout)
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return nil
}
```

NOTE: Successful cancellation depends on task function respecting `context.Done()`.

# CLI Usage

Install CLI

```go
go install github.com/dumbmachine/nq/tools/nq@latest
```

- Cancel task

```bash
$ nq -u nats://127.0.0.1:4222 task cancel --id customID
taskID=customID status=Cancelled
```

- Status of task

```bash
$ nq -u nats://127.0.0.1:4222 task cancel --id customID
Cancel message sent task=customID
```

- Queue stats

```go
$ nq -u nats://127.0.0.1:4222 queue stats --name scrap-url-dev
queue: scrap-url-dev | MessagesPending: 11 | Size: 3025 Bytes
```

## Automatic Failover

`ShutdownOnNatsDisconnect` option will shutdown workers and server is connection to nats-server is broken. Useful when tasks being `cancellable` at all times is required.
Note: When disconnect is observed, workers would stop processing new messages. The workers would be cancelled in `shutdownTimeout` duration. If any tasks is/are not completed after this, they will be cancelled and still be available in task queue for future / other workers to process.

Auto-shutdown of worker server if at any time server is incapable of respecting a cancel request. Eg. losing connection to nats-server

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

```go
$ go run examples/simple.go sub
nq: pid=24914 2022/08/21 15:43:45.650999 INFO: Registered queue=scrap-url-dev
nq: pid=24914 2022/08/21 15:43:45.652720 INFO: Started Server@DumbmachinePro-local/24914
nq: pid=24914 2022/08/21 15:43:45.652739 INFO: [*] Listening for messages
nq: pid=24914 2022/08/21 15:43:45.652742 INFO: cmd/ctrl + c to terminate the process
nq: pid=24914 2022/08/21 15:43:45.652744 INFO: cmd/ctrl + z to stop processing new tasks
nq: pid=24914 2022/08/21 15:43:48.363110 ERROR: Disconnected from nats
nq: pid=24914 2022/08/21 15:43:48.363173 INFO: Starting graceful shutdown
nq: pid=24914 2022/08/21 15:43:53.363535 INFO: Waiting for all workers to finish...
nq: pid=24914 2022/08/21 15:43:53.363550 INFO: All workers have finished
nq: pid=24914 2022/08/21 15:43:53.363570 INFO: Exiting
```

Server can configured to not shutdown and instead try to reconnect to nats.

```go
srv := nq.NewServer(nq.NatsClientOpt{
		Addr:          "nats://127.0.0.1:4222",
		ReconnectWait: time.Second * 5, // controls timeout between reconnects
		MaxReconnects: 100, // controls total number of reconnects before giving up
	}, nq.Config{ServerName:  "local-serv-1"})
```

```go
$ go run examples/simple.go sub
2022/08/21 21:45:31 Startup subscriber ...
response: true2022/08/21 21:45:31 Created stream=scrap-url-dev/cancel subject=scrap-url-dev.cancel
nq: pid=26209 2022/08/21 16:15:31.507059 INFO: Registered queue=scrap-url-dev
nq: pid=26209 2022/08/21 16:15:31.507070 INFO: Started Server@DumbmachinePro-local/26209
nq: pid=26209 2022/08/21 16:15:31.507080 INFO: [*] Listening for messages
nq: pid=26209 2022/08/21 16:15:31.507083 INFO: cmd/ctrl + c to terminate the process
nq: pid=26209 2022/08/21 16:15:31.507086 INFO: cmd/ctrl + z to stop processing new tasks
nq: pid=26209 2022/08/21 16:15:37.011436 INFO: Recieved subject=scrap-url-dev task=customID
nq: pid=26209 2022/08/21 16:15:38.487646 INFO: Processed subject=scrap-url-dev task=customID
disconnected from nats
2022/08/21 21:45:48 reconnection found nats://127.0.0.1:4222
nq: pid=26209 2022/08/21 16:15:48.591125 INFO: Re-registering subscriptions to nats-server
response: true2022/08/21 21:45:48 Created stream=scrap-url-dev/cancel subject=scrap-url-dev.cancel
nq: pid=26209 2022/08/21 16:15:48.609046 INFO: Registered queue=scrap-url-dev
nq: pid=26209 2022/08/21 16:15:48.609145 INFO: Registration succesful[nats://127.0.0.1:4222]
nq: pid=26209 2022/08/21 16:15:52.883764 INFO: Recieved subject=scrap-url-dev task=customID
nq: pid=26209 2022/08/21 16:15:54.252731 INFO: Processed subject=scrap-url-dev task=customID
^C
nq: pid=26209 2022/08/21 16:15:58.979857 INFO: Starting graceful shutdown
nq: pid=26209 2022/08/21 16:15:58.980598 INFO: Waiting for all workers to finish...
nq: pid=26209 2022/08/21 16:15:58.980655 INFO: All workers have finished
nq: pid=26209 2022/08/21 16:15:58.981880 INFO: Exiting
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

<!-- For more checkout [Getting Started](/link/to/wiki) -->

To learn more about `nq` APIs, see [godoc](https://pkg.go.dev/github.com/dumbmachine/nq)

## Acknowledgements

[Async](https://github.com/hibiken/asynq) : Many of the design ideas are taken from async

## License

NQ is released under the MIT license. See LICENSE.
