package main

import (
	"flag"
	"log"

	"github.com/dumbmachine/nq"
)

/*
url: url of thing
cancel: taskID
queuesize: list queue size for all
status:
*/

func cancelCommand(url string, id string, queue string) error {
	client := nq.NewPublishClient(nq.NatsClientOpt{Addr: url}, nq.NoAuthentcation())

	if queue != "" {
		return client.CancelInQueue(id, queue)
	} else {
		return client.Cancel(id)
	}
}

func statusCommand(url string, id string) error {
	client := nq.NewPublishClient(nq.NatsClientOpt{Addr: url}, nq.NoAuthentcation())
	if msg, err := client.Fetch(id); err != nil {
		return err
	} else {
		log.Printf("taskID=%s status=%s", msg.ID, msg.GetStatus())
		return nil
	}
}

func main() {
	// flag.Parse()
	// args := flag.Args()

	// if len(args) != 1 {
	// 	panic("not enough options")
	// }

	natsURL := flag.String("url", "", "Text to parse.")
	cancelID := flag.String("cancel", "", "")
	statusID := flag.String("status", "", "")
	flag.Parse()
	if *natsURL == "" {
		panic("url cannot be empty")
	}

	if *cancelID != "" {
		cancelCommand(*natsURL, *cancelID, "")
	}
	if *statusID != "" {
		statusCommand(*natsURL, *statusID)
	}
	// // client := nq.NewPublishClient(nq.NatsClientOpt{Addr: url}, nq.NoAuthentcation())

	// // client.
	// if nc, conErr := nats.Connect(*natsURL); conErr != nil {
	// 	panic(conErr)
	// } else {

	// 	js, _ := nc.JetStream()

	// 	for name := range js.StreamNames() {
	// 		if strings.HasPrefix(name, "stream/") && !strings.HasPrefix(name, "stream/cancel") {
	// 			info, _ := js.StreamInfo(name)
	// 			var consumers []string
	// 			for na := range js.ConsumerNames(name) {
	// 				consumers = append(consumers, na)
	// 			}
	// 			// println("name: ", name, "  consumers: ", info.State.Consumers, "  msgs: ", info.State.Msgs)
	// 			fmt.Printf("queue: %s | Consumers: %s  ( %s ) | Msgs: %s \n", name, fmt.Sprintf("%d", info.State.Consumers), strings.Join(consumers, ","), fmt.Sprintf("%d", info.State.Msgs))
	// 			println(info.State.Bytes)
	// 		}
	// 	}
	// }
}
