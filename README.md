# Reliable, Efficient and Cancellable Distributed Task Queue in Go

[![Go Report Card](https://goreportcard.com/badge/github.com/dumbmachine/nq)](https://goreportcard.com/report/github.com/dumbmachine/nq)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](https://opensource.org/licenses/MIT)
[![GoDoc](https://godoc.org/github.com/dumbmachine/nq?status.svg)](https://godoc.org/github.com/dumbmachine/nq)

NQ ( Nats Queue ) is Go package for queuing and processing jobs in background with workers. Based on [nats](https://nats.io/) with a focus on cancel-ability of enqueued jobs.

NQ requires nats-server version that supports both jetstream support and key-value store

**How does it work?:**

|                                                           ![Task Queue Figure](/docs//assets/1.svg) ![Task Queue Figure](/docs//assets/2.svg)                                                           |
| :-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------: |
| _This package was designed such that a task should always be cancellable by client. Workers can be configured to cancel and quit instantly upon network partision ( eg. disconnect from nats-server )._ |

<!-- NQ Client submits `task` to queues which are _load-balanced_ across servers ( subscribed to said queues ). When a task is to be `cancelled`, client issues `cancel` request to all servers subsribed to the `queue`, think multicast, and responsible server `cancels` executing task via go-context. A `task` in `state ∈ {Completed, Failed, Cancelled, Deleted}` cannot be cancelled. While a `task` still in queue, pending for it's execution, will be removed from the queue ( marked as `deleted` ) and a `task` in execution will marked be `cancelled` by calling `cancel` method on it's context.
For successful cancellations it is important that `ProcessingFunc`, the function executing said task, should respect `context` provided to it. -->

## Features

<!-- - Retries of failed tasks // todo -->

- Multiple task queues
- [Deadline and Timeout for tasks](#task-options-walkthrough)
- [Tasks can be cancelled via context](#task-cancellations)
- Horizontally scalable workers
- [Automatic failover](#automatic-failover)
- [Reconnection to nats-server for automatic failover](#reconnection)
- [Monitoring and Alerting](#monitoring-and-alerting)
- [CLI](#cli-usage)
<!-- - TODO: Nats cluster -->

# Task Options Walkthrough

## Retrying

By default `task` is submitted for retry, if it returns non-nil error.

```go
// a task that will be retried 2 before being marked as `failed`
taskWithRetry := nq.NewTask("my-queue", bytesPayload, nq.Retry(2))
```

Custom filtering function for error, to mark task as failed only on specific error.
Here if a task fails due to `ErrFailedDueToInvalidApiKeys`, it will be consider as `failure` and will be retried

```go
var ErrFailedDueToInvalidApiKeys = errors.New("failed to perform task, invalid api keys")

srv := nq.NewServer(nq.NatsClientOpt{Addr: nats.DefaultURL}, nq.Config{
	IsFailureFn: func(err error) bool {
		return errors.Is(err, ErrFailedDueToInvalidApiKeys)
	},
	ServerName:  nq.GenerateServerName(),
})

```

## Deadline / Timeout for tasks

```go
// a task that executes till time.Now() + 1 hour
taskWithDeadline := nq.NewTask("my-queue", bytesPayload, nq.Deadline(time.Now().Add(time.Hour)), nq.TaskID("deadlineTaskID"))

// a task that executes for 10 minutes
taskWithTimeout := nq.NewTask("my-queue", bytesPayload, nq.Timeout(time.Minute * 10), nq.TaskID("timeoutTaskID"))
```

## Task cancellations

Tasks that are either waiting for execution or being executed on any worker, can be `cancelled`. Cancellation of a task requires it's `taskID`.

```go
// Cancel a task by ID
taskSignature := nq.NewTask("my-queue", []byte())
ack, err := client.Enqueue(taskSignature);
client.Cancel(ack.ID)
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

# Automatic Failover

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
}, nq.ShutdownOnNatsDisconnect(),
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

## Reconnection

Server can configured to not shutdown and instead try to reconnect to nats, when disconnected.

```go
srv := nq.NewServer(nq.NatsClientOpt{
		Addr:          "nats://127.0.0.1:4222",
		ReconnectWait: time.Second * 5, // controls timeout between reconnects
		MaxReconnects: 100, // controls total number of reconnects before giving up
	}, nq.Config{ServerName:  "local-serv-1"})
```

If nats-server is up again:

1. With previous state ( i.e with expected queue data )

   ```bash
   nq: pid=7988 2022/08/22 17:24:44.349815 INFO: Registered queue=scrap-url-dev
   nq: pid=7988 2022/08/22 17:24:44.356378 INFO: Registered queue=another-one
   nq: pid=7988 2022/08/22 17:24:44.356393 INFO: Started Server@DumbmachinePro-local/7988
   nq: pid=7988 2022/08/22 17:24:44.356444 INFO: [*] Listening for messages
   nq: pid=7988 2022/08/22 17:24:44.356455 INFO: cmd/ctrl + c to terminate the process
   nq: pid=7988 2022/08/22 17:24:44.356459 INFO: cmd/ctrl + z to stop processing new tasks
   disconnected from nats
   2022/08/22 22:55:02 reconnection found nats://127.0.0.1:4222
   nq: pid=7988 2022/08/22 17:25:02.860051 INFO: Re-registering subscriptions to nats-server
   nq: pid=7988 2022/08/22 17:25:02.864988 INFO: Registration successful[nats://127.0.0.1:4222]
   disconnected from nats
   ```

2. Without previous state
   If registered queues are not found in nats-server, they will be created
   ```bash
   nq: pid=7998 2022/08/22 17:26:44.349815 INFO: Registered queue=scrap-url-dev
   nq: pid=7998 2022/08/22 17:26:44.356378 INFO: Registered queue=another-one
   nq: pid=7998 2022/08/22 17:26:44.356393 INFO: Started Server@DumbmachinePro-local/7998
   nq: pid=7998 2022/08/22 17:26:44.356444 INFO: [*] Listening for messages
   nq: pid=7998 2022/08/22 17:26:44.356455 INFO: cmd/ctrl + c to terminate the process
   nq: pid=7998 2022/08/22 17:26:44.356459 INFO: cmd/ctrl + z to stop processing new tasks
   disconnected from nats
   2022/08/22 22:57:25 reconnection found nats://127.0.0.1:4222
   nq: pid=7998 2022/08/22 17:27:25.518079 INFO: Re-registering subscriptions to nats-server
   nq: pid=7998 2022/08/22 17:27:25.524895 WARN: stream=scrap-url-dev re-registering
   nq: pid=7998 2022/08/22 17:27:25.542725 INFO: Registered queue=scrap-url-dev
   nq: pid=7998 2022/08/22 17:27:25.543668 WARN: stream=another-one re-registering
   nq: pid=7998 2022/08/22 17:27:25.554961 INFO: Registered queue=another-one
   nq: pid=7998 2022/08/22 17:27:25.555002 INFO: Registration successful[nats://127.0.0.1:4222]
   ```

## Monitoring and Alerting

Refer [nats monitoring section](https://docs.nats.io/running-a-nats-service/configuration/monitoring) and [monitoring tool by nats-io](https://github.com/nats-io/nats-surveyor)

# CLI Usage

Install CLI

```go
go install github.com/dumbmachine/nq/tools/nq@latest
```

- Cancel task

```bash
$ nq -u nats://127.0.0.1:4222 task cancel --id customID
Cancel message sent task=customID
```

- Status of task

```bash
$ nq -u nats://127.0.0.1:4222 task status --id customID
taskID=customID status=Cancelled
```

- Queue stats

```bash
$ nq -u nats://127.0.0.1:4222 queue stats --name scrap-url-dev
queue: scrap-url-dev | MessagesPending: 11 | Size: 3025 Bytes
```

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
// Creating publish client
package main

import (
	"encoding/json"
	"log"

	"github.com/dumbmachine/nq"
)

type Payload struct {
	Url string `json:"url"`
}

func main() {
	client := nq.NewPublishClient(nq.NatsClientOpt{
		Addr: "nats://127.0.0.1:4222",
	}, nq.NoAuthentcation(),
	// see godoc for more options
	)
	defer client.Close()

	bPayload, err := json.Marshal(Payload{Url: "https://httpstat.us/200?sleep=10000"})
	if err != nil {
		log.Println(err)
	}

	taskSig := nq.NewTask("scrap-url-dev", bPayload)
	if ack, err := client.Enqueue(taskSig); err == nil {
		log.Printf("Submitted queue=%s taskID=%s payload=%s", ack.Queue, ack.ID, ack.Payload)
	} else {
		log.Printf("err=%s", err)
	}
}


```

```go
// creating worker server
package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/dumbmachine/nq"
)

type Payload struct {
	Url string `json:"url"`
}

// Processing function
func fetchHTML(ctx context.Context, task *nq.TaskPayload) error {
	var payload Payload
	if err := json.Unmarshal(task.Payload, &payload); err != nil {
		return errors.New("invalid payload")
	}
	client := &http.Client{}
	req, _ := http.NewRequest("GET", payload.Url, nil)
	req = req.WithContext(ctx)
	if _, err := client.Do(req); err != nil {
		return err
	}
	return nil
}

func main() {

	srv := nq.NewServer(nq.NatsClientOpt{
		Addr:          "nats://127.0.0.1:4222",
		ReconnectWait: time.Second * 2,
		MaxReconnects: 100,
	}, nq.Config{
		ServerName:  nq.GenerateServerName(),
		Concurrency: 1,
		LogLevel:    nq.InfoLevel,
	},
	)

	srv.Register("scrap-url-dev", fetchHTML)

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
