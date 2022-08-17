// Copyright 2022 Ratin Kumar. All rights reserved.
// Use of this source code is governed by a MIT license
// that can be found in the LICENSE file.

// nq provides a go package to publish/process tasks via nats
package nq

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Task struct {
	// Stream subject
	queue string
	// Payload for task
	payload []byte
	opts    []TaskOption
	// Result writer for retention
}

// !TODO: Remove this later
// func (p *PublishClient) Subscribe(pattern string, handler ProcessingFunc, contextMap ContextStore) error {
// 	// return p.broker.Subscribe(pattern, handler, contextMap)
// }

// Value zero indicates no timeout and no deadline.
var (
	noTimeout  time.Duration = 0
	noDeadline time.Time     = time.Unix(0, 0)
)

type TaskOptionType int

const (
	MaxRetryOpt TaskOptionType = iota
	TaskIDOpt
	// QueueOpt
	TimeoutOpt
	DeadlineOpt
	// ProcessAtOpt
	// ProcessInOpt
)

type TaskOption interface {
	String() string
	Type() TaskOptionType
	Value() interface{}
}

// Internal option representations.
type (
	retryOption    int
	taskIDOption   string
	timeoutOption  time.Duration
	deadlineOption time.Time
)

func Retry(n int) TaskOption {
	return retryOption(n)
}
func (n retryOption) String() string       { return fmt.Sprintf("MaxRetry(%d)", int(n)) }
func (n retryOption) Type() TaskOptionType { return MaxRetryOpt }
func (n retryOption) Value() interface{}   { return int(n) }

// TaskID returns an option to specify the task ID.
func TaskID(id string) TaskOption {
	return taskIDOption(id)
}

func (id taskIDOption) String() string       { return fmt.Sprintf("TaskID(%q)", string(id)) }
func (id taskIDOption) Type() TaskOptionType { return TaskIDOpt }
func (id taskIDOption) Value() interface{}   { return string(id) }

// Timeout returns an option to specify how long a task may run.
//
// Zero duration means no limit ( math.MaxInt64 is chosen )
//
// If there's a conflicting Deadline option, whichever comes earliest
// will be used.
func Timeout(d time.Duration) TaskOption {
	return timeoutOption(d)
}

func (d timeoutOption) String() string       { return fmt.Sprintf("Timeout(%v)", time.Duration(d)) }
func (d timeoutOption) Type() TaskOptionType { return TimeoutOpt }
func (d timeoutOption) Value() interface{}   { return time.Duration(d) }

// Deadline returns an option to specify the deadline for the given task.
// If it reaches the deadline before the Handler returns, then the task
// will be retried.
//
// If there's a conflicting Timeout option, whichever comes earliest
// will be used.
func Deadline(t time.Time) TaskOption {
	return deadlineOption(t)
}

func (t deadlineOption) String() string {
	return fmt.Sprintf("Deadline(%v)", time.Time(t).Format(time.UnixDate))
}
func (t deadlineOption) Type() TaskOptionType { return DeadlineOpt }
func (t deadlineOption) Value() interface{}   { return time.Time(t) }

type option struct {
	retry    int
	taskID   string
	timeout  time.Duration
	deadline time.Time
}

// Composes options for a task, merging default and user-provided options
func withDefaultOptions(opts ...TaskOption) (option, error) {
	res := option{
		timeout:  0,
		retry:    0, // implies no futher retry, once executed
		deadline: time.Time{},
		taskID:   uuid.NewString(),
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
		default:
			// unexpected option
		}
	}
	return res, nil
}

// payload is jsonified data of whatever the ProcessingFunc expects
func NewTask(queue string, payload []byte, opts ...TaskOption) *Task {
	return &Task{
		queue:   queue,
		payload: payload,
		opts:    opts,
	}

}
