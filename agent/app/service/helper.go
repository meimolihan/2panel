package service

import (
	"context"

	"github.com/2Panel-dev/2Panel/agent/constant"
	"github.com/2Panel-dev/2Panel/agent/global"
	"gorm.io/gorm"
)

func getTxAndContext() (tx *gorm.DB, ctx context.Context) {
	tx = global.DB.Begin()
	ctx = context.WithValue(context.Background(), constant.DB, tx)
	return
}
