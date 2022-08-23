package nq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	iContext "github.com/dumbmachine/nq/internal/context"
	ilog "github.com/dumbmachine/nq/internal/log"

	"github.com/nats-io/nats.go"
)

type CancellationStore struct {
	// mu          sync.Mutex
	cancelFuncs map[string]context.CancelFunc
}

type PullAction struct {
	Q            *Queue
	Subscription *nats.Subscription
	Fn           ProcessingFunc
}

type PullStore struct {
	pullSubscriptions map[string]PullAction
}

type manager struct {
	servName    string
	isFailureFn func(error) bool
	broker      *NatsBroker
	logger      *ilog.Logger
	concurrency int

	// Stores context-cancel functions for active tasks
	cancellations CancellationStore

	// Stores registered nats.Subscription used for pulling new messages
	pullStore PullStore

	//
	shutdownTimeout time.Duration

	// channel to communicate forceful registration of queues.
	// When server reconnects to a nats-server instance, that does not remember the state of this subscription
	// a forceful re-registration is required
	forceReRegister chan struct{}

	// sema is a counting semaphore to ensure the number of active workers
	// does not exceed the limit.
	sema chan struct{}

	// channel to communicate back to the long running "manager" goroutine
	done chan struct{}

	// once is used to send value to the channel only once
	once sync.Once

	// quit channel is closed when the shutdown of the "manager" goroutine starts
	quit chan struct{}

	// abort channel communicates to the in-flight worker goroutines to stop
	abort chan struct{}

	// Use for result retention
	rw ResultHandlerIFACE
}

type managerParam struct {
	concurrency     int
	name            string
	dbName          string
	broker          *NatsBroker
	logger          *ilog.Logger
	forceReRegister chan struct{}
	isFailureFn     func(error) bool
}

// Constructs new manager
func newManager(
	params managerParam) *manager {

	if params.concurrency < 1 {
		params.concurrency = runtime.NumCPU()
	}

	if params.dbName == "" {
		params.dbName = defaultKVName
	}

	p := &manager{
		servName:        params.name,
		isFailureFn:     params.isFailureFn,
		rw:              NewResultHandlerNats(params.dbName, params.broker.js),
		broker:          params.broker,
		logger:          params.logger,
		cancellations:   NewCancelations(),
		pullStore:       NewPullStore(),
		forceReRegister: params.forceReRegister,
		sema:            make(chan struct{}, params.concurrency),
		done:            make(chan struct{}),
		quit:            make(chan struct{}),
		abort:           make(chan struct{}),
		shutdownTimeout: time.Duration(time.Second * 1),
		concurrency:     params.concurrency,
	}

	p.listenForReRegister(params.name)
	return p
}

// Listen for reconnection to nats-server
//
// Self-heal by connection to nats-server to same endpoints by registering subscriptions again
// Useful when the nats-server goes down and
func (p *manager) listenForReRegister(prefix string) {
	go func() {
		for {
			<-p.forceReRegister
			p.logger.Info("Re-registering subscriptions to nats-server")
			// TODO: validate re-registration flow
			for _, subj := range p.pullStore.pullSubscriptions {
				if !p.broker.isStreamExists(subj.Q.stream) {
					p.logger.Warnf("stream=%s re-registering", subj.Q.stream)
					p.broker.ConnectoQueue(subj.Q)
					p.register(subj.Q, subj.Fn)
				}
			}
			p.logger.Info("Registration successful", p.broker.ns.Servers())
		}
	}()
}

func NewPullStore() PullStore {
	return PullStore{
		pullSubscriptions: make(map[string]PullAction),
	}
}

// NewCancelations returns a Cancelations instance.
func NewCancelations() CancellationStore {
	return CancellationStore{
		cancelFuncs: make(map[string]context.CancelFunc),
	}
}

// Add adds a new cancel func to the collection.
func (c *CancellationStore) Add(id string, fn context.CancelFunc) {
	c.cancelFuncs[id] = fn
}

// Delete deletes a cancel func from the collection given an id.
func (c *CancellationStore) Delete(id string) {
	delete(c.cancelFuncs, id)
}

// Get returns a cancel func given an id.
func (c *CancellationStore) Get(id string) (fn context.CancelFunc, ok bool) {
	fn, ok = c.cancelFuncs[id]
	return fn, ok
}

// func getCancelStreamName(subject string) string {
// 	return fmt.Sprintf("cancel-%s", subject)
// }

// func StreamNameToDurableStreamName(prefix, pattern string) string {
// 	return fmt.Sprintf("%s-%s", prefix, pattern)
// }

// func streamNameFromCancelStream(name string) string {
// 	return strings.Replace(name, "cancel-", "", 1)
// }

// Registers `cancel` subscription
func (p *manager) register_cancel_for(q *Queue) {
	if _, err := p.broker.js.Subscribe(q.cancelSubject, func(m *nats.Msg) {
		defer func() {
			m.Ack()
		}()

		var cancelData TaskCancellationMessage
		if err := json.Unmarshal(m.Data, &cancelData); err != nil {
			p.logger.Error("invalid cancellation payload, skipping ...")
			return
		}

		if cancelFN, ok := p.cancellations.cancelFuncs[cancelData.ID]; ok {
			cancelFN()
			p.logger.Infof("Successfully cancelled: %s/%s", q.stream, cancelData.ID)
		} else {
			// // Cancel fn not found for this task
			// // Check task status
			// // if task stil pending, delete message
			// // if status, ok := p.rw.GetStatus(cancelData.ID); !ok {
			// if obj, err := p.rw.Get(cancelData.ID); err != nil {
			// 	p.logger.Infof("taskID (%s) not found", cancelData.ID)
			// } else {
			// 	status := obj.Status
			// 	switch status {
			// 	// case Pending:
			// 	// 	{
			// 	// 		// fallback case
			// 	// 		if err := p.broker.js.DeleteMsg(cancelData.StreamName, obj.Sequence); err != nil {
			// 	// 			p.logger.Error("Cancellation failed for %s/%s", m.Subject, cancelData.ID)
			// 	// 		} else {
			// 	// 			p.logger.Infof("Successfully cancelled: %s/%s", q.stream, cancelData.ID)
			// 	// 		}

			// 	// 		return
			// 	// 	}
			// 	case Processing:
			// 		{
			// 			// ignored
			// 			// this server does not have access to cancelFN, meaning some other serv does
			// 		}
			// 	case Cancelled:
			// 		{
			// 			// ignore
			// 		}
			// 	default:
			// 		{
			// 			// task is in a state that cannot be cancelled.
			// 			// ( eg. Cancelled, Completed, Failed )
			// 			p.logger.Warnf("Task is in uncancellable state. task=%s status=%s", obj.ID, obj.GetStatus())
			// 		}
			// 	}
			// }
		}
	}, nats.ManualAck()); err != nil {
		p.logger.Fatalf("error connecting cancel-subscriber for %s (%s)", q.stream, err)
	}
}

func (p *manager) register(q *Queue, fn ProcessingFunc) {
	if sub, err := p.broker.js.PullSubscribe(q.subject, "MONITOR", nats.ManualAck()); err != nil {
		panic(fmt.Sprintf("nq: registration of queue=%s failed. (%s)", q.stream, err))
	} else {
		p.register_cancel_for(q)
		p.logger.Infof("Registered queue=%s", q.stream)
		p.pullStore.pullSubscriptions[q.stream] = PullAction{
			Q:            q,
			Subscription: sub,
			Fn:           fn,
		}
	}
}

// perform calls the handler with the given task.
// If the call returns without panic, it simply returns the value,
// otherwise, it recovers from panic and returns an error.
// Ref: https://cs.github.com/hibiken/asynq/blob/master/processor.go#L413
func (p *manager) perform(ctx context.Context, fn ProcessingFunc, task *TaskMessage) (err error) {
	defer func() {
		if x := recover(); x != nil {
			p.logger.Errorf("recovering from panic. See the stack trace below for details:\n%s", string(debug.Stack()))
			_, file, line, ok := runtime.Caller(1) // skip the first frame (panic itself)
			if ok && strings.Contains(file, "runtime/") {
				// The panic came from the runtime, most likely due to incorrect
				// map/slice usage. The parent frame should have the real trigger.
				_, file, line, ok = runtime.Caller(2)
			}

			// Include the file and line number iƒnfo in the error, if runtime.Caller returned ok.
			if ok {
				err = fmt.Errorf("panic [%s:%d]: %v", file, line, x)
			} else {
				err = fmt.Errorf("panic: %v", x)
			}
		}
	}()
	// TODO: clean this up
	return fn(ctx, &TaskPayload{
		ID:      task.ID,
		Payload: task.Payload,
	})
}

// computeDeadline returns the given task's deadline,
func (p *manager) computeDeadline(msg *TaskMessage) time.Time {
	if msg.Timeout == 0 && msg.Deadline == 0 {
		return time.Now().Add(defaultTimeout)
	}
	if msg.Timeout != 0 && msg.Deadline != 0 {
		// Both timeout and deadline set, choosing smaller
		deadlineUnix := math.Min(float64(time.Now().Unix()+msg.Timeout), float64(msg.Deadline))
		return time.Unix(int64(deadlineUnix), 0)
	}
	if msg.Timeout != 0 {
		return time.Now().Add(time.Duration(msg.Timeout) * time.Second)
	}
	return time.Unix(msg.Deadline, 0)
}

// exec pulls a task out of the queue-group for a subject and starts a worker goroutine to
// process the task.
func (p *manager) exec(queueName string) {
	select {
	case <-p.quit:
		p.logger.Debug("manager quit is initiated")
		return
	case p.sema <- struct{}{}: // acquire
		{
			p.logger.Debug("fetching new messages")
			go func() {
				defer func() {
					<-p.sema // free
				}()

				taskMsgs, fn, err := p.pull(context.TODO(), queueName, 1)

				if err != nil {
					p.logger.Debug("failed to fetch tasks", err)
					return
				}

				for _, msg := range taskMsgs {
					deadline := p.computeDeadline(msg)
					fCtx, fCancel := iContext.New(msg.ID, msg.Queue, msg.MaxRetry, msg.CurrentRetry, deadline)
					p.cancellations.Add(msg.ID, fCancel)
					defer func() {
						// remove access to cancelFunc once `perform` is over
						fCancel()
						p.cancellations.Delete(msg.ID)
					}()

					// check if context is already canceled
					// e.g deadline exceeded
					select {
					case <-fCtx.Done():
						p.handleFailedMessage(fCtx, msg, fCtx.Err())
						return
					default:
					}

					resCh := make(chan error, 1)
					go func() {
						p.logger.Infof("Received subject=%s task=%s", queueName, msg.ID)
						msg.Status = Processing
						x, _ := EncodeTMToJSON(msg)
						p.rw.Set(msg.ID, x)

						resCh <- p.perform(fCtx, fn, msg)
					}()

					select {
					case <-p.abort:
						// time is up, quit this worker goroutine.
						// re-publish of message is not required as the msg was not-acked and available to other active workers
						p.logger.Warnf("Quitting worker. Abandoning task=%s", msg.ID)
						return
					case resErr := <-resCh:
						{
							if resErr != nil {
								p.handleFailedMessage(fCtx, msg, resErr)
								return
							}
							p.handleSucceededMessage(fCtx, msg)
							return
						}
					case <-fCtx.Done():
						{
							// acknowledge cancelled
							msg.ackFN()
							p.handleFailedMessage(fCtx, msg, fCtx.Err())
							return
						}
					}
				}

			}()
		}
	}
}

// Pulls a number of tasks
func (p *manager) pull(ctx context.Context, subject string, number int) ([]*TaskMessage, ProcessingFunc, error) {
	if store, ok := p.pullStore.pullSubscriptions[subject]; !ok {
		p.logger.Error("invalid subscription", subject)
		return nil, nil, nil
	} else {
		msgs, err := store.Subscription.Fetch(number, nats.Context(ctx))
		if err != nil {
			p.logger.Debug("failed to fetch messages", err, subject)
			return nil, nil, err
		}
		var taskPayloads []*TaskMessage
		for _, msg := range msgs {
			tp, err := payloadFromNatsMessage(msg)
			if err != nil {
				p.logger.Errorf("invalid task payload")
			} else {
				taskPayloads = append(taskPayloads, tp)
			}
		}
		return taskPayloads, store.Fn, nil
	}
}

func (p *manager) start(wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		p.logger.Debug("Starting Manager")
		defer wg.Done()
		for {
			select {
			case <-p.done:
				p.logger.Debug("Manager done")
				return
			default:
				// sequentially fetch messages from all queues
				for _, subj := range p.pullStore.pullSubscriptions {
					p.exec(subj.Q.stream)
				}
			}
		}
	}()
}

// TODO:
// Note: stops only the "processor" goroutine, does not stop workers.
// It's safe to call this method multiple times.
func (p *manager) stop() {
	for _, subj := range p.pullStore.pullSubscriptions {
		q := NewQueue(subj.Subscription.Subject)
		p.logger.Debugf("Cleaning subscription queue=%s", q.subject)
		subj.Subscription.Unsubscribe()
		p.broker.js.DeleteConsumer(q.stream, "MONITOR")
	}
	p.once.Do(func() {
		p.logger.Debug("Processor shutting down...")
		// Unblock if processor is waiting for sema token.
		close(p.quit)
		// Signal the processor goroutine to stop processing tasks
		// from the queue.
		p.done <- struct{}{}
	})
}

func (p *manager) handleCancelledMessage(ctx context.Context, msg *TaskMessage, err error) {
	p.logger.Debugf("handling cancel task=%s err=%s", msg.ID, ctx.Err())
	msg.CompletedAt = time.Now().Unix()
	msg.Status = Cancelled
	x, _ := EncodeTMToJSON(msg)
	p.rw.Set(msg.ID, x)
	msg.ackFN()
}

func (p *manager) handleSucceededMessage(ctx context.Context, msg *TaskMessage) {
	p.logger.Debugf("handling success task=%s", msg.ID)
	msg.CompletedAt = time.Now().Unix()
	msg.Status = Completed
	msg.ackFN()
	if d, err := EncodeTMToJSON(msg); err != nil {
		// TODO: retry request
		p.logger.Errorf("failed to save message %s", msg.ID)
	} else {
		p.rw.Set(msg.ID, d)
	}
	p.logger.Infof("processed subject=%s task=%s", msg.Queue, msg.ID)
}

func (p *manager) handleFailedMessage(ctx context.Context, msg *TaskMessage, err error) {
	p.logger.Debugf("handling failure task=%s err=%s", msg.ID, ctx.Err())

	if errors.Is(err, context.Canceled) {
		p.handleCancelledMessage(ctx, msg, err)
		return
	}

	if p.isFailureFn(err) || msg.MaxRetry == 0 {
		// mark task a failure
		p.logger.Infof("failed task=%s", msg.ID)
		msg.CompletedAt = time.Now().Unix()
		msg.Status = Failed
		x, _ := EncodeTMToJSON(msg)
		p.rw.Set(msg.ID, x)
		msg.ackFN()
	} else {
		// re-submit task for a ret
		if msg.CurrentRetry == msg.MaxRetry {
			p.logger.Infof("Retry limit reached task=%s. Marked as status=%s", msg.ID, "failed")
			msg.Status = Failed
			x, _ := EncodeTMToJSON(msg)
			p.rw.Set(msg.ID, x)
			msg.ackFN()
		} else {
			msg.CurrentRetry += 1
			p.logger.Infof("Retrying task=%s", msg.ID)
			p.requeue(msg)
		}
	}
}

// Requeue the message back into stream, if task was not completed successfully
//
// TODO: Consider retrying in the same worker instead of pushing message back into the stream
func (p *manager) requeue(t *TaskMessage) {
	p.broker.PublishWithMeta(t)
	msgBytes, _ := EncodeTMToJSON(t)
	p.rw.Set(t.ID, msgBytes)
}

func (p *manager) shutdown() {
	p.stop()

	time.AfterFunc(p.shutdownTimeout, func() { close(p.abort) })

	p.logger.Info("Waiting for all workers to finish...")
	// block until all workers have released the token
	for i := 0; i < cap(p.sema); i++ {
		p.sema <- struct{}{}
	}
	p.logger.Info("All workers have finished")
}

func payloadFromNatsMessage(msg *nats.Msg) (*TaskMessage, error) {
	var tp TaskMessage
	if err := json.Unmarshal(msg.Data, &tp); err != nil || tp.ID == "" {
		return nil, ErrInvalidTaskPayload
	}

	tp.ackFN = msg.Ack
	return &tp, nil
}
