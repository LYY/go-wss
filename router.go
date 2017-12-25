package wss

// IRouter router interface
type IRouter interface {
	Channel() string
	Register(*Client, *Message) error
	Unregister(*Client, *Message) error
	Perform(*Client, *Message) error
	IsClosed() bool
	Run()
	Stop()
}
