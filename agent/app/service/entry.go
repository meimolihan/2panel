package service

import "github.com/2Panel-dev/2Panel/agent/app/repo"

var (
	imageRepoRepo = repo.NewIImageRepoRepo()
	composeRepo   = repo.NewIComposeTemplateRepo()

	scriptRepo  = repo.NewIScriptRepo()
	cronjobRepo = repo.NewICronjobRepo()

	hostRepo    = repo.NewIHostRepo()
	ftpRepo     = repo.NewIFtpRepo()
	clamRepo    = repo.NewIClamRepo()
	monitorRepo = repo.NewIMonitorRepo()

	settingRepo = repo.NewISettingRepo()
	backupRepo  = repo.NewIBackupRepo()

	snapshotRepo = repo.NewISnapshotRepo()

	fileShareRepo = repo.NewIFileShareRepo()

	taskRepo = repo.NewITaskRepo()
)
