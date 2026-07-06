package providers

import (
	"net/http"

	"github.com/2Panel-dev/2Panel/agent/app/model"
	"github.com/gin-gonic/gin"
)

type MultiNodeProvider interface {
	IsXpack() bool
	IsUseCustomApp() bool
	GetImagePrefix() string
	RemoveTamper(website string)
	StartClam(startClam *model.Clam, isUpdate bool) (int, error)
	LoadNodeInfo(isBase bool) (model.NodeInfo, error)

	LoadRequestTransport() *http.Transport
	ValidateCertificate(c *gin.Context) bool
}
