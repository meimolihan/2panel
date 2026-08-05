package service

import (
	"fmt"
	"strings"

	"github.com/2panel-dev/2panel/internal/dto"
	"github.com/2panel-dev/2panel/internal/model"
	"github.com/2panel-dev/2panel/internal/repo"
)

var scriptLibraryRepo repo.ScriptLibraryRepo

var scriptService ScriptService

type ScriptService struct{}

func (u *ScriptService) SearchWithPage(search dto.ScriptSearch) (int64, []dto.ScriptInfo, error) {
	var opts []repo.DBOption
	if len(search.Info) != 0 {
		opts = append(opts, repo.WithByInfo(search.Info))
	}
	opts = append(opts, repo.WithOrderBy("created_at", "desc"))
	total, scripts, err := scriptLibraryRepo.Page(search.Page.Page, search.Page.PageSize, opts...)
	if err != nil {
		return 0, nil, err
	}
	items := make([]dto.ScriptInfo, 0)
	for _, script := range scripts {
		items = append(items, dto.ScriptInfo{
			ID:          script.ID,
			Name:        script.Name,
			Description: script.Description,
			Script:      script.Script,
			CreatedAt:   script.CreatedAt,
		})
	}
	return total, items, nil
}

func (u *ScriptService) LoadInfo(id uint) (dto.ScriptOperate, error) {
	script, err := scriptLibraryRepo.Get(repo.WithByID(id))
	if err != nil {
		return dto.ScriptOperate{}, err
	}
	return dto.ScriptOperate{
		ID:          script.ID,
		Name:        script.Name,
		Description: script.Description,
		Script:      script.Script,
	}, nil
}

func (u *ScriptService) Create(req dto.ScriptOperate) error {
	name := strings.TrimSpace(req.Name)
	if len(name) == 0 {
		return fmt.Errorf("the script name is required")
	}
	if _, err := scriptLibraryRepo.Get(repo.WithByName(name)); err == nil {
		return fmt.Errorf("the script name already exists")
	}
	script := model.ScriptLibrary{
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		Script:      req.Script,
	}
	return scriptLibraryRepo.Create(&script)
}

func (u *ScriptService) Update(req dto.ScriptOperate) error {
	script, err := scriptLibraryRepo.Get(repo.WithByID(req.ID))
	if err != nil {
		return fmt.Errorf("script not found")
	}
	name := strings.TrimSpace(req.Name)
	if len(name) == 0 {
		return fmt.Errorf("the script name is required")
	}
	if existing, err := scriptLibraryRepo.Get(repo.WithByName(name)); err == nil && existing.ID != script.ID {
		return fmt.Errorf("the script name already exists")
	}
	return scriptLibraryRepo.Update(script.ID, map[string]interface{}{
		"name":        name,
		"description": strings.TrimSpace(req.Description),
		"script":      req.Script,
	})
}

func (u *ScriptService) Delete(ids []uint) error {
	for _, id := range ids {
		if err := scriptLibraryRepo.Delete(repo.WithByID(id)); err != nil {
			return err
		}
	}
	return nil
}

// Options returns the lightweight name list used by the cronjob editor.
func (u *ScriptService) Options() ([]dto.ScriptOption, error) {
	scripts, err := scriptLibraryRepo.List(repo.WithOrderBy("name", "asc"))
	if err != nil {
		return nil, err
	}
	items := make([]dto.ScriptOption, 0)
	for _, script := range scripts {
		items = append(items, dto.ScriptOption{ID: script.ID, Name: script.Name})
	}
	return items, nil
}

// ResolveByName returns the script content for a library script, or empty
// string when the script does not exist.
func (u *ScriptService) ResolveByName(name string) string {
	script, err := scriptLibraryRepo.Get(repo.WithByName(name))
	if err != nil {
		return ""
	}
	return script.Script
}
