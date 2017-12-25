package wss

import (
	"net/http"

	"github.com/gorilla/websocket"
)

// Conn is an middleman between the websocket connection and the hub.
type Conn struct {
	// The websocket connection.
	ws *websocket.Conn

	// Selected request headers
	headers map[string]string

	// Connection subscriptions
	subscriptions map[string]bool

	// Buffered channel of outbound messages.
	send chan []byte
}

// Config Server config
type Config struct {
}

// Server Websocket server
type Server struct {
	hub      *hub
	Conf     *Config
	routers  routerMap
	upgrader *websocket.Upgrader
}

type routerMap map[string]IRouter

// NewServer make a new websocket server
func NewServer(conf *Config) *Server {
	return &Server{
		hub:     newHub(),
		Conf:    conf,
		routers: make(routerMap),
		upgrader: &websocket.Upgrader{
			CheckOrigin:     func(r *http.Request) bool { return true },
			Subprotocols:    []string{"actioncable-v1-json"},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}
}

// ServeWS handles websocket requests from the peer.
func (s *Server) ServeWS(w http.ResponseWriter, r *http.Request) {
	ws, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := newClient(s, ws, r.Header.Get("X-Request-ID"))
	client.run()
}

// Start start server
func (s *Server) Start() {
	go s.hub.run()
	for _, r := range s.routers {
		go r.Run()
	}
}

// Stop stop server
func (s *Server) Stop() {
	for _, r := range s.routers {
		r.Stop()
	}
	s.hub.stop()
}

// RegisterRouter register a router which implement IRouter interface
func (s *Server) RegisterRouter(router IRouter) {
	s.routers[router.Channel()] = router
}
