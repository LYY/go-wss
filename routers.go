package wss

import "encoding/json"

type subchannelClients map[*Client]bool

// SubchannelWorker subchannel worker interface
type SubchannelWorker interface {
	SetRouter(*SubchannelRouter)
	Run()
	Stop()
}

// SubchannelRouter subchannel router, used for subchannel message
type SubchannelRouter struct {
	channel     string
	subchannels map[string]subchannelClients
	register    chan *subchannelRegistrar
	unregister  chan *subchannelRegistrar
	broker      chan *SubchannelReply
	closed      chan bool
	worker      SubchannelWorker
}

// SubchannelMessage subchannel message
type SubchannelMessage struct {
	Subchannel string
	Message    json.RawMessage
}

type subchannelRegistrar struct {
	subchannel string
	client     *Client
}

// SubchannelReply subchannel reply
type SubchannelReply struct {
	Subchannel string
	Data       json.RawMessage
}

// NewSubchannelRouter create a new subchannel router
func NewSubchannelRouter(channel string, worker SubchannelWorker) *SubchannelRouter {
	router := &SubchannelRouter{
		channel:     channel,
		subchannels: make(map[string]subchannelClients),
		register:    make(chan *subchannelRegistrar),
		unregister:  make(chan *subchannelRegistrar),
		broker:      make(chan *SubchannelReply, 256),
		closed:      make(chan bool),
		worker:      worker,
	}
	worker.SetRouter(router)
	return router
}

// Channel interface implementation
func (sr *SubchannelRouter) Channel() string {
	return sr.channel
}

// IsClosed interface implementation
func (sr *SubchannelRouter) IsClosed() bool {
	select {
	case <-sr.closed:
		return true
	default:
	}
	return false
}

// Register interface implementation
func (sr *SubchannelRouter) Register(c *Client, msg *Message) error {
	if sr.IsClosed() {
		return nil
	}
	var sbcMsg SubchannelMessage
	json.Unmarshal(msg.Data, &sbcMsg)
	sr.register <- &subchannelRegistrar{subchannel: sbcMsg.Subchannel, client: c}
	return nil
}

// Unregister interface implementation
func (sr *SubchannelRouter) Unregister(c *Client, msg *Message) error {
	if sr.IsClosed() {
		return nil
	}
	subchan := ""
	if msg != nil {
		var sbcMsg SubchannelMessage
		json.Unmarshal(msg.Data, &sbcMsg)
		subchan = sbcMsg.Subchannel
	}
	sr.unregister <- &subchannelRegistrar{subchannel: subchan, client: c}
	return nil
}

// Perform interface implementation
func (sr *SubchannelRouter) Perform(c *Client, msg *Message) error {
	var sbcMsg SubchannelMessage
	json.Unmarshal(msg.Data, &sbcMsg)
	return nil
}

// Reply interface implementation
func (sr *SubchannelRouter) Reply(subReply *SubchannelReply) {
	if sr.IsClosed() {
		return
	}
	sr.broker <- subReply
}

// Run interface implementation
func (sr *SubchannelRouter) Run() {
	sr.worker.Run()

	for {
		select {
		case <-sr.closed:
			for subchannel, clients := range sr.subchannels {
				for c := range clients {
					delete(clients, c)
				}
				delete(sr.subchannels, subchannel)
			}
			return
		case subReply := <-sr.broker:
			data, _ := json.Marshal(subReply)
			reply := &Reply{
				Channel: sr.channel,
				Data:    data,
			}
			channel := subReply.Subchannel
			clients, ok := sr.subchannels[channel]
			if ok {
				for c := range clients {
					c.Reply(reply)
				}
			}
		case r := <-sr.register:
			subchan := r.subchannel
			c := r.client
			clients, ok := sr.subchannels[subchan]
			if !ok {
				clients = make(subchannelClients)
				sr.subchannels[subchan] = clients
			}
			if _, ok := clients[c]; !ok {
				clients[c] = true
			}
		case r := <-sr.unregister:
			subchan := r.subchannel
			c := r.client
			if subchan == "" {
				for _, clients := range sr.subchannels {
					delete(clients, c)
				}
			} else {
				clients, ok := sr.subchannels[subchan]
				if ok {
					delete(clients, c)
				}
			}
		}
	}
}

// Stop interface implementation
func (sr *SubchannelRouter) Stop() {
	close(sr.closed)
	sr.worker.Stop()
}
