package server

import (
	"fmt"
	teams_api "github.com/fossteams/teams-api"
	"github.com/gin-gonic/gin"
	"github.com/matrix-org/gomatrix"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type MatrixSettings struct {
	RoomAlias string
}

type Server struct {
	addr           string
	engine         *gin.Engine
	debug          bool
	hsToken        string
	asToken        string
	matrixUrl      url.URL
	matrixClient   *gomatrix.Client
	matrixSettings *MatrixSettings
	logger         *logrus.Logger
	teamsClient    *teams_api.TeamsClient
}

func New(addr string, opts ...OptFunction) (*Server, error) {
	s := Server{
		addr:   addr,
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

	transactionsRg := s.engine.Group("/transactions")
	s.handleTransactions(transactionsRg)
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

func (s *Server) handleTransactions(rg *gin.RouterGroup) {
	rg.PUT("/:id", func(c *gin.Context) {
		var mUrl struct {
			Id int `uri:"id"`
		}
		err := c.BindUri(&mUrl)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "invalid ID",
			})
			return
		}

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			s.logger.Errorf("unable to read body: %v", err)
			return
		}

		s.logger.Debugf("transaction %d: %v", mUrl.Id, string(body))
	})
}

func (s *Server) initTeamsClient() error {
	t, err := teams_api.New()
	if err != nil {
		return err
	}
	s.teamsClient = t
	return nil
}

func (s *Server) Init() error {
	s.matrixSettings = &MatrixSettings{RoomAlias: "teams_"}
	err := s.initTeamsClient()
	if err != nil {
		return fmt.Errorf("unable to initialize Teams Client: %v", err)
	}

	// Teams Client is initialized, let's fetch its rooms
	conv, err := s.teamsClient.GetConversations()
	if err != nil {
		return fmt.Errorf("unable to get conversations: %v", err)
	}

	for _, t := range conv.Teams {
		for _, c := range t.Channels {
			roomAlias := fmt.Sprintf("%s%s", s.matrixSettings.RoomAlias, cleanId(c.Id))
			roomName := fmt.Sprintf("%s - %s", t.DisplayName, c.DisplayName)
			s.logger.Debugf("creating room alias: %v for %s", roomAlias, roomName)
			resp, err := s.matrixClient.CreateRoom(
				&gomatrix.ReqCreateRoom{
					RoomAliasName: roomAlias,
					Name:          roomName,
					Visibility:    "public",
			})
			var roomId string
			if err != nil {
				// Room already exists?
				respJoinRoom, err := s.matrixClient.JoinRoom("#" + roomAlias, "matrix-teams", nil)
				if err != nil {
					s.logger.Warnf("unable to join room: %s", err.(gomatrix.HTTPError).Contents)
					continue
				}
				roomId = respJoinRoom.RoomID
			} else {
				roomId = resp.RoomID
			}

			s.logger.Debugf("created room for channel %v: %v", roomName, roomId)

			// Fill Messages
			messages, err := s.teamsClient.GetMessages(&c)
			if err != nil {
				s.logger.Warnf("unable to get messages for channel %v: %v", c.Id, err)
				continue
			}

			for _, m := range messages {
				resp, err := s.matrixClient.SendMessageEvent(roomId, "m.room.message", gin.H{
					"msgtype": "m.text",
					"body": m.Content,
				})
				if err != nil {
					s.logger.Warnf("unable to send message to %s: %v", roomId, err)
					continue
				}
				s.logger.Debugf("sendMessage response=%v", resp)
			}
		}
	}

	return nil
}

func cleanId(id string) string {
	s := id
	s = strings.Replace(s, "@thread.tacv2", "", -1)
	s = strings.Replace(s, ":", "", -1)
	return s
}
