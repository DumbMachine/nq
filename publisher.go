package nq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/nats-io/nats.go"
)

// Signature for function executed by a worker.
// `ProcessingFunc` type are be registered to subjects, process messages published by client
type ProcessingFunc func(context.Context, *TaskPayload) error

type PublishClient struct {
	broker *NatsBroker
	kv     ResultHandlerIFACE
}

func NewPublishClient(config NatsClientOpt, opts ...ClientConnectionOption) *PublishClient {
	opt, err := withDefaultClientOptions(opts...)
	if err != nil {
		panic(err)
	}

	broker, err := NewNatsBroker(config, opt, make(chan struct{}), make(chan struct{}))
	if err != nil {
		panic(err)
	}

	if config.DBName == "" {
		config.DBName = KVName
	}

	kv := NewResultHandlerNats(config.DBName, broker.js)
	if err != nil {
		panic(err)
	}

	return &PublishClient{broker: broker, kv: kv}
}

type PackagePubAck struct {
	// ID assigned to published message
	ID string
	*nats.PubAck
}

func (p *PublishClient) Stats(queue string) error {
	return p.broker.Stats(NewQueue(queue))
}

// Publishes task message to a queue
func (p *PublishClient) publishMessage(msg *TaskMessage) (*TaskMessage, error) {
	q := NewQueue(msg.Queue)
	bytesMsg, err := EncodeTMToJSON(msg)
	if err != nil {
		return nil, err
	}
	if pubAck, err := p.broker.Publish(q.subject, bytesMsg); err != nil {
		return nil, err
	} else {
		// updating sequence info, required for cancelling a task
		msg.Sequence = pubAck.Sequence
		msgBytes, _ := EncodeTMToJSON(msg)
		p.kv.Set(msg.ID, msgBytes)
		return msg, nil
	}
}

// Publish a TaskMessage into a stream
func (p *PublishClient) PublishToSubject(task *Task, opts ...TaskOption) (*TaskMessage, error) {
	opts = append(task.opts, opts...)
	opt, err := withDefaultOptions(opts...)
	if err != nil {
		return nil, err
	}
	deadline := noDeadline
	if !opt.deadline.IsZero() {
		deadline = opt.deadline
	}
	timeout := noTimeout
	if opt.timeout != 0 {
		timeout = opt.timeout
	}
	taskMessage := &TaskMessage{
		Sequence:     0, // default value
		ID:           opt.taskID,
		Queue:        task.queue,
		Payload:      task.payload,
		Deadline:     deadline.Unix(),
		CurrentRetry: 0,
		MaxRetry:     opt.retry,
		Timeout:      int64(timeout.Seconds()),
		Status:       Pending,
		CompletedAt:  0,
	}
	return p.publishMessage(taskMessage)
}

func (p *PublishClient) Enqueue(task *Task, opts ...TaskOption) (*TaskMessage, error) {
	q := NewQueue(task.queue)
	p.connectOrCreateQueue(q)

	return p.PublishToSubject(task, opts...)
}

// Fetch qname from kv store instead
func (p *PublishClient) Cancel(id string) error {
	if taskInfo, err := p.kv.Get(id); err != nil {
		return ErrTaskNotFound
	} else {
		if taskInfo.Status == Deleted {
			return ErrCannotCancelDeletedTask
		}
		if taskInfo.Status == Pending {
			// task is still pending, safe to remove from list
			q := NewQueue(taskInfo.Queue)
			if err := p.broker.js.DeleteMsg(q.stream, taskInfo.Sequence); err != nil {
				// fmt.Printf("Cancellation failed for %s/%s", taskInfo.StreamName, taskInfo.ID)
				return err
			} else {
				// fmt.Printf("Successfuly cancelled: %s/%s", taskInfo.Queue, taskInfo.ID)
				taskInfo.Status = Deleted
				x, _ := EncodeTMToJSON(taskInfo)
				p.kv.Set(taskInfo.ID, x)
			}
			return nil
		}

		// if not in pending state, multicast cancellation request to all workers
		q := NewQueue(taskInfo.Queue)
		return p.cancelInStream(id, q)
	}
}

// Faster than using `Cancel` method, if queue name is known
func (p *PublishClient) CancelInQueue(id string, qname string) error {
	q := NewQueue(qname)
	return p.cancelInStream(id, q)
}

// Connect to a queue
//
//
// func (p *PublishClient) Queue(name string) error {
// 	q := Queue{name: name}
// 	return p.connectOrCreateQueue(q)
// }

// TODO: Check first before creating
// TODO: Change naming schema to
// queue.task for task message
// queue.cancel for cancel message

func (p *PublishClient) connectOrCreateQueue(q *Queue) error {

	// create task-stream
	if err := p.createStream(q.stream, q.subject, nats.WorkQueuePolicy); err != nil {
		// failed to create task-stream
		// todo
		panic(err)
	} else {
		log.Printf("Created queue=%s", q.stream)
		// create cancel stream for task-stream
		if err := p.createStream(q.cancelStream, q.cancelSubject, nats.InterestPolicy); err != nil {
			panic(err)
		}
		log.Printf("Created cancel queue=%s", q.stream)

	}
	return nil
}

func (p *PublishClient) createStream(streamName, subject string, policy nats.RetentionPolicy) error {
	if err := p.broker.AddStream(nats.StreamConfig{
		Name:      streamName,
		Subjects:  []string{subject},
		Retention: policy,
	}); err != nil {
		return err
	}
	return nil
}

// Cleanup method
func (p *PublishClient) DeleteQueue(qname string) {
	q := NewQueue(qname)
	if err := p.broker.DeleteStream(q.stream); err != nil {
		log.Println(fmt.Sprintf("error deleting stream: %s"))
	}
}

func (p *PublishClient) cancelInStream(id string, q *Queue) error {

	payload := TaskCancellationMessage{
		StreamName: q.cancelSubject,
		ID:         id,
	}

	if pb, err := json.Marshal(payload); err != nil {
		return err
	} else {
		if _, err := p.broker.Publish(q.cancelSubject, pb); err != nil {
			return err
		}
		return nil
	}
}

func (p *PublishClient) Fetch(id string) (*TaskMessage, error) {
	return p.kv.Get(id)
}

// Also delete stream for cleanup
func (p *PublishClient) Close() error {
	// cleanup: close stream and delete-stream
	defer p.broker.Close()
	return nil
}
