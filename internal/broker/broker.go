package broker

// import (
// 	"context"
// 	"encoding/json"
// 	"fmt"

// 	"github.com/nats-io/nats.go"
// 	"github.com/sirupsen/logrus"
// )

// type NatsBroker struct {
// 	// NATS Jetstream client
// 	natsClient *nats.Conn
// 	js   nats.JetStreamContext
// }

// func (n *NatsBroker) Ping() error {
// 	return nil
// }
// func (n *NatsBroker) Close() error {
// 	n.natsClient.Close()
// 	return nil
// }
// func (n *NatsBroker) Submit(subject string, payload TaskPayload) (*TaskMessage, error) {
// 	return nil, nil
// }
// func (n *NatsBroker) Cancel(subject string, id string) (*TaskMessage, error) {
// 	return nil, nil
// }

// func (n *NatsBroker) Publish(subject string, payload []byte) (*nats.PubAck, error) {
// 	return n.js.Publish(subject, payload)
// }

// // TODO: Furnish this
// func (n *NatsBroker) PublishWithMeta(task *TaskMessage) (*nats.PubAck, error) {
// 	return n.js.Publish(task.Subject, task.Payload)
// }

// func (n *NatsBroker) AddStream(conf nats.StreamConfig) error {
// 	if _, err := n.js.AddStream(&conf); err != nil {
// 		return err
// 	}
// 	return nil
// }
// func (n *NatsBroker) DeleteStream(name string) error {
// 	if err := n.js.DeleteStream(name); err != nil {
// 		logrus.Println(err)
// 	}
// 	return nil
// }

// // NewNatsBroker returns a new instance of NatsBroker.
// func NewNatsBroker(conf NatsClientOpt, opt clientOption) (*NatsBroker, error) {
// 	// nc, conErr := nats.Connect(conf.Addr, opt.natsOption...)
// 	nc, conErr := nats.Connect(conf.Addr)
// 	if conErr != nil {
// 		return nil, conErr
// 	} else {
// 		if js, err := nc.JetStream(); err != nil {
// 			return nil, err
// 		} else {
// 			return &NatsBroker{nc, js}, err
// 		}
// 	}
// }

// func (n *NatsBroker) Subscribe(pattern string, handler ProcessingFunc, contextMap ContextStore) error {
// 	// Create subscription for task handler
// 	if _, err := n.js.Subscribe(pattern, func(m *nats.Msg) {
// 		logrus.Printf("[SUB] Received from %s: %s\n", m.Subject, string(m.Data))
// 		ctx, cancel := context.WithCancel(context.Background())
// 		var tp TaskPayload
// 		if err := json.Unmarshal(m.Data, &tp); err != nil {
// 			logrus.Error("invalid bytes from publisher")
// 		} else {
// 			go func(tpp TaskPayload) {
// 				contextMap.M[tpp.ID] = cancel
// 				if res := handler(ctx, &tpp); res != nil {
// 					// it failed
// 					logrus.Error("[ERROR] %s/%s (%s)", pattern, tpp.ID, res)
// 					defer cancel()
// 				} else {
// 					// it succeded
// 					logrus.Debugf("Processed %s/%s. Result %s", pattern, tpp.ID, res)
// 					defer cancel()
// 				}
// 			}(tp)
// 		}
// 	}); err != nil {
// 		logrus.Fatal("error creating subscriber", err)
// 	}
// 	// Also create cancel subscription
// 	if _, err := n.js.Subscribe(fmt.Sprintf("delete-%s", pattern), func(m *nats.Msg) {
// 		logrus.Printf("[Cancel] Received from %s: %s\n", m.Subject, string(m.Data))
// 		if thisCancel, ok := contextMap.M[string(m.Data)]; ok {
// 			thisCancel()
// 		} else {
// 			logrus.Debug("taskID not found")
// 		}

// 	}); err != nil {
// 		logrus.Fatalf("error creating cancel-subscriber for %s", pattern)
// 	}
// 	return nil
// }
