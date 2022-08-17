package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"time"

	"github.com/hibiken/asynq"
	"github.com/sirupsen/logrus"
)

// Task payload for any email related tasks.
type EmailTaskPayload struct {
	// ID for the email recipient.
	UserID int
}

func sendWelcomeEmail(ctx context.Context, t *asynq.Task) error {
	var p EmailTaskPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return err
	}
	log.Printf(" [*] Send Welcome Email to User %d", p.UserID)
	return nil
}

func sendReminderEmail(ctx context.Context, t *asynq.Task) error {
	var p EmailTaskPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return err
	}
	log.Printf(" [*] Send Reminder Email to User %d", p.UserID)
	return nil
}

func main() {
	// logrus.SetReportCaller(true)

	flag.Parse()
	args := flag.Args()

	if len(args) != 1 {
		logrus.Fatal("only one single subject command line arg should be provided", args)
	}
	appType := args[0]

	switch appType {
	case "pub":
		{
			logrus.Info("Startup publisher ...")
			client := asynq.NewClient(asynq.RedisClientOpt{Addr: "localhost:6379"})

			// Create a task with typename and payload.
			payload, err := json.Marshal(EmailTaskPayload{UserID: 42})
			if err != nil {
				log.Fatal(err)
			}
			t1 := asynq.NewTask("email:welcome", payload)
			t2 := asynq.NewTask("email:reminder", payload)

			// Process the task immediately.
			info, err := client.Enqueue(t1)
			if err != nil {
				log.Fatal(err)
			}
			log.Printf(" [*] Successfully enqueued task: %+v", info)

			// Process the task 24 hours later.
			info, err = client.Enqueue(t2, asynq.ProcessIn(24*time.Hour))
			if err != nil {
				log.Fatal(err)
			}
			log.Printf(" [*] Successfully enqueued task: %+v", info)

		}
	case "sub":
		{
			logrus.Info("Startup subscriber ...")
			srv := asynq.NewServer(
				asynq.RedisClientOpt{Addr: "localhost:6379"},
				asynq.Config{Concurrency: 10},
			)

			mux := asynq.NewServeMux()
			mux.HandleFunc("email:welcome", sendWelcomeEmail)
			mux.HandleFunc("email:reminder", sendReminderEmail)

			if err := srv.Run(mux); err != nil {
				log.Fatal(err)
			}

		}
	}

}
