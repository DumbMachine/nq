// nq provides a go package to publish/process tasks via nats
package nq

import (
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nuid"
)

// Task is a representation work to be performed by a worker
type Task struct {
	// Stream subject
	queue string
	// Payload for task
	payload []byte
	// Options to configure task processing behavior
	opts []TaskOption
}

// Value zero indicates no timeout and no deadline.
var (
	noTimeout   time.Duration = 0
	noDeadline  time.Time     = time.Unix(0, 0)
	noProcessAt time.Time     = time.Unix(0, 0)
)

type TaskOptionType int

const (
	MaxRetryOpt TaskOptionType = iota
	TaskIDOpt
	// QueueOpt
	TimeoutOpt
	DeadlineOpt
	ProcessAtOpt
	// ProcessInOpt
)

type TaskOption interface {
	String() string
	Type() TaskOptionType
	Value() interface{}
}

type (
	retryOption     int
	taskIDOption    string
	timeoutOption   time.Duration
	deadlineOption  time.Time
	processAtOption time.Time
)

// Returns an options to specify maximum number of times a task will be retried before being marked as failed.
//
// -ve retry count is assigned defaultRetry ( 0 )
func Retry(n int) TaskOption {
	if n < 0 {
		return retryOption(defaultMaxRetry)
	}
	return retryOption(n)
}
func (n retryOption) String() string       { return fmt.Sprintf("MaxRetry(%d)", int(n)) }
func (n retryOption) Type() TaskOptionType { return MaxRetryOpt }
func (n retryOption) Value() interface{}   { return int(n) }

// TaskID returns an option to specify the task ID
func TaskID(id string) TaskOption {
	return taskIDOption(id)
}

func (id taskIDOption) String() string       { return fmt.Sprintf("TaskID(%q)", string(id)) }
func (id taskIDOption) Type() TaskOptionType { return TaskIDOpt }
func (id taskIDOption) Value() interface{}   { return string(id) }

// Timeout returns an option to specify how long a task can run before being cancelled.
//
// Zero duration means no limit ( math.MaxInt32 )
//
// If both Deadline and Timeout options are set, whichever comes earliest
// will be used.
func Timeout(d time.Duration) TaskOption {
	return timeoutOption(d)
}

func (d timeoutOption) String() string       { return fmt.Sprintf("Timeout(%v)", time.Duration(d)) }
func (d timeoutOption) Type() TaskOptionType { return TimeoutOpt }
func (d timeoutOption) Value() interface{}   { return time.Duration(d) }

// Deadline returns an option to specify the deadline for the given task.
//
// If both Deadline and Timeout options are set, whichever comes earliest
// will be used.
func Deadline(t time.Time) TaskOption {
	return deadlineOption(t)
}

func (t deadlineOption) String() string {
	return fmt.Sprintf("Deadline(%v)", time.Time(t).Format(time.UnixDate))
}
func (t deadlineOption) Type() TaskOptionType { return DeadlineOpt }
func (t deadlineOption) Value() interface{}   { return time.Time(t) }

// ProcessAt returns an option to specify the date/time the task should be run at
//
// This is done by allowing the published task to be handled by any instance
// at which point a delay is calculated and nack the msg with delay allowing nats
// to redeliver it again after the duration has elapsed, if ProcessAt is not set
// the task will be handled as usual
func ProcessAt(t time.Time) TaskOption {
	return processAtOption(t)
}

func (t processAtOption) String() string {
	return fmt.Sprintf("ProcessAt(%v)", time.Time(t).Format(time.UnixDate))
}
func (t processAtOption) Type() TaskOptionType { return ProcessAtOpt }
func (t processAtOption) Value() interface{}   { return time.Time(t) }

type option struct {
	retry     int
	taskID    string
	timeout   time.Duration
	deadline  time.Time
	processAt time.Time
}

// Composes options for a task, merging default and user-provided options
func withDefaultOptions(opts ...TaskOption) (option, error) {
	res := option{
		timeout:  0,
		retry:    defaultMaxRetry,
		deadline: time.Time{},
		// TODO: store generator per server
		taskID: nuid.New().Next(),
	}
	for _, opt := range opts {
		switch opt := opt.(type) {
		case timeoutOption:
			res.timeout = time.Duration(opt)
		case taskIDOption:
			{
				id := string(opt)
				if strings.TrimSpace(id) == "" {
					return option{}, ErrTaskIDEmpty
				}
				res.taskID = id
			}
		case deadlineOption:
			res.deadline = time.Time(opt)
		case retryOption:
			res.retry = int(opt)
		case processAtOption:
			res.processAt = time.Time(opt)
		default:
			// unexpected option
		}
	}
	return res, nil
}

// NewTask returns a new Task given queue and byte payload
//
// TaskOption can be used to configure task processing
func NewTask(queue string, payload []byte, opts ...TaskOption) *Task {
	return &Task{
		queue:   queue,
		payload: payload,
		opts:    opts,
	}

}
