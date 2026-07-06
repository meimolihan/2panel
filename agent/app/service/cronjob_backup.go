package service

import (
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/2Panel-dev/2Panel/agent/app/task"
	"github.com/2Panel-dev/2Panel/agent/buserr"
	"github.com/2Panel-dev/2Panel/agent/i18n"

	"github.com/2Panel-dev/2Panel/agent/app/dto"
	"github.com/2Panel-dev/2Panel/agent/app/model"
	"github.com/2Panel-dev/2Panel/agent/constant"
	"github.com/2Panel-dev/2Panel/agent/global"
	"github.com/2Panel-dev/2Panel/agent/utils/common"
	"github.com/2Panel-dev/2Panel/agent/utils/files"
)

func (u *CronjobService) handleApp(cronjob model.Cronjob, startTime time.Time, taskItem *task.Task) error {
	addSkipTask("App", taskItem)
	return nil
}

func (u *CronjobService) handleWebsite(cronjob model.Cronjob, startTime time.Time, taskItem *task.Task) error {
	addSkipTask("Website", taskItem)
	return nil
}

func (u *CronjobService) handleDatabase(cronjob model.Cronjob, startTime time.Time, taskItem *task.Task) error {
	addSkipTask("Database", taskItem)
	return nil
}

func (u *CronjobService) handleDirectory(cronjob model.Cronjob, startTime time.Time, taskItem *task.Task) error {
	accountMap := NewBackupClientMap(strings.Split(cronjob.SourceAccountIDs, ","))
	if !accountMap[fmt.Sprintf("%d", cronjob.DownloadAccountID)].isOk {
		return buserr.New(i18n.GetMsgWithDetail("LoadBackupFailed", accountMap[fmt.Sprintf("%d", cronjob.DownloadAccountID)].message))
	}
	taskItem.AddSubTaskWithOps(task.GetTaskName(cronjob.SourceDir, task.TaskBackup, task.TaskScopeCronjob), func(task *task.Task) error {
		fileName := fmt.Sprintf("%s.tar.gz", startTime.Format(constant.DateTimeSlimLayout)+common.RandStrAndNum(2))
		if cronjob.IsDir || len(strings.Split(cronjob.SourceDir, ",")) == 1 {
			fileName = loadFileName(cronjob.SourceDir)
		}
		fileName = simplifiedFileName(fileName)
		backupDir := path.Join(global.Dir.LocalBackupDir, fmt.Sprintf("tmp/%s/%s", cronjob.Type, cronjob.Name))

		fileOp := files.NewFileOp()
		if cronjob.IsDir {
			taskItem.Logf("Dir: %s, Excludes: %s", cronjob.SourceDir, cronjob.ExclusionRules)
			if err := fileOp.TarGzCompressPro(true, cronjob.SourceDir, path.Join(backupDir, fileName), cronjob.Secret, cronjob.ExclusionRules); err != nil {
				return err
			}
		} else {
			taskItem.Logf("Files: %s", cronjob.SourceDir)
			fileLists := strings.Split(cronjob.SourceDir, ",")
			if err := fileOp.TarGzFilesWithCompressPro(fileLists, path.Join(backupDir, fileName), cronjob.Secret); err != nil {
				return err
			}
		}
		var record model.BackupRecord
		record.Status = constant.StatusSuccess
		record.From = "cronjob"
		record.Type = "directory"
		record.CronjobID = cronjob.ID
		record.Name = cronjob.Name
		record.DownloadAccountID, record.SourceAccountIDs = cronjob.DownloadAccountID, cronjob.SourceAccountIDs

		src := path.Join(backupDir, fileName)
		dst := strings.TrimPrefix(src, global.Dir.LocalBackupDir+"/tmp/")
		if err := uploadWithMap(*task, accountMap, src, dst, cronjob.SourceAccountIDs, cronjob.DownloadAccountID, cronjob.RetryTimes); err != nil {
			return err
		}
		record.FileDir = path.Dir(dst)
		record.FileName = fileName
		if err := backupRepo.CreateRecord(&record); err != nil {
			return err
		}
		u.removeExpiredBackup(cronjob, accountMap, record)
		return nil
	}, nil, int(cronjob.RetryTimes), time.Duration(cronjob.Timeout)*time.Second)
	return nil
}

func (u *CronjobService) handleSystemLog(cronjob model.Cronjob, startTime time.Time, taskItem *task.Task) error {
	accountMap := NewBackupClientMap(strings.Split(cronjob.SourceAccountIDs, ","))
	if !accountMap[fmt.Sprintf("%d", cronjob.DownloadAccountID)].isOk {
		return buserr.New(i18n.GetMsgWithDetail("LoadBackupFailed", accountMap[fmt.Sprintf("%d", cronjob.DownloadAccountID)].message))
	}
	taskItem.AddSubTaskWithOps(task.GetTaskName(i18n.GetMsgByKey("SystemLog"), task.TaskBackup, task.TaskScopeCronjob), func(task *task.Task) error {
		nameItem := startTime.Format(constant.DateTimeSlimLayout) + common.RandStrAndNum(5)
		fileName := fmt.Sprintf("system_log_%s.tar.gz", nameItem)
		backupDir := path.Join(global.Dir.LocalBackupDir, "tmp/log", nameItem)
		if err := handleBackupLogs(taskItem, backupDir, fileName, cronjob.Secret); err != nil {
			return err
		}
		var record model.BackupRecord
		record.Status = constant.StatusSuccess
		record.From = "cronjob"
		record.Type = "log"
		record.CronjobID = cronjob.ID
		record.Name = cronjob.Name
		record.DownloadAccountID, record.SourceAccountIDs = cronjob.DownloadAccountID, cronjob.SourceAccountIDs

		src := path.Join(path.Dir(backupDir), fileName)
		dst := strings.TrimPrefix(src, global.Dir.LocalBackupDir+"/tmp/")
		if err := uploadWithMap(*task, accountMap, src, dst, cronjob.SourceAccountIDs, cronjob.DownloadAccountID, cronjob.RetryTimes); err != nil {
			return err
		}
		record.FileDir = path.Dir(dst)
		record.FileName = fileName
		if err := backupRepo.CreateRecord(&record); err != nil {
			return err
		}
		u.removeExpiredBackup(cronjob, accountMap, record)
		return nil
	}, nil, int(cronjob.RetryTimes), time.Duration(cronjob.Timeout)*time.Second)
	return nil
}

func (u *CronjobService) handleSnapshot(cronjob model.Cronjob, jobRecord model.JobRecords, taskItem *task.Task) error {
	accountMap := NewBackupClientMap(strings.Split(cronjob.SourceAccountIDs, ","))
	if !accountMap[fmt.Sprintf("%d", cronjob.DownloadAccountID)].isOk {
		return buserr.New(i18n.GetMsgWithDetail("LoadBackupFailed", accountMap[fmt.Sprintf("%d", cronjob.DownloadAccountID)].message))
	}
	var record model.BackupRecord
	record.Status = constant.StatusSuccess
	record.From = "cronjob"
	record.Type = "snapshot"
	record.CronjobID = cronjob.ID
	record.Name = cronjob.Name
	record.DownloadAccountID, record.SourceAccountIDs = cronjob.DownloadAccountID, cronjob.SourceAccountIDs
	record.FileDir = "system_snapshot"

	versionItem, _ := settingRepo.Get(settingRepo.WithByKey("SystemVersion"))
	scope := "core"
	if !global.IsMaster {
		scope = "agent"
	}

	itemData, err := loadSnapWithRule(cronjob)
	if err != nil {
		return err
	}
	req := dto.SnapshotCreate{
		Name:    fmt.Sprintf("snapshot-1panel-%s-%s-linux-%s-%s", scope, versionItem.Value, loadOs(), jobRecord.StartTime.Format(constant.DateTimeSlimLayout)+common.RandStrAndNum(5)),
		Secret:  cronjob.Secret,
		TaskID:  jobRecord.TaskID,
		Timeout: cronjob.Timeout,

		SourceAccountIDs:  record.SourceAccountIDs,
		DownloadAccountID: cronjob.DownloadAccountID,
		AppData:           itemData.AppData,
		PanelData:         itemData.PanelData,
		BackupData:        itemData.BackupData,
		WithDockerConf:    true,
		WithMonitorData:   true,
		WithLoginLog:      true,
		WithOperationLog:  true,
		WithSystemLog:     true,
		WithTaskLog:       true,
		IgnoreFiles:       strings.Split(cronjob.ExclusionRules, ","),
	}

	if err := NewISnapshotService().SnapshotCreate(taskItem, req, jobRecord.ID, cronjob.RetryTimes); err != nil {
		return err
	}
	record.FileName = req.Name + ".tar.gz"

	if err := backupRepo.CreateRecord(&record); err != nil {
		global.LOG.Errorf("save backup record failed, err: %v", err)
		return err
	}
	u.removeExpiredBackup(cronjob, accountMap, record)
	return nil
}

func loadAppsForJob(cronjob model.Cronjob) []interface{} {
	return nil
}

type DatabaseHelper struct {
	ID       uint
	DBType   string
	Database string
	Name     string
	Args     []string
}

func addSkipTask(source string, taskItem *task.Task) {
	taskItem.AddSubTask(task.GetTaskName(i18n.GetMsgByKey(source), task.TaskBackup, task.TaskScopeCronjob), func(task *task.Task) error {
		taskItem.Log(i18n.GetMsgByKey("NoSuchResource"))
		return nil
	}, nil)
}

func loadDbsForJob(cronjob model.Cronjob) []DatabaseHelper {
	return nil
}

func loadWebsForJob(cronjob model.Cronjob) []interface{} {
	return nil
}

func handleBackupLogs(taskItem *task.Task, targetDir, fileName string, secret string) error {
	return nil
}

func loadSnapWithRule(cronjob model.Cronjob) (dto.SnapshotData, error) {
	itemData, err := NewISnapshotService().LoadSnapshotData()
	if err != nil {
		return itemData, err
	}

	if len(cronjob.SnapshotRule) == 0 {
		return itemData, nil
	}
	var snapRule dto.SnapshotRule
	if err := json.Unmarshal([]byte(cronjob.SnapshotRule), &snapRule); err != nil {
		return itemData, err
	}
	_ = snapRule

	return itemData, nil
}

func loadFileName(src string) string {
	dirs := strings.Split(filepath.ToSlash(src), "/")
	var keyPart string
	if len(dirs) >= 3 {
		keyPart = filepath.Join(dirs[len(dirs)-3], dirs[len(dirs)-2], dirs[len(dirs)-1])
	}
	cleanName := strings.ReplaceAll(keyPart, string(filepath.Separator), "_")
	timestamp := time.Now().Format(constant.DateTimeSlimLayout)
	return fmt.Sprintf("%s_%s_%s.tar.gz", cleanName, timestamp, common.RandStrAndNum(2))
}

func simplifiedFileName(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, ":", "_")
	name = strings.ReplaceAll(name, "*", "_")
	name = strings.ReplaceAll(name, "?", "_")
	name = strings.ReplaceAll(name, "\"", "_")
	name = strings.ReplaceAll(name, "<", "_")
	name = strings.ReplaceAll(name, ">", "_")
	name = strings.ReplaceAll(name, "|", "_")
	return name
}

func cleanAccountMap(accountMap map[string]backupClientHelper) {
	for key, val := range accountMap {
		val.hasBackup = false
		accountMap[key] = val
	}
}
