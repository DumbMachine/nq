package nq

import (
	"errors"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	ilog "github.com/dumbmachine/nq/internal/log"
	"github.com/nats-io/nats.go"
	"golang.org/x/sys/unix"
)

// Generates a server name, combination of hostname and process id
//
func GenerateServerName() string {
	host, err := os.Hostname()
	if err != nil {
		host = "unknown_host"
	}
	// Hostname may have "." in them, which are not safe for use as nats' durable subscription name
	host = strings.Replace(host, ".", "-", -1) // replace all dots
	return fmt.Sprintf("%s/%d", host, os.Getpid())
}

// LogLevel represents a log level.
type LogLevel int32

const (
	// Note: Zero value, when unspecified
	level_unspecified LogLevel = iota

	// DebugLevel is the lowest level of logging.
	// Debug logs are intended for debugging and development purposes.
	DebugLevel

	// InfoLevel is used for gener al informational log messages.
	InfoLevel

	// WarnLevel is used for undesired but relatively expected events,
	// which may indicate a problem.
	WarnLevel

	// ErrorLevel is used for undesired and unexpected events that
	// the program can recover from.
	ErrorLevel

	// FatalLevel is used for undesired and unexpected events that
	// the program cannot recover from.
	FatalLevel
)

// Server config
type Config struct {
	// Durable name for this workser. Required to re-establish connection
	// to nats-server
	ServerName string

	// Maximum number of concurrent processing of tasks.
	//
	// If set to a zero or negative value, the number of CPUs usable by the current process is picked.
	Concurrency int

	// Predicate function to determine whether the error returned from a task is an error.
	// If function returns true, Server will retry the task ( bounded by retry-limit set on task )
	//
	// By default, non-nil the function returns true.
	IsFailureFn func(error) bool

	// Logger specifies the logger used by the server instance.
	//
	// go's logger is used by default.
	Logger ilog.Base

	// LogLevel specifies the minimum log level to enable.
	//
	// InfoLevel is used by default.
	LogLevel LogLevel

	// ShutdownTimeout specifies the duration to wait to let workers finish their tasks
	// before forcing them to abort when stopping the server.
	//
	// Defaults to timeout of 5 seconds.
	ShutdownTimeout time.Duration
}

var (
	// Default function to validate if task returned error indicates failure
	defaultIsFailureFn = func(err error) bool {
		return err == nil
	}

	// Default timeout used if both timeout and deadline are not specified.
	defaultTimeout = (math.MaxInt32) * time.Second
)

var ErrServerNameEmpty = errors.New("server name cannot be empty")
var ErrServerClosed = errors.New("nq: Server closed")
var ErrFailedToConnect = errors.New("failed to connect to nats client")
var ErrQueueNotFound = errors.New("nq: queue not found")
var ErrStreamNotCreated = errors.New("nq: stream not created")
var ErrTaskIDEmpty = errors.New("nq: task id cannot be empty")
var ErrInvalidTaskPayload = errors.New("invalid task payload") // Happens when malformed data is sent to task-stream
var ErrTaskNotFound = errors.New("task not found")
var ErrCannotCancelDeletedTask = errors.New("deleted task cannot be cancelled") // trying to cancel an already cancelled task

var serverStates = []string{
	"new",
	"active",
	"stopped",
	"closed",
}

func (s serverStateValue) String() string {
	if srvStateNew <= s && s <= srvStateClosed {
		return serverStates[s]
	}
	return "unknown status"
}

type serverStateValue int

const (
	// StateNew represents a new server.
	//
	// Server begins in this state and then
	// transition to StatusActive when
	// Start or Run is called.
	srvStateNew serverStateValue = iota

	// StateActive indicates the server is up and active.
	srvStateActive

	// StateStopped indicates the server is up but no longer processing new tasks.
	srvStateStopped

	// StateClosed indicates the server has been shutdown.
	srvStateClosed
)

type serverState struct {
	mu    sync.Mutex
	value serverStateValue
}

func toInternalLogLevel(l LogLevel) ilog.Level {
	switch l {
	case DebugLevel:
		return ilog.DebugLevel
	case InfoLevel:
		return ilog.InfoLevel
	case WarnLevel:
		return ilog.WarnLevel
	case ErrorLevel:
		return ilog.ErrorLevel
	case FatalLevel:
		return ilog.FatalLevel
	}
	panic(fmt.Sprintf("nq: unexpected log level: %v", l))
}

// Responsible for task lifecycle management and  processing
type Server struct {
	name string
	// wait group to wait for all goroutines to finish.
	wg                   sync.WaitGroup
	state                *serverState
	natsConnectionClosed chan struct{}
	signals              chan os.Signal
	logger               *ilog.Logger
	processor            *manager
}

func NewServer(natsConfig NatsClientOpt, servCfg Config, opts ...ClientConnectionOption) *Server {
	lvl := servCfg.LogLevel
	if lvl == level_unspecified {
		lvl = InfoLevel
	}
	logger := ilog.NewLogger(servCfg.Logger, ilog.Level(lvl-1)) // use toInternalLogLevel instead

	if servCfg.ServerName == "" {
		servCfg.ServerName = GenerateServerName()
		logger.Warnf("Empty server name replace with to %s", servCfg.ServerName)
	}

	opt, err := withDefaultClientOptions(opts...)
	opt.NatsOption = append(opt.NatsOption, nats.Name(servCfg.ServerName))
	if err != nil {
		panic(err)
	}

	natsConnectionClosed := make(chan struct{})
	forceReRegister := make(chan struct{})
	sigs := make(chan os.Signal, 1)

	broker, err := NewNatsBroker(natsConfig, opt, natsConnectionClosed, forceReRegister)
	if err != nil {
		panic(err)
	}
	if servCfg.IsFailureFn == nil {
		servCfg.IsFailureFn = defaultIsFailureFn
	}

	if servCfg.IsFailureFn == nil {
		servCfg.IsFailureFn = defaultIsFailureFn
	}

	proc := newManager(managerParam{
		concurrency:     servCfg.Concurrency,
		name:            servCfg.ServerName,
		dbName:          natsConfig.DBName,
		broker:          broker,
		logger:          logger,
		forceReRegister: forceReRegister,
		isFailureFn:     servCfg.IsFailureFn,
	})

	srvState := &serverState{value: srvStateNew}
	srv := &Server{
		name:                 servCfg.ServerName,
		state:                srvState,
		logger:               logger,
		processor:            proc,
		natsConnectionClosed: natsConnectionClosed,
		signals:              sigs,
	}
	return srv
}

// Subscribe to a stream
func (srv *Server) Register(qname string, fn ProcessingFunc) {
	q := NewQueue(qname)
	srv.processor.broker.ConnectoQueue(q)
	srv.processor.register(q, fn)
}

func (srv *Server) start() error {
	srv.state.mu.Lock()
	defer srv.state.mu.Unlock()
	switch srv.state.value {
	case srvStateActive:
		return fmt.Errorf("nq: the server is already running")
	case srvStateStopped:
		return fmt.Errorf("nq: the server is in the stopped state. Waiting for shutdown")
	case srvStateClosed:
		return ErrServerClosed
	}
	srv.state.value = srvStateActive
	return nil
}

// Start starts the worker server. Once the server has started,
// it pulls tasks off queues and starts a worker goroutine for each task
// and then call Handler to process it.
// Tasks are processed concurrently by the workers up to the number of
// concurrency specified in Config.Concurrency.
//
// Start returns any error encountered at server startup time.
// If the server has already been shutdown, ErrServerClosed is returned.
func (srv *Server) Start() error {
	if err := srv.start(); err != nil {
		return err
	}
	srv.logger.Infof("Started Server@%s", srv.name)
	srv.processor.start(&srv.wg)
	return nil
}

// TODO: close this goroutine when srv shuts, or maybe a waitgroup here
func (srv *Server) listenForDisconnect() {
	go func() {
		for {
			<-srv.natsConnectionClosed
			srv.logger.Error("Disconnected from nats")
			srv.signals <- unix.SIGINT
			return
		}
	}()
}

// waitForSignals waits for signals and handles them.
// It handles SIGTERM, SIGINT, and SIGTSTP.
// SIGTERM and SIGINT will signal the process to exit.
// SIGTSTP will signal the process to stop processing new tasks.
func (srv *Server) waitForSignals(sigs chan os.Signal) {
	srv.logger.Infof("[*] Listening for messages")
	srv.logger.Info("cmd/ctrl + c to terminate the process")
	srv.logger.Info("cmd/ctrl + z to stop processing new tasks")

	signal.Notify(sigs, unix.SIGTERM, unix.SIGINT, unix.SIGTSTP)
	for {
		sig := <-sigs
		if sig == unix.SIGTSTP {
			srv.Stop()
			continue
		}
		break
	}
}

// Run starts the task processing and blocks until
// an os signal to exit the program is received. Once it receives
// a signal, it gracefully shuts down all active workers and other
// goroutines to process the tasks.
//
// Run returns any error encountered at server startup time.
// If the server has already been shutdown, ErrServerClosed is returned.
func (srv *Server) Run() error {
	if err := srv.Start(); err != nil {
		return err
	}
	srv.listenForDisconnect()
	srv.waitForSignals(srv.signals)
	srv.Shutdown()
	return nil
}

// Shutdown gracefully shuts down the server.
// It gracefully closes all active workers. The server will wait for
// active workers to finish processing tasks for duration specified in Config.ShutdownTimeout.
// If worker didn't finish processing a task during the timeout, the task will be pushed back to Redis.
func (srv *Server) Shutdown() {
	srv.state.mu.Lock()
	if srv.state.value == srvStateNew || srv.state.value == srvStateClosed {
		srv.state.mu.Unlock()
		// server is not running, do nothing.
		return
	}
	srv.state.value = srvStateClosed
	srv.state.mu.Unlock()

	srv.logger.Info("Starting graceful shutdown")
	srv.processor.shutdown()

	srv.wg.Wait()

	srv.logger.Info("Exiting")
}

// Stop signals the server to stop pulling new tasks off queues.
// Stop can be used before shutting down the server to ensure that all
// currently active tasks are processed before server shutdown.
//
// Stop does not shutdown the server, make sure to call Shutdown before exit.
func (srv *Server) Stop() {
	srv.state.mu.Lock()
	if srv.state.value != srvStateActive {
		// Invalid calll to Stop, server can only go from Active state to Stopped state.
		srv.state.mu.Unlock()
		return
	}
	srv.state.value = srvStateStopped
	srv.state.mu.Unlock()

	srv.logger.Info("Stopping processor")
	srv.processor.stop()
	srv.logger.Info("Processor stopped")
}
