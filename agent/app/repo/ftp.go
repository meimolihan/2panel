package repo

import (
	"github.com/2Panel-dev/2Panel/agent/app/model"
	"github.com/2Panel-dev/2Panel/agent/global"
	"gorm.io/gorm"
)

type FtpRepo struct{}

type IFtpRepo interface {
	Page(limit, offset int, opts ...DBOption) (int64, []model.Ftp, error)
	GetList() ([]model.Ftp, error)
	Get(opts ...DBOption) (model.Ftp, error)
	Create(ftp *model.Ftp) error
	Update(id uint, vars map[string]interface{}) error
	Delete(opts ...DBOption) error

	WithByUser(user string) DBOption
	WithLikeUser(info string) DBOption
}

func NewIFtpRepo() IFtpRepo {
	return &FtpRepo{}
}

func (u *FtpRepo) Get(opts ...DBOption) (model.Ftp, error) {
	var ftp model.Ftp
	db := global.DB
	for _, opt := range opts {
		db = opt(db)
	}
	err := db.First(&ftp).Error
	return ftp, err
}

func (u *FtpRepo) GetList() ([]model.Ftp, error) {
	var ftps []model.Ftp
	err := global.DB.Find(&ftps).Error
	return ftps, err
}

func (u *FtpRepo) Page(page, size int, opts ...DBOption) (int64, []model.Ftp, error) {
	var ftps []model.Ftp
	db := global.DB.Model(&model.Ftp{})
	for _, opt := range opts {
		db = opt(db)
	}
	count := int64(0)
	db.Count(&count)
	err := db.Limit(size).Offset((page - 1) * size).Find(&ftps).Error
	return count, ftps, err
}

func (u *FtpRepo) Create(ftp *model.Ftp) error {
	return global.DB.Create(ftp).Error
}

func (u *FtpRepo) Update(id uint, vars map[string]interface{}) error {
	return global.DB.Model(&model.Ftp{}).Where("id = ?", id).Updates(vars).Error
}

func (u *FtpRepo) Delete(opts ...DBOption) error {
	db := global.DB
	for _, opt := range opts {
		db = opt(db)
	}
	return db.Delete(&model.Ftp{}).Error
}

func (u *FtpRepo) WithByUser(user string) DBOption {
	return func(g *gorm.DB) *gorm.DB {
		return g.Where("user = ?", user)
	}
}

func (u *FtpRepo) WithLikeUser(info string) DBOption {
	return func(g *gorm.DB) *gorm.DB {
		return g.Where("user LIKE ?", "%"+info+"%")
	}
}
