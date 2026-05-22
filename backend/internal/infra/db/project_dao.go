package db

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/ygpkg/yg-go/logs"

	"github.com/insmtx/Leros/backend/types"
)

// ProjectQuery 项目列表查询参数
type ProjectQuery struct {
	Filters []Filter
	OrderBy []string
	OrgID   uint
	Uin     uint
	Offset  int
	Limit   int
	ListAll bool
}

// Filter 过滤条件
type Filter struct {
	Field      string
	Value      []string
	ExactMatch bool
}

// CreateProject 创建项目
func CreateProject(ctx context.Context, db *gorm.DB, project *types.Project) error {
	return db.WithContext(ctx).Create(project).Error
}

// GetProjectByPublicID 根据组织ID和PublicID获取项目
func GetProjectByPublicID(ctx context.Context, db *gorm.DB, orgID uint, publicID string) (*types.Project, error) {
	var entity types.Project
	err := db.WithContext(ctx).Where("org_id = ? AND public_id = ?", orgID, publicID).First(&entity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &entity, nil
}

// UpdateProject 更新项目
func UpdateProject(ctx context.Context, db *gorm.DB, project *types.Project) error {
	return db.WithContext(ctx).Save(project).Error
}

// DeleteProject 删除项目（软删除）
func DeleteProject(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Delete(&types.Project{}, id).Error
}

// ListProjectsResponse 项目列表查询结果
type ListProjectsResponse struct {
	Total  int64            `json:"total"`
	Offset int              `json:"offset"`
	Limit  int              `json:"limit"`
	Items  []*types.Project `json:"items"`
}

// ListProjects 查询项目列表，使用 ProjectQuery 作为查询参数
func ListProjects(ctx context.Context, d *gorm.DB, opt *ProjectQuery, ret *ListProjectsResponse) error {
	query := d.WithContext(ctx).Table(types.TableNameProject).
		Where("org_id = ? AND deleted_at IS NULL", opt.OrgID)

	for _, filter := range opt.Filters {
		switch filter.Field {
		case "name":
			if filter.ExactMatch {
				query = query.Where("name IN (?)", filter.Value)
			} else {
				conds := make([]string, len(filter.Value))
				vals := make([]interface{}, len(filter.Value))
				for i, v := range filter.Value {
					conds[i] = "name LIKE ?"
					vals[i] = "%" + v + "%"
				}
				query = query.Where(strings.Join(conds, " OR "), vals...)
			}
		case "status":
			query = query.Where("status IN (?)", filter.Value)
		case "public_id":
			query = query.Where("public_id IN (?)", filter.Value)
		default:
			logs.WarnContextf(ctx, "[project][ListProjects] invalid filter field: %s", filter.Field)
			return fmt.Errorf("invalid filter field: %s", filter.Field)
		}
	}

	if err := query.Count(&ret.Total).Error; err != nil {
		return err
	}
	if ret.Total == 0 {
		return nil
	}

	if len(opt.OrderBy) > 0 {
		query = query.Order(strings.Join(opt.OrderBy, ","))
	} else {
		query = query.Order("created_at DESC")
	}

	query = query.Offset(opt.Offset)
	if !opt.ListAll && opt.Limit > 0 {
		query = query.Limit(opt.Limit)
	} else {
		query = query.Limit(150)
	}

	err := query.Find(&ret.Items).Error
	if err != nil {
		return err
	}
	return nil
}
