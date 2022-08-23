module github.com/dumbmachine/nq

go 1.17

retract (
	// unintentional uploads
	v0.1.0-beta.4
	v0.1.0-beta.3
	v0.1.0-beta.2
	v0.1.0-beta.1
	v0.1.0-beta.0
)

require (
	github.com/nats-io/nats-server/v2 v2.8.4
	github.com/nats-io/nats.go v1.16.0
	github.com/nats-io/nuid v1.0.1
	golang.org/x/sys v0.0.0-20220804214406-8e32c043e418
)

require (
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/klauspost/compress v1.14.4 // indirect
	github.com/minio/highwayhash v1.0.2 // indirect
	github.com/nats-io/jwt/v2 v2.2.1-0.20220330180145-442af02fd36a // indirect
	github.com/nats-io/nkeys v0.3.0 // indirect
	golang.org/x/crypto v0.0.0-20220315160706-3147a52a75dd // indirect
	golang.org/x/time v0.0.0-20220722155302-e5dcc9cfc0b9 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
)
