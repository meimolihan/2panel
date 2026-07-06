package cache

import (
	"github.com/2Panel-dev/2Panel/agent/global"
	cachedb "github.com/2Panel-dev/2Panel/agent/init/cache/db"
)

func Init() {
	global.CACHE = cachedb.NewCacheDB()
}
