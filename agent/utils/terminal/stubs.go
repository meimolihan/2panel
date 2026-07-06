package terminal

import (
	gossh "golang.org/x/crypto/ssh"
)

const WsMsgCmd = "cmd"

type WsMsg struct {
	Type string `json:"type"`
	Data string `json:"data"`
}

type LocalWsSession struct{}

func NewLocalWsSession(cols, rows int, wsConn interface{}, slave *LocalCommand, isWindows bool) (*LocalWsSession, error) {
	return &LocalWsSession{}, nil
}

func (s *LocalWsSession) Start(quitChan chan bool) {}
func (s *LocalWsSession) Close()                   {}

type LogicSshWsSession struct{}

func NewLogicSshWsSession(cols, rows int, sshClient *gossh.Client, wsConn interface{}, cmd string) (*LogicSshWsSession, error) {
	return &LogicSshWsSession{}, nil
}

func (s *LogicSshWsSession) Start(quitChan chan bool) {}
func (s *LogicSshWsSession) Wait(quitChan chan bool)  {}
func (s *LogicSshWsSession) Close()                    {}
