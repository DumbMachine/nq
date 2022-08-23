package nq

import (
	"fmt"
	"log"

	"github.com/nats-io/nats.go"
)

type NatsBroker struct {
	ns *nats.Conn
	js nats.JetStreamContext
}

func (n *NatsBroker) Ping() error {
	return nil
}
func (n *NatsBroker) Close() error {
	n.ns.Close()
	return nil
}
func (n *NatsBroker) Submit(subject string, payload TaskPayload) (*TaskMessage, error) {
	return nil, nil
}
func (n *NatsBroker) Cancel(subject string, id string) (*TaskMessage, error) {
	return nil, nil
}

func (n *NatsBroker) Publish(subject string, payload []byte) (*nats.PubAck, error) {
	return n.js.Publish(subject, payload)
}

// Checks is a stream exists
func (n *NatsBroker) isStreamExists(stream string) bool {
	_, err := n.js.StreamInfo(stream)
	return err == nil
}

func (n *NatsBroker) PublishWithMeta(msg *TaskMessage) (*TaskMessage, error) {
	bytesMsg, err := EncodeTMToJSON(msg)
	if err != nil {
		return nil, err
	}
	if pubAck, err := n.Publish(msg.Queue, bytesMsg); err != nil {
		return nil, err
	} else {
		// updating sequence info, required for cancelling a task
		msg.Sequence = pubAck.Sequence
		return msg, nil
	}
}

func (n *NatsBroker) AddStream(conf nats.StreamConfig) error {
	if _, err := n.js.AddStream(&conf); err != nil {
		return err
	}
	return nil
}
func (n *NatsBroker) DeleteStream(name string) error {
	if err := n.js.DeleteStream(name); err != nil {
		return err
	}
	return nil
}

func natsDisconnectHandler(disconnect bool, natsConnectionClosed chan struct{}) nats.Option {
	if disconnect {
		return nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			// Send closing signal
			// debug
			// fmt.Println("activating closed connection")
			natsConnectionClosed <- struct{}{}
		})
	} else {
		return nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			// TODO: user logger
			// log.Debug("disconnected from nats")
			fmt.Println("disconnected from nats")
		})
	}
}

func natsClosedHandler(disconnect bool, natsConnectionClosed chan struct{}) nats.Option {
	if disconnect {
		return nats.ClosedHandler(func(nc *nats.Conn) {
			// TODO: use a logger instead
			println("Nats Client Connection closed!", "Reason", nc.LastError())
			// Send closing signal
			natsConnectionClosed <- struct{}{}
		})
	}
	return nil
}

// Utility that creates a nats jetstream
func (n *NatsBroker) createStream(streamName, subject string, policy nats.RetentionPolicy) error {
	if err := n.AddStream(nats.StreamConfig{
		Name:      streamName,
		Subjects:  []string{subject},
		Retention: policy,
	}); err != nil {
		return err
	}
	return nil
}

// Temporary function that fulfill statistic demands from nq-cli
func (n *NatsBroker) Stats(q *Queue) error {
	jinfo, err := n.js.StreamInfo(q.stream)
	if err != nil {
		return err
	}
	fmt.Printf("queue: %s | MessagesPending: %d | Size: %d Bytes \n", q.stream, jinfo.State.Msgs, jinfo.State.Bytes)
	return nil
}

// Creates queue stream if not exists
//
// Also create underlying nets-stream for queue and cancel-queue
func (n *NatsBroker) ConnectoQueue(q *Queue) error {
	// create task-stream
	if ok := n.isStreamExists(q.stream); !ok {
		// if stream does not exist, create
		if err := n.createStream(q.stream, q.subject, nats.WorkQueuePolicy); err != nil {
			// failed to create task-stream
			// todo
			panic(err)
		}
		log.Printf("Created queue=%s", q.stream)
	}
	if ok := n.isStreamExists(q.cancelStream); !ok {
		// create cancel stream for task-stream
		if err := n.createStream(q.cancelStream, q.cancelSubject, nats.InterestPolicy); err != nil {
			panic(err)
		}
		log.Printf("Created cancel queue=%s", q.stream)
	}
	return nil
}

// TODO: Allow users to specify `forceReRegister` as a boolean
// NewNatsBroker returns a new instance of NatsBroker.
func NewNatsBroker(conf NatsClientOpt, opt ClientOption, natsConnectionClosed chan struct{}, forceReRegister chan struct{}) (*NatsBroker, error) {
	opt.NatsOption = append(opt.NatsOption,
		nats.ReconnectWait(conf.ReconnectWait), nats.MaxReconnects(conf.MaxReconnects),
		// nats.ReconnectWait(time.Second), nats.MaxReconnects(100),
		natsDisconnectHandler(opt.ShutdownOnNatsDisconnect, natsConnectionClosed),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			// TODO: Use a logger here
			if !opt.ShutdownOnNatsDisconnect {
				log.Println("reconnection found", nc.ConnectedUrl())
				forceReRegister <- struct{}{}
			}
		}),
		natsClosedHandler(opt.ShutdownOnNatsDisconnect, natsConnectionClosed),
	)

	nc, conErr := nats.Connect(conf.Addr,
		opt.NatsOption...,
	)
	if conErr != nil {
		return nil, conErr
	} else {
		if js, err := nc.JetStream(); err != nil {
			return nil, err
		} else {
			return &NatsBroker{nc, js}, err
		}
	}
}
