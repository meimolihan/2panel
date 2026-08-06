package repo

import (
	"github.com/2panel-dev/2panel/internal/database"
	"github.com/2panel-dev/2panel/internal/model"
	"gorm.io/gorm"
)

type ScriptLibraryRepo struct{}

func WithByInfo(info string) DBOption {
	return func(g *gorm.DB) *gorm.DB {
		if len(info) == 0 {
			return g
		}
		return g.Where("name LIKE ? OR description LIKE ?", "%"+info+"%", "%"+info+"%")
	}
}

func (u *ScriptLibraryRepo) Get(opts ...DBOption) (model.ScriptLibrary, error) {
	var script model.ScriptLibrary
	db := database.DB
	for _, opt := range opts {
		db = opt(db)
	}
	err := db.First(&script).Error
	return script, err
}

func (u *ScriptLibraryRepo) Page(page, size int, opts ...DBOption) (int64, []model.ScriptLibrary, error) {
	var scripts []model.ScriptLibrary
	db := database.DB.Model(&model.ScriptLibrary{})
	for _, opt := range opts {
		db = opt(db)
	}
	var count int64
	if err := db.Count(&count).Error; err != nil {
		return 0, nil, err
	}
	size = normalizePageSize(size)
	if page <= 0 {
		page = 1
	}
	err := db.Limit(size).Offset(size * (page - 1)).Find(&scripts).Error
	return count, scripts, err
}

func (u *ScriptLibraryRepo) List(opts ...DBOption) ([]model.ScriptLibrary, error) {
	var scripts []model.ScriptLibrary
	db := database.DB.Model(&model.ScriptLibrary{})
	for _, opt := range opts {
		db = opt(db)
	}
	err := db.Find(&scripts).Error
	return scripts, err
}

func (u *ScriptLibraryRepo) Create(script *model.ScriptLibrary) error {
	return database.DB.Create(script).Error
}

func (u *ScriptLibraryRepo) Update(id uint, vars map[string]interface{}) error {
	return database.DB.Model(&model.ScriptLibrary{}).Where("id = ?", id).Updates(vars).Error
}

func (u *ScriptLibraryRepo) Delete(opts ...DBOption) error {
	db := database.DB
	for _, opt := range opts {
		db = opt(db)
	}
	return db.Delete(&model.ScriptLibrary{}).Error
}
