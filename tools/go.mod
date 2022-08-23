module github.com/dumbmachine/nq/tools

go 1.18

// dev
replace (
	github.com/dumbmachine/nq => ../
)

require (
	github.com/dumbmachine/nq v0.1.0-beta.4
	github.com/spf13/cobra v1.5.0
)

require (
	github.com/google/uuid v1.3.0 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/nats-io/nats-server/v2 v2.8.4 // indirect
	github.com/nats-io/nats.go v1.16.0 // indirect
	github.com/nats-io/nkeys v0.3.0 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/sirupsen/logrus v1.9.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	golang.org/x/crypto v0.0.0-20220315160706-3147a52a75dd // indirect
	golang.org/x/sys v0.0.0-20220804214406-8e32c043e418 // indirect
)
