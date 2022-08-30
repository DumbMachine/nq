package nq

import (
	"context"
	"encoding/json"
	"log"
)

// Signature for function executed by a worker.
// `ProcessingFunc` type are be registered to subjects, process messages published by client
type ProcessingFunc func(context.Context, *TaskPayload) error

type ListenUpdates struct {
	m map[string]chan string
}

type Inspector struct {
	broker    *NatsBroker
	listeners *ListenUpdates
}

func NewInspector(broker *NatsBroker) *Inspector {
	return &Inspector{
		broker:    broker,
		listeners: &ListenUpdates{make(map[string]chan string)},
	}
}

func (i *Inspector) AddAnother(queue string, sendUpdatesTo chan string) error {
	return nil
}
func (i *Inspector) Servers() {}
func (i *Inspector) Queues()  {}

// Client responsible for interaction with nq tasks
//
// Client is used to enqueue / cancel tasks or fetch metadata for tasks
type PublishClient struct {
	broker *NatsBroker
	kv     ResultHandlerIFACE
}

// NewPublishClient returns a new Client instance, given nats connection options, to interact with nq tasks
//
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
		config.DBName = defaultKVName
	}

	kv := NewResultHandlerNats(config.DBName, broker.js)
	if err != nil {
		panic(err)
	}

	return &PublishClient{broker: broker, kv: kv}
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

func (p *PublishClient) publishToSubject(task *Task, opts ...TaskOption) (*TaskMessage, error) {
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

// GetUpdates can be used get changes to a task's metadata
//
// Returns error if failed to start watching for changes
// Channel is closed, once task reaches terminal state
func (p *PublishClient) GetUpdates(taskID string) (chan *TaskMessage, error) {
	if status, err := p.kv.Watch(taskID); err != nil {
		return nil, err
	} else {
		return status, err
	}
}

// Enqueue can be used to enqueu given task to a queue
//
// Returns TaskMessage and nil error is enqueued successfully, else non-nill error
func (p *PublishClient) Enqueue(task *Task, opts ...TaskOption) (*TaskMessage, error) {
	q := NewQueue(task.queue)
	p.broker.ConnectoQueue(q)

	return p.publishToSubject(task, opts...)
}

// Cancel sends `cancel` request for given task to workers
func (p *PublishClient) Cancel(id string) error {
	if taskInfo, err := p.kv.Get(id); err != nil {
		return ErrTaskNotFound
	} else {
		if taskInfo.Status == Deleted || taskInfo.Status == Cancelled || taskInfo.Status == Completed || taskInfo.Status == Failed {
			return ErrNonCancellableState
		}
		if taskInfo.Status == Pending {
			// task is still pending, safe to remove from list
			q := NewQueue(taskInfo.Queue)
			if err := p.broker.js.DeleteMsg(q.stream, taskInfo.Sequence); err != nil {
				// fmt.Printf("Cancellation failed for %s/%s", taskInfo.StreamName, taskInfo.ID)
				return err
			} else {
				// fmt.Printf("Successfully cancelled: %s/%s", taskInfo.Queue, taskInfo.ID)
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

// Delete a queue
//
// Deletes underlying nats stream assosociated with a queue
func (p *PublishClient) DeleteQueue(qname string) {
	q := NewQueue(qname)
	// delete task stream
	if err := p.broker.DeleteStream(q.stream); err != nil {
		log.Printf("error deleting stream=%s", q.stream)
	}
	// delete cancellation stream
	if err := p.broker.DeleteStream(q.cancelStream); err != nil {
		log.Printf("error deleting stream=%s", q.stream)
	}
}

// Fetch fetches TaskMessage for given task
//
func (p *PublishClient) Fetch(id string) (*TaskMessage, error) {
	return p.kv.Get(id)
}

// Close closes the connection with nats
func (p *PublishClient) Close() error {
	defer p.broker.Close()
	return nil
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
