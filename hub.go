package wss

type hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients.
	broadcast chan *Message

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client
}

func newHub() *hub {
	return &hub{
		broadcast:  make(chan *Message),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

func (h *hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				// client.close()
			}
		case message := <-h.broadcast:
			// TODO: make reply
			reply := &Reply{
				Data: message.Data,
			}
			for client := range h.clients {
				select {
				case client.send <- reply:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

func (h *hub) stop() {
	for c := range h.clients {
		c.close()
	}
}
