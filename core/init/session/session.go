package session

import (
	"github.com/2Panel-dev/2Panel/core/global"
	"github.com/2Panel-dev/2Panel/core/init/session/psession"
)

func Init() {
	global.SESSION = psession.NewPSession("")
	global.LOG.Info("init in-memory session successfully")
}
