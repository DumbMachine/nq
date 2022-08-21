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
)

type UrlPayload struct {
	Url string `json:"url"`
}

func fetchHTML(ctx context.Context, task *nq.TaskPayload) error {
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

const (
	QueueDev  = "scrap-url-dev"
	QueueProd = "scrap-url-prod"
)

func main() {

	flag.Parse()
	args := flag.Args()

	if len(args) != 1 {
		log.Println("only one single subject command line arg should be provided", args)
	}
	appType := args[0]

	switch appType {
	case "pub":
		{
			// TODO: Defer closing of publish client
			log.Println("Startup publisher ...")
			client := nq.NewPublishClient(nq.NatsClientOpt{
				Addr: "nats://127.0.0.1:4222",
			}, nq.NoAuthentcation(),
			// see godoc for more options
			)

			defer client.Close()

			bytesPayload1, err := json.Marshal(UrlPayload{Url: "https://httpstat.us/200?sleep=10000"})
			if err != nil {
				log.Println(err)
			}

			task1 := nq.NewTask(
				// QueueDev, bytesPayload1, nq.TaskID("customID"), nq.Retry(2))
				// QueueDev, bytesPayload1)
				// QueueDev, bytesPayload1, nq.Deadline(time.Now().Add(time.Second)), nq.TaskID("customID"))
				QueueDev, bytesPayload1, nq.Timeout(time.Minute*10), nq.TaskID("customID"))
			if ack, err := client.Enqueue(task1); err == nil {
				log.Printf("Submitted queue=%s taskID=%s payload=%s", ack.Queue, ack.ID, ack.Payload)
			} else {
				log.Printf("err=%s", err)
			}

			// Cleanup
			// defer func() {
			// 	client.DeleteQueue(QueueDev)
			// 	defer client.Close()
			// }()

		}
	case "sub":
		{
			log.Println("Startup subscriber ...")
			srv := nq.NewServer(nq.NatsClientOpt{
				Addr:          "nats://127.0.0.1:4222",
				ReconnectWait: time.Second * 2,
				MaxReconnects: 100,
			}, nq.Config{
				// ServerName: "local",
				ServerName:  nq.GenerateServerName(),
				Concurrency: 1,
				LogLevel:    nq.InfoLevel,
			},
			// nq.NoAuthentcation(), nq.ShutdownOnNatsDisconnect(),
			)

			srv.Register(QueueDev, fetchHTML)

			if err := srv.Run(); err != nil {
				panic(err)
			}
		}
	}

}
