package nq

import (
	"errors"

	"github.com/nats-io/nats.go"
)

type ResultHandlerIFACE interface {
	// Get the result of a task in nats kv store
	Get(id string) (*TaskMessage, error)
	// Set the result of a task in nats kv store
	Set(id string, data []byte) error
	Watch(id string) (chan *TaskMessage, error)
	GetAllKeys(id string, data []byte) ([]string, error)
}

type ResultHandlerNats struct {
	kv nats.KeyValue
}

func NewResultHandlerNats(name string, js nats.JetStreamContext) *ResultHandlerNats {
	kv, err := js.KeyValue(name)

	if errors.Is(err, nats.ErrBucketNotFound) {
		// create the bucket
		kv, err := js.CreateKeyValue(&nats.KeyValueConfig{
			Bucket:      defaultKVName,
			Description: "used by package for status retention and fetching",
			Storage:     nats.FileStorage,
		})

		if err != nil {
			// failed to create a kv store
			panic(err)
		}

		return &ResultHandlerNats{
			kv: kv,
		}
	}

	if err != nil {
		panic(err)
	}

	return &ResultHandlerNats{
		kv: kv,
	}
}

func (rn *ResultHandlerNats) Get(id string) (*TaskMessage, error) {
	x, err := rn.kv.Get(id)
	if err != nil {
		if errors.Is(err, nats.ErrKeyNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}
	return DecodeTMFromJSON(x.Value())

}
func (rn *ResultHandlerNats) Set(id string, data []byte) error {
	if _, err := rn.kv.Put(id, data); err != nil {
		return err
	} else {
		return nil
	}
}

// Get all keys from nats key-value store
func (rn *ResultHandlerNats) GetAllKeys(id string, data []byte) ([]string, error) {
	if keys, err := rn.kv.Keys(); err != nil {
		return nil, err
	} else {
		return keys, nil
	}
}

func (rn *ResultHandlerNats) Watch(id string) (chan *TaskMessage, error) {
	watcher, err := rn.kv.Watch(id)
	status := make(chan *TaskMessage)
	go func() {
		for updated := range watcher.Updates() {
			if updated != nil {
				// TODO: handle error
				x, err := DecodeTMFromJSON(updated.Value())
				if err != nil {
					close(status)
					return
				}
				// log.Println("decode err", err, x)
				status <- x

				// check if status is terminal
				if x.Status == Failed || x.Status == Completed || x.Status == Deleted || x.Status == Cancelled {
					close(status)
					return
				}
			}
		}
	}()

	return status, err
}
