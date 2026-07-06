package v2

import "github.com/2Panel-dev/2Panel/agent/app/service"

type ApiGroup struct {
	BaseApi
}

var ApiGroupApp = new(ApiGroup)

type BaseApi struct{}

var (
	dashboardService = service.NewIDashboardService()

	containerService       = service.NewIContainerService()
	composeTemplateService = service.NewIComposeTemplateService()
	imageRepoService       = service.NewIImageRepoService()
	imageService           = service.NewIImageService()
	dockerService          = service.NewIDockerService()

	cronjobService = service.NewICronjobService()

	fileService        = service.NewIFileService()
	fileHistoryService = service.NewIFileHistoryService()
	fileShareService   = service.NewIFileShareService()
	sshService         = service.NewISSHService()
	firewallService    = service.NewIFirewallService()
	iptablesService    = service.NewIIptablesService()
	monitorService     = service.NewIMonitorService()
	systemService      = service.NewISystemService()

	deviceService   = service.NewIDeviceService()
	fail2banService = service.NewIFail2BanService()
	ftpService      = service.NewIFtpService()
	clamService     = service.NewIClamService()

	settingService      = service.NewISettingService()
	backupService       = service.NewIBackupService()
	backupRecordService = service.NewIBackupRecordService()

	logService      = service.NewILogService()
	snapshotService = service.NewISnapshotService()

	hostService     = service.NewIHostService()
	hostToolService = service.NewIHostToolService()
	taskService     = service.NewITaskService()

	diskService = service.NewIDiskService()

	fullBackupService = service.NewIFullBackupService()
)
