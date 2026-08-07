package service

import (
	"fmt"
	"strings"

	"github.com/2panel-dev/2panel/internal/dto"
	"github.com/2panel-dev/2panel/internal/model"
	"github.com/2panel-dev/2panel/internal/repo"
)

var groupRepo repo.GroupRepo

var groupService GroupService

type GroupService struct{}

func (u *GroupService) List(req dto.GroupSearch) ([]dto.GroupInfo, error) {
	options := []repo.DBOption{
		repo.WithOrderBy("is_default", "desc"),
		repo.WithOrderBy("created_at", "asc"),
	}
	if len(req.Type) != 0 {
		options = append(options, repo.WithByType(req.Type))
	}
	groups, err := groupRepo.GetList(options...)
	if err != nil {
		return nil, err
	}
	items := make([]dto.GroupInfo, 0, len(groups))
	for _, group := range groups {
		items = append(items, dto.GroupInfo{
			ID:        group.ID,
			Name:      group.Name,
			Type:      group.Type,
			IsDefault: group.IsDefault,
		})
	}
	return items, nil
}

func (u *GroupService) Create(req dto.GroupCreate) error {
	name := strings.TrimSpace(req.Name)
	if len(name) == 0 {
		return fmt.Errorf("the group name is required")
	}
	if len(req.Type) == 0 {
		return fmt.Errorf("the group type is required")
	}
	if group, err := groupRepo.Get(repo.WithByName(name), repo.WithByType(req.Type)); err == nil && group.ID != 0 {
		return fmt.Errorf("the group name already exists")
	}
	return groupRepo.Create(&model.Group{Name: name, Type: req.Type})
}

func (u *GroupService) Update(req dto.GroupUpdate) error {
	group, err := groupRepo.Get(repo.WithByID(req.ID))
	if err != nil {
		return fmt.Errorf("group not found")
	}
	vars := map[string]interface{}{
		"is_default": req.IsDefault,
	}
	if name := strings.TrimSpace(req.Name); len(name) != 0 {
		if existing, err := groupRepo.Get(repo.WithByName(name), repo.WithByType(group.Type)); err == nil && existing.ID != group.ID {
			return fmt.Errorf("the group name already exists")
		}
		vars["name"] = name
	}
	if req.IsDefault {
		if err := groupRepo.CancelDefault(group.Type); err != nil {
			return err
		}
	}
	return groupRepo.Update(req.ID, vars)
}

func (u *GroupService) Delete(id uint) error {
	group, err := groupRepo.Get(repo.WithByID(id))
	if err != nil {
		return fmt.Errorf("group not found")
	}
	if group.Type == "script" {
		inUse, err := scriptGroupInUse(id)
		if err != nil {
			return err
		}
		if inUse {
			return fmt.Errorf("the group is in use by scripts and cannot be deleted")
		}
	}
	if group.IsDefault {
		return fmt.Errorf("the default group cannot be deleted")
	}
	return groupRepo.Delete(repo.WithByID(id))
}

// scriptGroupInUse reports whether any script in the library references the
// given group id through its comma-separated Groups field.
func scriptGroupInUse(groupID uint) (bool, error) {
	scripts, err := scriptLibraryRepo.List()
	if err != nil {
		return false, err
	}
	for _, script := range scripts {
		for _, idItem := range strings.Split(script.Groups, ",") {
			id := parseUint(idItem)
			if id != 0 && id == groupID {
				return true, nil
			}
		}
	}
	return false, nil
}
