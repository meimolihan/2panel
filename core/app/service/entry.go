package service

import "github.com/2Panel-dev/2Panel/core/app/repo"

var (
	commandRepo    = repo.NewICommandRepo()
	settingRepo    = repo.NewISettingRepo()
	backupRepo     = repo.NewIBackupRepo()
	logRepo        = repo.NewILogRepo()
	groupRepo      = repo.NewIGroupRepo()
	upgradeLogRepo = repo.NewIUpgradeLogRepo()

	agentRepo  = repo.NewIAgentRepo()
	scriptRepo = repo.NewIScriptRepo()
)
