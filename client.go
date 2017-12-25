package wss

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/gommon/random"
)

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	srv *Server

	id string

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan *Reply

	closed chan bool

	routers map[string]IRouter
}

func newClient(srv *Server, conn *websocket.Conn, id string) *Client {
	if id == "" {
		id = generator()
	}

	return &Client{
		srv:     srv,
		id:      id,
		conn:    conn,
		send:    make(chan *Reply, 256),
		closed:  make(chan bool),
		routers: make(map[string]IRouter),
	}
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump() {
	defer func() {
		c.close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("error: %v", err)
			}
			break
		}
		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("error: %v", err)
			break
		}
		if router, ok := c.srv.routers[msg.Channel]; ok {
			switch msg.Command {
			case cmdSubscribe:
				if err := router.Register(c, &msg); err == nil {
					c.routers[msg.Channel] = router
				}
			case cmdUnsubscribe:
				router.Unregister(c, &msg)
				delete(c.routers, msg.Channel)
			default:
				router.Perform(c, &msg)
			}
		}
	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.close()
	}()
	for {
		select {
		case reply, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteJSON(reply); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) run() {
	c.srv.hub.register <- c
	go c.readPump()
	go c.writePump()
}

func (c *Client) close() {
	select {
	case <-c.closed:
	default:
		close(c.closed)
		close(c.send)
		c.conn.Close()
		for _, r := range c.routers {
			r.Unregister(c, nil)
		}
		c.srv.hub.unregister <- c
	}
}

// Connection return websocket connection
func (c *Client) Connection() *websocket.Conn {
	return c.conn
}

// Reply send reply to client
func (c *Client) Reply(reply *Reply) {
	select {
	case <-c.closed:
	default:
		c.send <- reply
	}
}

func generator() string {
	return random.String(32)
}
