package service

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/2Panel-dev/2Panel/agent/app/dto"
	"github.com/2Panel-dev/2Panel/agent/app/model"
	"github.com/2Panel-dev/2Panel/agent/app/repo"
	"github.com/2Panel-dev/2Panel/agent/constant"
	"github.com/2Panel-dev/2Panel/agent/global"
	"github.com/2Panel-dev/2Panel/agent/utils/files"
	"github.com/jinzhu/copier"
	"github.com/google/uuid"
)

type FullBackupService struct{}

type IFullBackupService interface {
	Create(req dto.FullBackupRequest) (*dto.FullBackupInfo, error)
	Restore(req dto.FullRecoverRequest) error
	SearchWithPage(search dto.PageSnapshot) (int64, interface{}, error)
	Delete(id uint, deleteWithFile bool) error
	UpdateDescription(req dto.UpdateDescription) error
}

func NewIFullBackupService() IFullBackupService {
	return &FullBackupService{}
}

func (s *FullBackupService) Create(req dto.FullBackupRequest) (*dto.FullBackupInfo, error) {
	versionItem, _ := settingRepo.Get(settingRepo.WithByKey("SystemVersion"))
	scope := "core"
	if !global.IsMaster {
		scope = "agent"
	}
	name := fmt.Sprintf("2panel-fullbackup-%s-%s-%s", scope, versionItem.Value, time.Now().Format(constant.DateTimeSlimLayout))

	backupDir := global.Dir.LocalBackupDir
	tmpDir := path.Join(backupDir, "tmp", "fullbackup", name)
	if err := os.MkdirAll(tmpDir, constant.DirPerm); err != nil {
		return nil, err
	}

	dataDir := global.Dir.DataDir
	dbDir := global.Dir.DbDir
	logDir := global.Dir.LogDir

	global.LOG.Infof("creating full backup: %s", name)

	dirsToBackup := []string{dataDir, dbDir, logDir}
	for _, dir := range dirsToBackup {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			continue
		}
		destDir := path.Join(tmpDir, filepath.Base(dir))
		if err := files.NewFileOp().CopyDir(dir, destDir); err != nil {
			global.LOG.Errorf("failed to copy %s: %v", dir, err)
		}
	}

	configFiles, _ := filepath.Glob(path.Join(global.CONF.Base.InstallDir, "2panel/conf/*"))
	for _, f := range configFiles {
		dest := path.Join(tmpDir, "conf", filepath.Base(f))
		os.MkdirAll(path.Dir(dest), constant.DirPerm)
		files.NewFileOp().Copy(f, path.Dir(dest))
	}

	tarPath := path.Join(backupDir, "system_snapshot", name+".tar.gz")
	os.MkdirAll(path.Dir(tarPath), constant.DirPerm)

	if err := tarDir(tmpDir, tarPath); err != nil {
		global.LOG.Errorf("failed to create tar: %v", err)
		return nil, err
	}

	os.RemoveAll(tmpDir)

	fileInfo, _ := os.Stat(tarPath)
	size := int64(0)
	if fileInfo != nil {
		size = fileInfo.Size()
	}

	record := &dto.FullBackupInfo{
		Name:      name,
		Status:    constant.StatusSuccess,
		CreatedAt: time.Now(),
		Size:      size,
		Version:   versionItem.Value,
	}

	backupRecord := model.BackupRecord{
		Type:     "full_backup",
		Name:     name,
		FileName: name + ".tar.gz",
		FileDir:  "system_snapshot",
		Status:   constant.StatusSuccess,
		TaskID:   uuid.New().String(),
	}
	if req.DownloadAccountID != 0 {
		backupRecord.DownloadAccountID = req.DownloadAccountID
		backupRecord.SourceAccountIDs = req.SourceAccountIDs
	}
	if err := backupRepo.CreateRecord(&backupRecord); err != nil {
		global.LOG.Errorf("failed to save backup record: %v", err)
	}

	record.ID = backupRecord.ID
	return record, nil
}

func (s *FullBackupService) Restore(req dto.FullRecoverRequest) error {
	record, err := backupRepo.GetRecord(repo.WithByID(req.ID))
	if err != nil {
		return err
	}
	if record == nil || record.ID == 0 {
		return fmt.Errorf("backup record not found")
	}

	backupDir := global.Dir.LocalBackupDir
	tarPath := path.Join(backupDir, record.FileDir, record.FileName)

	if req.ReDownload {
		tarPath = path.Join(backupDir, "tmp", "system", record.FileName)
	}

	if _, err := os.Stat(tarPath); os.IsNotExist(err) {
		return fmt.Errorf("backup file not found: %s", tarPath)
	}

	restoreDir := path.Join(backupDir, "tmp", "fullbackup", "restore_"+time.Now().Format("20060102150405"))
	if err := os.MkdirAll(restoreDir, constant.DirPerm); err != nil {
		return err
	}

	global.LOG.Infof("restoring full backup from: %s", tarPath)
	if err := untar(tarPath, restoreDir); err != nil {
		return fmt.Errorf("failed to extract backup: %v", err)
	}

	entries, _ := os.ReadDir(restoreDir)
	for _, entry := range entries {
		srcPath := path.Join(restoreDir, entry.Name())
		switch entry.Name() {
		case filepath.Base(global.Dir.DataDir):
			global.LOG.Infof("restoring data directory: %s -> %s", srcPath, global.Dir.DataDir)
			os.RemoveAll(global.Dir.DataDir)
			if err := files.NewFileOp().CopyDir(srcPath, global.Dir.DataDir); err != nil {
				return fmt.Errorf("failed to restore data dir: %v", err)
			}
		case filepath.Base(global.Dir.DbDir):
			global.LOG.Infof("restoring db directory: %s -> %s", srcPath, global.Dir.DbDir)
			os.RemoveAll(global.Dir.DbDir)
			if err := files.NewFileOp().CopyDir(srcPath, global.Dir.DbDir); err != nil {
				return fmt.Errorf("failed to restore db dir: %v", err)
			}
		case "conf":
			confDir := path.Join(global.CONF.Base.InstallDir, "2panel/conf")
			global.LOG.Infof("restoring conf: %s -> %s", srcPath, confDir)
			if err := files.NewFileOp().CopyDir(srcPath, confDir); err != nil {
				global.LOG.Errorf("failed to restore conf: %v", err)
			}
		case filepath.Base(global.Dir.LogDir):
			global.LOG.Infof("restoring log directory: %s -> %s", srcPath, global.Dir.LogDir)
			if err := files.NewFileOp().CopyDir(srcPath, global.Dir.LogDir); err != nil {
				global.LOG.Errorf("failed to restore log dir: %v", err)
			}
		}
	}

	os.RemoveAll(restoreDir)
	_ = backupRepo.UpdateRecordByMap(record.ID, map[string]interface{}{"status": constant.StatusSuccess})

	global.LOG.Info("full backup restore completed successfully")
	return nil
}

func (s *FullBackupService) SearchWithPage(search dto.PageSnapshot) (int64, interface{}, error) {
	total, records, err := backupRepo.PageRecord(
		search.Page, search.PageSize,
		repo.WithByType("full_backup"),
		repo.WithOrderDesc("created_at"),
		repo.WithByLikeName(search.Info),
	)
	if err != nil {
		return 0, nil, err
	}
	var datas []dto.FullBackupInfo
	for i := 0; i < len(records); i++ {
		var item dto.FullBackupInfo
		if err := copier.Copy(&item, &records[i]); err != nil {
			return 0, nil, err
		}
		item.Name = records[i].Name
		item.CreatedAt = records[i].CreatedAt
		item.Status = records[i].Status
		item.Message = records[i].Message
		item.Description = records[i].Description
		datas = append(datas, item)
	}
	return total, datas, nil
}

func (s *FullBackupService) Delete(id uint, deleteWithFile bool) error {
	if deleteWithFile {
		record, err := backupRepo.GetRecord(repo.WithByID(id))
		if err == nil && record != nil && record.ID != 0 {
			backupDir := global.Dir.LocalBackupDir
			tarPath := path.Join(backupDir, record.FileDir, record.FileName)
			os.Remove(tarPath)
		}
	}
	return backupRepo.DeleteRecord(context.Background(), repo.WithByID(id))
}

func (s *FullBackupService) UpdateDescription(req dto.UpdateDescription) error {
	return backupRepo.UpdateRecordByMap(req.ID, map[string]interface{}{"description": req.Description})
}

func tarDir(src, dst string) error {
	tarFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer tarFile.Close()

	gw := gzip.NewWriter(tarFile)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	return filepath.Walk(src, func(filePath string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(src, filePath)
		header.Name = relPath
		if header.Name == "." {
			return nil
		}
		if fi.IsDir() {
			header.Name += "/"
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if !fi.IsDir() {
			f, err := os.Open(filePath)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(tw, f)
			return err
		}
		return nil
	})
}

func untar(src, dst string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target := path.Join(dst, header.Name)
		if !strings.HasPrefix(target, filepath.Clean(dst)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path: %s", target)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, constant.DirPerm); err != nil {
				return err
			}
		case tar.TypeReg:
			os.MkdirAll(path.Dir(target), constant.DirPerm)
			fw, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			_, err = io.Copy(fw, tr)
			fw.Close()
			if err != nil {
				return err
			}
		}
	}
	return nil
}
