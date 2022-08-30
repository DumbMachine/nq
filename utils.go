package nq

import (
	"encoding/json"
	"fmt"
	"strings"
)

//
// TaskMessage Encode/Decode utilities
//

func EncodeTMToJSON(t *TaskMessage) ([]byte, error) {
	if b, err := json.Marshal(t); err != nil {
		return nil, err
	} else {
		return b, nil
	}
}

func DecodeTMFromJSON(data []byte) (*TaskMessage, error) {
	var t TaskMessage
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	} else {
		return &t, nil
	}
}

//
// Stream / Subject Name utilities
//

// Returns a durable name for stream
//
// Helps re-establishing connection to nats-server while maintaining sequence state
func StreamNameToDurableStreamName(srvName, stream string) string {
	return fmt.Sprintf("%s-%s", srvName, stream)
}

// streamNameToCancelStreamName returns the name of stream responsible for cancellation of tasks in given stream
func StreamNameToCancelStreamName(subject string) string {
	return fmt.Sprintf("cancel-%s", subject)
}

func CancelStreamNameToStreamName(stream, subject string) string {
	return strings.Replace(subject, "cancel-", "", 1)
}

//
//
// Internal `Queue`s represent an abstraction over a nats stream -> subject
type Queue struct {
	stream        string
	subject       string
	cancelStream  string
	cancelSubject string
}

func NewQueue(name string) *Queue {
	return &Queue{
		stream:        name,
		subject:       fmt.Sprintf("%s.task", name),
		cancelStream:  fmt.Sprintf("%s/cancel", name),
		cancelSubject: fmt.Sprintf("%s.cancel", name),
	}
}

func (q *Queue) DurableStream(prefix string) string {
	return fmt.Sprintf("%s/%s", prefix, q.stream)
}
