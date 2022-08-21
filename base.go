package nq

import (
	"time"

	"github.com/nats-io/nats.go"
)

// NatsClientOpt represent NATS connection configuration option.
type NatsClientOpt struct {
	// nats server address
	Addr string

	// Name for key-value store used to store task metadata
	//
	// Defaults to nq
	DBName string

	// ReconnectWait is an Option to set the wait time between reconnect attempts.
	//
	// Defaults to 10 seconds
	ReconnectWait time.Duration

	// MaxReconnects is an Option to set the maximum number of reconnect attempts.
	//
	// Defaults to 100
	MaxReconnects int
}

type CancelPayload string

type TaskPayload struct {
	ID      string
	Payload []byte
}

// Possible task statuses
const (
	// waiting for task to be recieved by worker
	Pending = iota

	// task is being processed by a worker
	Processing

	// taskFN returns an error
	Failed

	// successfully processed
	Completed

	// cancelled by user
	Cancelled

	// deleted before being run
	Deleted
)

type TaskMessage struct {

	// Sequence indicates sequence number of message in nats jetstream
	Sequence uint64

	// ID is a unique identifier for each task, used for cancellation.
	ID string

	// . Autofilled
	StreamName string

	//
	Queue string

	// Payload holds data needed to process the task.
	Payload []byte

	// Status indicated status of task execution
	Status int

	// Timeout specifies timeout in seconds.
	// Use zero to indicate no deadline.
	Timeout int64

	// Deadline specifies the deadline for the task in Unix time.
	// Use zero to indicate no deadline.
	Deadline int64

	// CompletedAt is the time the task was processed successfully in Unix time,
	// the number of seconds elapsed since January 1, 1970 UTC.
	//
	// Negative value indicated cancelled.
	// Use zero to indicate no value.
	CompletedAt int64
	// PubAck is an ack received after successfully publishing a message.
	// NatsAck nats.PubAck

	// Current
	CurrentRetry int

	// Total number of retries possible for this task
	MaxRetry int

	// Function to acknowledge this TaskMessage when recived as a subscription
	ackFN func(opts ...nats.AckOpt) error
}

func (msg *TaskMessage) GetStatus() string {
	switch msg.Status {
	case Pending:
		return "Pending"
	case Processing:
		return "Processing"
	case Failed:
		return "Failed"
	case Completed:
		return "Completed"
	case Cancelled:
		return "Cancelled"
	case Deleted:
		return "Deleted"
	default:
		return "invalid state"
	}
}

type TaskCancellationMessage struct {
	// ID corresponds to task's ID
	ID string

	// StreamName is the name of stream whose subject is handled by this task
	StreamName string
}

type ClientOptionType int

// Internal representation of options for nats-server connection
type ClientOption struct {
	Timeout              time.Duration //todo
	AuthenticationType   ClientOptionType
	AuthenticationObject interface{}
	NatsOption           []nats.Option
	// Defaults to false
	ShutdownOnNatsDisconnect bool
}
