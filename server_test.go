package nq

import (
	"context"
	"fmt"
	"testing"
	"time"

	// "github.com/nats-io/gnatsd/server"
	"github.com/nats-io/nats-server/v2/server"
	natsserver "github.com/nats-io/nats-server/v2/test"
)

// func TestServerRun(t *testing.T) {

// 	srv := NewServer(NatsClientOpt{
// 		Addr: nats.DefaultURL,
// 	}, Config{
// 		ServerName:  GenerateServerName(),
// 		Concurrency: 1,
// 		LogLevel:    InfoLevel,
// 	},
// 	)

// 	done := make(chan struct{})
// 	// Make sure server exits when receiving TERM signal.
// 	go func() {
// 		time.Sleep(2 * time.Second)
// 		syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
// 		done <- struct{}{}
// 	}()

// 	go func() {
// 		select {
// 		case <-time.After(10 * time.Second):
// 			panic("server did not stop after receiving TERM signal")
// 		case <-done:
// 		}
// 	}()

// 	if err := srv.Run(); err != nil {
// 		t.Fatal(err)
// 	}
// }

const TEST_PORT = 4223

func RunServerOnPort(port int) *server.Server {
	opts := natsserver.DefaultTestOptions
	opts.Port = port
	return RunServerWithOptions(&opts)
}

func RunServerWithOptions(opts *server.Options) *server.Server {
	return natsserver.RunServer(opts)
}

// func TestServerReconnect(t *testing.T) {
// 	s := RunServerOnPort(TEST_PORT)
// 	s.EnableJetStream(&server.JetStreamConfig{})
// 	go func() {
// 		s.Start()
// 	}()
// 	time.Sleep(time.Second * 1)
// 	srv := NewServer(NatsClientOpt{
// 		Addr:          fmt.Sprintf("nats://127.0.0.1:%d", TEST_PORT),
// 		ReconnectWait: time.Second * 2,
// 		MaxReconnects: 100,
// 	}, Config{
// 		ServerName:  GenerateServerName(),
// 		Concurrency: 1,
// 		LogLevel:    InfoLevel,
// 	},
// 	)
// 	srv.Register("test-q", func(ctx context.Context, tp *TaskPayload) error {
// 		// no-op task function
// 		return nil
// 	})

// 	go func() {
// 		srv.Start()
// 	}()

// 	done := make(chan struct{})
// 	go func() {
// 		time.Sleep(2 * time.Second)
// 		s.Shutdown()
// 		time.Sleep(2 * time.Second)
// 		// restart server
// 		s = RunServerOnPort(TEST_PORT)
// 		s.EnableJetStream(&server.JetStreamConfig{})
// 		done <- struct{}{}
// 	}()

// 	// go func() {
// 	// defer srv.Shutdown()
// 	select {
// 	case <-time.After(10 * time.Second):
// 		panic("failed to restart nats-server")
// 	case <-done:
// 		{
// 			// check if actually reconnected
// 			client := NewPublishClient(NatsClientOpt{
// 				Addr: fmt.Sprintf("nats://127.0.0.1:%d", TEST_PORT),
// 			})
// 			defer client.Close()

// 			task1 := NewTask("test-q", []byte{})
// 			if ack, err := client.Enqueue(task1); err == nil {
// 				time.Sleep(time.Second * 2)
// 				msg, err := client.Fetch(ack.ID)
// 				if err != nil {
// 					panic(err)
// 				}
// 				if msg.GetStatus() != "completed" {
// 					panic(fmt.Sprintf("expected=%s found=%s", "completed", msg.GetStatus()))
// 				}
// 				srv.Shutdown()
// 			} else {
// 				panic("failed to send task")
// 			}
// 		}
// 	}

// 	defer s.Shutdown()
// }

func TestServerDie(t *testing.T) {
	s := RunServerOnPort(TEST_PORT)
	s.EnableJetStream(&server.JetStreamConfig{})
	defer s.Shutdown()
	go func() {
		s.Start()
	}()

	time.Sleep(time.Second * 1) // time for nats to start
	srv := NewServer(NatsClientOpt{
		Addr:          fmt.Sprintf("nats://127.0.0.1:%d", TEST_PORT),
		ReconnectWait: time.Second * 2,
		MaxReconnects: 100,
	}, Config{
		ServerName:  GenerateServerName(),
		Concurrency: 1,
		LogLevel:    InfoLevel,
	},
		ShutdownOnNatsDisconnect(),
	)

	srv.Register("test-q", func(ctx context.Context, tp *TaskPayload) error {
		// no-op task function
		return nil
	})

	go func() {
		// note2self: run() over start() since run added `listeners`
		_ = srv.Run()
	}()

	done := make(chan struct{})
	go func() {
		time.Sleep(2 * time.Second)
		t.Log("shutting down nats")
		s.Shutdown()
		time.Sleep(2 * time.Second)
		done <- struct{}{}
	}()

	select {
	case <-time.After(10 * time.Second):
		panic("failed to restart nats-server")
	case <-done:
		{
			if srv.state.value != srvStateClosed {
				t.Fatal("nq failed to quit")
			}
		}
	}

}
