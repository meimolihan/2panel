package v2

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/2Panel-dev/2Panel/agent/app/api/v2/helper"
	"github.com/2Panel-dev/2Panel/agent/app/model"
	"github.com/2Panel-dev/2Panel/agent/app/service"
	"github.com/2Panel-dev/2Panel/agent/global"
	"github.com/2Panel-dev/2Panel/agent/utils/cmd"
	"github.com/2Panel-dev/2Panel/agent/utils/ssh"
	"github.com/2Panel-dev/2Panel/agent/utils/terminal"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

// @Tags Terminal
// @Summary Ws local terminal
// @Param command query string false "command"
// @Success 200
// @Security ApiKeyAuth
// @Security Timestamp
// @Router /hosts/terminal/local [get]
func (b *BaseApi) WsLocalTerminal(c *gin.Context) {
	b.runSSHSession(c, loadLocalConn, c.DefaultQuery("command", ""))
}

// @Tags Terminal
// @Summary Ws host SSH
// @Param id query integer false "id"
// @Param command query string false "command"
// @Success 200
// @Security ApiKeyAuth
// @Security Timestamp
// @Router /hosts/terminal/ssh [get]
func (b *BaseApi) WsHostSSH(c *gin.Context) {
	b.runSSHSession(c, func() (*ssh.SSHClient, error) {
		hostID, _ := strconv.Atoi(c.DefaultQuery("id", "0"))
		if hostID <= 0 {
			return nil, errors.New("missing host id")
		}
		host, err := service.GetHostInfo(uint(hostID))
		return newHostSSHClient(host, err)
	}, c.DefaultQuery("command", ""))
}

// @Tags Terminal
// @Summary Ws container terminal
// @Param cols query integer false "cols"
// @Param rows query integer false "rows"
// @Success 200
// @Security ApiKeyAuth
// @Security Timestamp
// @Router /hosts/terminal/container [get]
func (b *BaseApi) WsContainerTerminal(c *gin.Context) {
	wsConn, cols, rows, ok := prepareTerminalSession(c)
	if !ok {
		return
	}
	defer wsConn.Close()

	slave, err := loadContainerTerminalCommand(c)
	if wshandleError(wsConn, err) {
		return
	}
	defer slave.Close()

	tty, err := terminal.NewLocalWsSession(cols, rows, wsConn, slave, false)
	if wshandleError(wsConn, err) {
		return
	}

	quitChan := make(chan bool, 3)
	tty.Start(quitChan)
	go slave.Wait(quitChan)

	<-quitChan

	global.LOG.Info("websocket finished")
	closeTerminalConn(wsConn)
}

func prepareTerminalSession(c *gin.Context) (*websocket.Conn, int, int, bool) {
	if !websocket.IsWebSocketUpgrade(c.Request) {
		helper.Success(c)
		return nil, 0, 0, false
	}
	wsConn, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		global.LOG.Errorf("gin context http handler failed, err: %v", err)
		return nil, 0, 0, false
	}

	if global.CONF.Base.IsDemo {
		if wshandleError(wsConn, errors.New("   demo server, prohibit this operation!")) {
			return nil, 0, 0, false
		}
	}

	cols, err := strconv.Atoi(c.DefaultQuery("cols", "80"))
	if wshandleError(wsConn, errors.WithMessage(err, "invalid param cols in request")) {
		return nil, 0, 0, false
	}
	rows, err := strconv.Atoi(c.DefaultQuery("rows", "40"))
	if wshandleError(wsConn, errors.WithMessage(err, "invalid param rows in request")) {
		return nil, 0, 0, false
	}
	return wsConn, cols, rows, true
}

func (b *BaseApi) runSSHSession(c *gin.Context, connect func() (*ssh.SSHClient, error), command string) {
	wsConn, cols, rows, ok := prepareTerminalSession(c)
	if !ok {
		return
	}
	defer wsConn.Close()

	client, clientErr := connect()
	if wshandleError(wsConn, errors.WithMessage(clientErr, "failed to set up the connection. Please check the host information")) {
		return
	}
	defer client.Close()

	sws, err := terminal.NewLogicSshWsSession(cols, rows, client.Client, wsConn, command)
	if wshandleError(wsConn, err) {
		return
	}
	defer sws.Close()

	quitChan := make(chan bool, 3)
	sws.Start(quitChan)
	go sws.Wait(quitChan)

	<-quitChan

	closeTerminalConn(wsConn)
}

func closeTerminalConn(wsConn *websocket.Conn) {
	dt := time.Now().Add(time.Second)
	_ = wsConn.WriteControl(websocket.CloseMessage, nil, dt)
}

func newHostSSHClient(host *model.Host, err error) (*ssh.SSHClient, error) {
	if err != nil {
		return nil, errors.WithMessage(err, "load host info by id failed")
	}
	connInfo := ssh.ConnInfo{
		Addr:       host.Addr,
		Port:       int(host.Port),
		User:       host.User,
		AuthMode:   host.AuthMode,
		Password:   host.Password,
		PrivateKey: []byte(host.PrivateKey),
	}
	if len(host.PassPhrase) != 0 {
		connInfo.PassPhrase = []byte(host.PassPhrase)
	}
	return ssh.NewClient(connInfo)
}

func loadContainerTerminalCommand(c *gin.Context) (*terminal.LocalCommand, error) {
	source := c.Query("source")
	switch source {
	case "container":
		initCmd, err := loadContainerInitCmd(c)
		if err != nil {
			return nil, err
		}
		return terminal.NewCommand("docker", initCmd...)
	default:
		return nil, fmt.Errorf("not support such source %s", source)
	}
}

func loadContainerInitCmd(c *gin.Context) ([]string, error) {
	containerID := c.Query("containerid")
	command := c.Query("command")
	user := c.Query("user")
	if cmd.CheckIllegal(user, containerID, command) {
		return nil, fmt.Errorf("the command contains illegal characters. command: %s, user: %s, containerID: %s", command, user, containerID)
	}
	if len(command) == 0 || len(containerID) == 0 {
		return nil, fmt.Errorf("error param of command: %s or containerID: %s", command, containerID)
	}
	commands := []string{"exec", "-it", containerID, command}
	if len(user) != 0 {
		commands = []string{"exec", "-it", "-u", user, containerID, command}
	}

	return commands, nil
}

func wshandleError(ws *websocket.Conn, err error) bool {
	if err != nil {
		global.LOG.Errorf("handler ws faled:, err: %v", err)
		dt := time.Now().Add(time.Second)
		if ctlerr := ws.WriteControl(websocket.CloseMessage, []byte(err.Error()), dt); ctlerr != nil {
			wsData, err := json.Marshal(terminal.WsMsg{
				Type: terminal.WsMsgCmd,
				Data: base64.StdEncoding.EncodeToString([]byte(err.Error())),
			})
			if err != nil {
				_ = ws.WriteMessage(websocket.TextMessage, []byte("{\"type\":\"cmd\",\"data\":\"failed to encoding to json\"}"))
			} else {
				_ = ws.WriteMessage(websocket.TextMessage, wsData)
			}
		}
		return true
	}
	return false
}

var upGrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 16384,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}
