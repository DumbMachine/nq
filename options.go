package nq

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

const (
	defaultMaxReconnects            = 100
	defaultShutdownOnNatsDisconnect = false
	defaultKVName                   = "nq-store"
	defaultReconnectWait            = time.Second * 10
	defaultAuthenticationType       = NoAuthenticationOpt
	defaultMaxRetry                 = 0
)

var defaultNatsOptions = make([]nats.Option, 0)
var defaultAuthenticationObject interface{} = nil

const (
	// Authentication types
	// TODO: error on using multiple of belwo
	UPassAuthenticationOpt ClientOptionType = iota
	TokenAuthenticationOpt
	NoAuthenticationOpt

	// General options
	ShutdownOnNatsDisconnectOpt
)

type ClientConnectionOption interface {
	String() string
	Type() ClientOptionType
	Value() interface{}
}

// Internal option representations.
type (
	shutdownOnNatsDisconnect struct{}
	noAuthentication         struct{}
	uPassAuthentication      struct {
		Username string
		Password string
	}
	tokenAuthentication struct {
		Name  string
		Token string
	}
)

// Shutdown server and workers if connection with nats-server is broken.
// Results in any executing tasks being cancelled, if not finished in time specified by `shutdownTimeout`. Option is useful when workers should be
// `cancellable` at all times.
//
// By default, inactive
func ShutdownOnNatsDisconnect() shutdownOnNatsDisconnect {
	return shutdownOnNatsDisconnect{}
}

func (u shutdownOnNatsDisconnect) String() string {
	return "nil"
}
func (u shutdownOnNatsDisconnect) Type() ClientOptionType { return ShutdownOnNatsDisconnectOpt }
func (u shutdownOnNatsDisconnect) Value() interface{}     { return u }

// Connect to nats-server without any authentication
//
// Default
func NoAuthentcation() noAuthentication {
	return noAuthentication{}
}

func (u noAuthentication) String() string {
	return "nil"
}
func (u noAuthentication) Type() ClientOptionType { return NoAuthenticationOpt }
func (u noAuthentication) Value() interface{}     { return u }

// Connect to nats-server using username:password pair
//
// Read more: https://docs.nats.io/running-a-nats-service/configuration/securing_nats/auth_intro/username_passwordß
func UserPassAuthentcation(username, password string) uPassAuthentication {
	return uPassAuthentication{username, password}
}

func (u uPassAuthentication) String() string {
	return fmt.Sprintf("Username(%s), Password(%s)", u.Username, u.Password)
}
func (u uPassAuthentication) Type() ClientOptionType { return UPassAuthenticationOpt }
func (u uPassAuthentication) Value() interface{}     { return u }

// Connect to nats-server using token authentication
//
// Read more: https://docs.nats.io/running-a-nats-service/configuration/securing_nats/auth_intro/tokens
func TokenAuthentication(username, password string) tokenAuthentication {
	return tokenAuthentication{username, password}
}

func (u tokenAuthentication) String() string {
	return fmt.Sprintf("Name(%s), Token(%s)", u.Name, u.Token)
}
func (u tokenAuthentication) Type() ClientOptionType { return TokenAuthenticationOpt }
func (u tokenAuthentication) Value() interface{}     { return u }

// Merges user defined options with default options
// Also validates user provided options
func withDefaultClientOptions(opts ...ClientConnectionOption) (ClientOption, error) {
	res := ClientOption{
		Timeout:                  time.Duration(time.Minute),
		AuthenticationType:       defaultAuthenticationType,
		AuthenticationObject:     defaultAuthenticationObject,
		NatsOption:               defaultNatsOptions,
		ShutdownOnNatsDisconnect: defaultShutdownOnNatsDisconnect,
	}

	for _, opt := range opts {
		switch opt := opt.(type) {
		case shutdownOnNatsDisconnect:
			{
				res.ShutdownOnNatsDisconnect = true
			}
		case noAuthentication:
			{
				// skip
				continue
			}
		case uPassAuthentication:
			{
				res.NatsOption = append(res.NatsOption, nats.UserInfo(opt.Username, opt.Password))

			}
		case tokenAuthentication:
			{
				res.NatsOption = append(res.NatsOption, nats.Name(opt.Name), nats.Token(opt.Token))
			}
		default:
			{ // unexpected option
			}
		}
	}
	return res, nil
}
