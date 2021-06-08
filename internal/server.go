package server

import (
	"github.com/gin-gonic/gin"
	"github.com/matrix-org/gomatrix"
	"github.com/sirupsen/logrus"
	"net/http"
	"net/url"
	"strings"
)

type Server struct {
	addr         string
	engine       *gin.Engine
	debug        bool
	hsToken      string
	asToken      string
	matrixUrl    url.URL
	matrixClient *gomatrix.Client
	logger       *logrus.Logger
}

func New(addr string, opts ...OptFunction) (*Server, error) {
	s := Server{
		addr: addr,
		logger: logrus.New(),
	}

	for _, doOpt := range opts {
		doOpt(&s)
	}

	if !s.debug {
		gin.SetMode(gin.ReleaseMode)
	}

	// Connect to Matrix
	var err error
	s.matrixClient, err = gomatrix.NewClient(s.matrixUrl.String(), "teams-proxy", s.asToken)
	if err != nil {
		return nil, err
	}

	r := gin.Default()
	s.setupRoutes(r)
	return &s, nil
}

func (s *Server) Run() error {
	return s.run()
}

func (s *Server) setupRoutes(engine *gin.Engine) {
	s.engine = engine

	roomsRg := s.engine.Group("/rooms")
	s.handleRooms(roomsRg)
}

func (s *Server) run() error {
	if s.engine == nil {
		return nil
	}

	return s.engine.Run(s.addr)
}

func (s *Server) handleRooms(routerGroup *gin.RouterGroup) {
	routerGroup.GET("/:roomAlias", s.handleRoomAlias)
}

type RoomAliasRequest struct {
	RoomAlias string `uri:"roomAlias"`
}

func (s *Server) handleRoomAlias(c *gin.Context) {
	var rar RoomAliasRequest
	if err := c.ShouldBindUri(&rar); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"msg": "invalid request"})
		return
	}

	localPart := strings.SplitN(rar.RoomAlias, ":", 2)[0][1:]

	// Make request to Matrix Server
	room, err := s.matrixClient.CreateRoom(&gomatrix.ReqCreateRoom{RoomAliasName: localPart})
	if err != nil {
		s.logger.Errorf("unable to create room %s: %s", localPart, err)
		c.JSON(http.StatusInternalServerError, gin.H{"msg": "unable to create room"})
		return
	}

	s.logger.Debugf("room: %v", room)
}
