package migration

import (
	"github.com/2Panel-dev/2Panel/agent/global"
	"github.com/2Panel-dev/2Panel/agent/init/migration/migrations"

	"github.com/go-gormigrate/gormigrate/v2"
)

func Init() {
	InitAgentDB()
	InitTaskDB()
	InitAlertDB()
	global.LOG.Info("Migration run successfully")
}

func InitAgentDB() {
	m := gormigrate.New(global.DB, gormigrate.DefaultOptions, []*gormigrate.Migration{
		migrations.AddTable,
		migrations.AddMonitorTable,
		migrations.InitSetting,
		migrations.InitImageRepo,
		migrations.AddTaskTable,
		migrations.InitBackup,
		migrations.AddSnapshotRule,
		migrations.AddSnapshotIgnore,
		migrations.InitLocalSSHConn,
		migrations.InitLocalSSHShow,
		migrations.AddTimeoutForClam,
		migrations.UpdateCronjobSpec,
		migrations.UpdateMonitorInterval,
		migrations.AddMonitorProcess,
		migrations.UpdateCronJob,
		migrations.AddIptablesFilterRuleTable,
		migrations.AddCommonDescription,
		migrations.InitIptablesStatus,
		migrations.InitFirewallPortWhiteList,
		migrations.InitPingStatus,
		migrations.AddCronjobArgs,
		migrations.AddEditionSetting,
		migrations.AddFileShareTable,
		migrations.AddFileHistoryTable,
		migrations.AddGPUMonitor,
	})
	if err := m.Migrate(); err != nil {
		global.LOG.Error(err)
		panic(err)
	}
}

func InitTaskDB() {
	m := gormigrate.New(global.TaskDB, gormigrate.DefaultOptions, []*gormigrate.Migration{
		migrations.AddTaskTable,
	})
	if err := m.Migrate(); err != nil {
		global.LOG.Error(err)
		panic(err)
	}
}

func InitAlertDB() {
}
