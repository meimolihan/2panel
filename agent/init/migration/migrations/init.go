package migrations

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path"
	"strconv"
	"strings"

	"github.com/2Panel-dev/2Panel/agent/app/dto"
	"github.com/2Panel-dev/2Panel/agent/app/model"
	"github.com/2Panel-dev/2Panel/agent/app/service"
	"github.com/2Panel-dev/2Panel/agent/constant"
	"github.com/2Panel-dev/2Panel/agent/global"
	"github.com/2Panel-dev/2Panel/agent/utils/common"
	"github.com/2Panel-dev/2Panel/agent/utils/copier"
	"github.com/2Panel-dev/2Panel/agent/utils/encrypt"
	"github.com/2Panel-dev/2Panel/agent/utils/firewall"
	"github.com/2Panel-dev/2Panel/agent/utils/ssh"
	"github.com/2Panel-dev/2Panel/agent/utils/xpack"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

var AddTable = &gormigrate.Migration{
	ID: "20250930-add-table",
	Migrate: func(tx *gorm.DB) error {
		return tx.AutoMigrate(
			&model.BackupAccount{},
			&model.BackupRecord{},
			&model.Clam{},
			&model.ClamRecord{},
			&model.ComposeTemplate{},
			&model.Cronjob{},
			&model.FileShare{},
			&model.Firewall{},
			&model.Host{},
			&model.Ftp{},
			&model.ImageRepo{},
			&model.ScriptLibrary{},
			&model.JobRecords{},
			&model.MonitorBase{},
			&model.MonitorIO{},
			&model.MonitorNetwork{},
			&model.Setting{},
			&model.Snapshot{},
			&model.Compose{},
		)
	},
}

var AddMonitorTable = &gormigrate.Migration{
	ID: "20240813-add-monitor-table",
	Migrate: func(tx *gorm.DB) error {
		return global.MonitorDB.AutoMigrate(
			&model.MonitorBase{},
			&model.MonitorIO{},
			&model.MonitorNetwork{},
		)
	},
}

var InitSetting = &gormigrate.Migration{
	ID: "20240722-init-setting",
	Migrate: func(tx *gorm.DB) error {
		global.CONF.Base.EncryptKey = common.RandStr(16)
		nodeInfo, err := xpack.MultiNodeProvider.LoadNodeInfo(true)
		if err != nil {
			return err
		}
		if err := tx.Create(&model.Setting{Key: "BaseDir", Value: nodeInfo.BaseDir}).Error; err != nil {
			return err
		}
		itemKey, _ := encrypt.StringEncrypt(nodeInfo.ServerKey)
		if err := tx.Create(&model.Setting{Key: "ServerKey", Value: itemKey}).Error; err != nil {
			return err
		}
		itemCrt, _ := encrypt.StringEncrypt(nodeInfo.ServerCrt)
		if err := tx.Create(&model.Setting{Key: "ServerCrt", Value: itemCrt}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Setting{Key: "NodeScope", Value: nodeInfo.Scope}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Setting{Key: "NodePort", Value: fmt.Sprintf("%v", nodeInfo.NodePort)}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Setting{Key: "SystemVersion", Value: nodeInfo.Version}).Error; err != nil {
			return err
		}

		if err := tx.Create(&model.Setting{Key: "EncryptKey", Value: global.CONF.Base.EncryptKey}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Setting{Key: "DockerSockPath", Value: "unix:///var/run/docker.sock"}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Setting{Key: "SystemStatus", Value: "Free"}).Error; err != nil {
			return err
		}
		lang := common.LoadParamsWithoutPanic("LANGUAGE")
		if err := tx.Create(&model.Setting{Key: "Language", Value: lang}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Setting{Key: "SystemIP", Value: ""}).Error; err != nil {
			return err
		}

		if err := tx.Create(&model.Setting{Key: "LocalTime", Value: ""}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Setting{Key: "TimeZone", Value: common.LoadTimeZoneByCmd()}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Setting{Key: "NtpSite", Value: "pool.ntp.org"}).Error; err != nil {
			return err
		}

		if err := tx.Create(&model.Setting{Key: "LastCleanTime", Value: ""}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Setting{Key: "LastCleanSize", Value: ""}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Setting{Key: "LastCleanData", Value: ""}).Error; err != nil {
			return err
		}

		if err := tx.Create(&model.Setting{Key: "DefaultNetwork", Value: "all"}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Setting{Key: "MonitorStatus", Value: constant.StatusEnable}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Setting{Key: "MonitorStoreDays", Value: "7"}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Setting{Key: "MonitorInterval", Value: "5"}).Error; err != nil {
			return err
		}

		if err := tx.Create(&model.Setting{Key: "ProxyType", Value: ""}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Setting{Key: "ProxyUrl", Value: ""}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Setting{Key: "ProxyPort", Value: ""}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Setting{Key: "ProxyUser", Value: ""}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Setting{Key: "ProxyPasswd", Value: ""}).Error; err != nil {
			return err
		}

		if err := tx.Create(&model.Setting{Key: "AppStoreVersion", Value: ""}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Setting{Key: "AppStoreSyncStatus", Value: "SyncSuccess"}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Setting{Key: "AppStoreLastModified", Value: "0"}).Error; err != nil {
			return err
		}

		if err := tx.Create(&model.Setting{Key: "FileRecycleBin", Value: constant.StatusEnable}).Error; err != nil {
			return err
		}

		if err := tx.Create(&model.Setting{Key: "LocalSSHConn", Value: ""}).Error; err != nil {
			return err
		}

		return nil
	},
}

var InitImageRepo = &gormigrate.Migration{
	ID: "20240722-init-imagerepo",
	Migrate: func(tx *gorm.DB) error {
		item := &model.ImageRepo{
			Name:        "Docker Hub",
			Protocol:    "https",
			DownloadUrl: "docker.io",
			Status:      constant.StatusSuccess,
		}
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		return nil
	},
}

var AddTaskTable = &gormigrate.Migration{
	ID: "20241226-add-task",
	Migrate: func(tx *gorm.DB) error {
		return tx.AutoMigrate(
			&model.Task{},
		)
	},
}

var InitBackup = &gormigrate.Migration{
	ID: "20241226-init-backup",
	Migrate: func(tx *gorm.DB) error {
		if err := tx.Create(&model.BackupAccount{
			Name:       "localhost",
			Type:       "LOCAL",
			BackupPath: path.Join(global.Dir.DataDir, "backup"),
		}).Error; err != nil {
			return err
		}
		return nil
	},
}

var AddSnapshotRule = &gormigrate.Migration{
	ID: "20250703-add-snapshot-rule",
	Migrate: func(tx *gorm.DB) error {
		return tx.AutoMigrate(
			&model.Cronjob{},
		)
	},
}

var AddSnapshotIgnore = &gormigrate.Migration{
	ID: "20250716-add-snapshot-ignore",
	Migrate: func(tx *gorm.DB) error {
		return tx.AutoMigrate(
			&model.Snapshot{},
		)
	},
}

var InitLocalSSHConn = &gormigrate.Migration{
	ID: "20250905-init-local-ssh",
	Migrate: func(tx *gorm.DB) error {
		itemPath := ""
		currentInfo, _ := user.Current()
		if len(currentInfo.HomeDir) == 0 {
			itemPath = "/root/.ssh/id_ed25519_2panel"
		} else {
			itemPath = path.Join(currentInfo.HomeDir, ".ssh/id_ed25519_2panel")
		}
		if _, err := os.Stat(itemPath); err != nil {
			_ = service.NewISSHService().CreateRootCert(dto.RootCertOperate{EncryptionMode: "ed25519", Name: "id_ed25519_2panel", Description: "1Panel Terminal"})
		}
		privateKey, _ := os.ReadFile(itemPath)
		connWithKey := ssh.ConnInfo{
			Addr:       "127.0.0.1",
			User:       "root",
			Port:       22,
			AuthMode:   "key",
			PrivateKey: privateKey,
		}
		if _, err := ssh.NewClient(connWithKey); err != nil {
			return nil
		}
		var conn model.LocalConnInfo
		_ = copier.Copy(&conn, &connWithKey)
		conn.PrivateKey = string(privateKey)
		conn.PassPhrase = ""
		localConn, _ := json.Marshal(&conn)
		connAfterEncrypt, _ := encrypt.StringEncrypt(string(localConn))
		if err := tx.Model(&model.Setting{}).Where("key = ?", "LocalSSHConn").Updates(map[string]interface{}{"value": connAfterEncrypt}).Error; err != nil {
			return err
		}
		return nil
	},
}

var InitLocalSSHShow = &gormigrate.Migration{
	ID: "20250908-init-local-ssh-show",
	Migrate: func(tx *gorm.DB) error {
		if err := tx.Create(&model.Setting{Key: "LocalSSHConnShow", Value: constant.StatusEnable}).Error; err != nil {
			return err
		}
		return nil
	},
}

var InitRecordStatus = &gormigrate.Migration{
	ID: "20250910-init-record-status",
	Migrate: func(tx *gorm.DB) error {
		if err := tx.AutoMigrate(&model.BackupRecord{}); err != nil {
			return err
		}
		if err := tx.Model(&model.BackupRecord{}).Where("1 == 1").Updates(map[string]interface{}{"status": constant.StatusSuccess}).Error; err != nil {
			return err
		}
		return nil
	},
}

var AddTimeoutForClam = &gormigrate.Migration{
	ID: "20250922-add-timeout-for-clam",
	Migrate: func(tx *gorm.DB) error {
		if err := tx.AutoMigrate(&model.Clam{}); err != nil {
			return err
		}
		if err := tx.Model(&model.Clam{}).Where("1 == 1").Updates(map[string]interface{}{"timeout": 18000}).Error; err != nil {
			return err
		}
		return nil
	},
}

var UpdateCronjobSpec = &gormigrate.Migration{
	ID: "20250925-update-cronjob-spec",
	Migrate: func(tx *gorm.DB) error {
		var cronjobs []model.Cronjob
		if err := tx.Where("1 == 1").Find(&cronjobs).Error; err != nil {
			return err
		}
		for _, item := range cronjobs {
			if !strings.Contains(item.Spec, ",") {
				continue
			}
			if err := tx.Model(&model.Cronjob{}).Where("id = ?", item.ID).Updates(
				map[string]interface{}{"spec": strings.ReplaceAll(item.Spec, ",", "&&")}).Error; err != nil {
				return err
			}
		}
		return nil
	},
}

var UpdateMonitorInterval = &gormigrate.Migration{
	ID: "20251026-update-monitor-interval",
	Migrate: func(tx *gorm.DB) error {
		var monitorInterval model.Setting
		if err := tx.Where("key = ?", "MonitorInterval").First(&monitorInterval).Error; err != nil {
			return err
		}
		interval, _ := strconv.Atoi(monitorInterval.Value)
		if interval == 0 {
			interval = 300
		}
		if err := tx.Model(&model.Setting{}).
			Where("key = ?", "MonitorInterval").
			Updates(map[string]interface{}{"value": fmt.Sprintf("%v", interval*60)}).
			Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Setting{Key: "DefaultIO", Value: "all"}).Error; err != nil {
			return err
		}
		return nil
	},
}

var AddIptablesFilterRuleTable = &gormigrate.Migration{
	ID: "20251106-add-iptables-filter-rule-table",
	Migrate: func(tx *gorm.DB) error {
		if err := tx.AutoMigrate(&model.Firewall{}); err != nil {
			return err
		}
		var firewalls []model.Firewall
		_ = tx.Where("1 = 1").Find(&firewalls).Error

		firewallType := ""
		client, err := firewall.NewFirewallClient()
		if err == nil {
			firewallType = client.Name()
		}
		for _, item := range firewalls {
			if err := tx.Model(&model.Firewall{}).
				Where("id = ?", item.ID).
				Updates(map[string]interface{}{"dst_port": item.Port, "src_ip": item.Address, "firewall_type": firewallType}); err != nil {
				global.LOG.Errorf("update firewall failed, err: %v", err)
			}
		}
		return nil
	},
}

var AddMonitorProcess = &gormigrate.Migration{
	ID: "20251030-add-monitor-process",
	Migrate: func(tx *gorm.DB) error {
		return global.MonitorDB.AutoMigrate(&model.MonitorBase{})
	},
}

var UpdateCronJob = &gormigrate.Migration{
	ID: "20251105-update-cronjob",
	Migrate: func(tx *gorm.DB) error {
		return tx.AutoMigrate(&model.Cronjob{})
	},
}

var AddCommonDescription = &gormigrate.Migration{
	ID: "20251117-add-common-description",
	Migrate: func(tx *gorm.DB) error {
		return tx.AutoMigrate(&model.CommonDescription{})
	},
}

var AddGPUMonitor = &gormigrate.Migration{
	ID: "20251122-add-gpu-monitor",
	Migrate: func(tx *gorm.DB) error {
		return global.GPUMonitorDB.AutoMigrate(&model.MonitorGPU{})
	},
}

var InitIptablesStatus = &gormigrate.Migration{
	ID: "20251201-init-iptables-status",
	Migrate: func(tx *gorm.DB) error {
		if err := tx.Create(&model.Setting{Key: "IptablesStatus", Value: constant.StatusDisable}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Setting{Key: "IptablesForwardStatus", Value: constant.StatusDisable}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Setting{Key: "IptablesInputStatus", Value: constant.StatusDisable}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Setting{Key: "IptablesOutputStatus", Value: constant.StatusDisable}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Setting{Key: constant.FirewallPortWhiteList, Value: constant.FirewallPortWhiteListValue}).Error; err != nil {
			return err
		}
		return nil
	},
}

var InitFirewallPortWhiteList = &gormigrate.Migration{
	ID: "20260601-init-firewall-port-whitelist",
	Migrate: func(tx *gorm.DB) error {
		var setting model.Setting
		if err := tx.Where("key = ?", constant.FirewallPortWhiteList).First(&setting).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return tx.Create(&model.Setting{Key: constant.FirewallPortWhiteList, Value: constant.FirewallPortWhiteListValue}).Error
			}
			return err
		}
		return nil
	},
}

var InitPingStatus = &gormigrate.Migration{
	ID: "20251201-init-ping-status",
	Migrate: func(tx *gorm.DB) error {
		status := firewall.LoadPingStatus()
		if err := tx.Create(&model.Setting{Key: "BanPing", Value: status}).Error; err != nil {
			return err
		}
		return nil
	},
}

var AddCronjobArgs = &gormigrate.Migration{
	ID: "20260106-add-cronjob-args",
	Migrate: func(tx *gorm.DB) error {
		return tx.AutoMigrate(&model.Cronjob{})
	},
}

var AddEditionSetting = &gormigrate.Migration{
	ID: "20260224-add-edition-setting",
	Migrate: func(tx *gorm.DB) error {
		var setting model.Setting
		edition := common.LoadParamsWithoutPanic("PANEL_EDITION")
		if err := tx.Where("key = ?", "Edition").First(&setting).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return tx.Create(&model.Setting{Key: "Edition", Value: edition}).Error
			}
			return err
		}
		if setting.Value == "" {
			return tx.Model(&model.Setting{}).Where("key = ?", "Edition").Update("value", edition).Error
		}
		return nil
	},
}

var AddFileShareTable = &gormigrate.Migration{
	ID: "20260410-add-file-share-table",
	Migrate: func(tx *gorm.DB) error {
		return tx.AutoMigrate(&model.FileShare{})
	},
}

var AddFileHistoryTable = &gormigrate.Migration{
	ID: "20260414-add-file-history-table",
	Migrate: func(tx *gorm.DB) error {
		if err := tx.AutoMigrate(&model.FileHistory{}); err != nil {
			return err
		}
		defaultRows := []model.Setting{
			{Key: "FileHistoryStatus", Value: constant.StatusEnable},
			{Key: "FileHistoryMaxPerPath", Value: "20"},
			{Key: "FileHistoryDiskQuotaMB", Value: "1024"},
		}
		for i := range defaultRows {
			var exist model.Setting
			if err := tx.Where("`key` = ?", defaultRows[i].Key).First(&exist).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					if err := tx.Create(&defaultRows[i]).Error; err != nil {
						return err
					}
				} else {
					return err
				}
			}
		}
		return nil
	},
}
