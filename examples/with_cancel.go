/*
Cancelling nq task
*/

package main

// import (
// 	"context"
// 	"encoding/json"
// 	"errors"
// 	"flag"
// 	"log"
// 	"time"

// 	"github.com/dumbmachine/nq"
// )

// type NumberParameters struct {
// 	N1 int `json:"n1"`
// 	N2 int `json:"n2"`
// }

// // Slowly add 2 integers, by sleeping `n2` times
// //
// // An example of task that can take long time to process
// func cancellableAdder(ctx context.Context, task *nq.TaskPayload) error {
// 	var payload NumberParameters
// 	if err := json.Unmarshal(task.Payload, &payload); err != nil {
// 		return errors.New("invalid payload")
// 	}

// 	ret := payload.N1

// 	for i := 0; i < payload.N2; i++ {
// 		select {
// 		case <-ctx.Done():
// 			return ctx.Err()
// 		default:
// 			{
// 				ret += 1
// 				time.Sleep(time.Second)
// 			}
// 		}
// 	}
// 	return nil
// }

// const QName = "cancellable-summer"

// func main() {

// 	flag.Parse()
// 	args := flag.Args()

// 	if len(args) != 1 {
// 		log.Println("only one single subject command line arg should be provided", args)
// 	}
// 	appType := args[0]

// 	switch appType {
// 	case "pub":
// 		{
// 			// TODO: Defer closing of publish client
// 			log.Println("Startup publisher ...")
// 			client := nq.NewPublishClient(nq.NatsClientOpt{
// 				Addr: "nats://127.0.0.1:4222",
// 			}, nq.NoAuthentcation(),
// 			// see godoc for more options
// 			)

// 			defer client.Close()

// 			bytesPayload1, err := json.Marshal(NumberParameters{N1: 10, N2: 5})
// 			if err != nil {
// 				log.Println(err)
// 			}

// 			task1 := nq.NewTask(QName, bytesPayload1)
// 			if ack, err := client.Enqueue(task1); err == nil {
// 				log.Printf("Submitted queue=%s taskID=%s payload=%s", ack.Queue, ack.ID, ack.Payload)
// 				// Sleep for 2 seconds before cancelling message
// 				time.Sleep(time.Second * 2)
// 				client.Cancel(ack.ID)
// 				client.CancelInQueue(ack.ID, ack.Queue)
// 				// if cancel message was sent succesfully, it's status should be cancelled
// 				msg, _ := client.Fetch(ack.ID)
// 				log.Printf("task=%s status=%s", ack.ID, msg.GetStatus())
// 			} else {
// 				log.Printf("err=%s", err)
// 			}

// 		}
// 	case "sub":
// 		{
// 			srv := nq.NewServer(nq.NatsClientOpt{
// 				Addr:          "nats://127.0.0.1:4222",
// 				ReconnectWait: time.Second * 5,
// 				MaxReconnects: 100,
// 			}, nq.Config{
// 				ServerName:  nq.GenerateServerName(),
// 				Concurrency: 1,
// 				LogLevel:    nq.InfoLevel,
// 			})
// 			srv.Register(QName, cancellableAdder)
// 			if err := srv.Run(); err != nil {
// 				panic(err)
// 			}
// 		}
// 	}

// }
