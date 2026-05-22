package db

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/insmtx/Leros/backend/types"
)

// DigitalAssistantQuery 数字助手列表查询参数
type DigitalAssistantQuery struct {
	PageQuery
	OwnerID *uint
	Status  *string
	Keyword *string
}

// CreateDigitalAssistant 创建数字助手
func CreateDigitalAssistant(ctx context.Context, db *gorm.DB, da *types.DigitalAssistant) error {
	return db.WithContext(ctx).Create(da).Error
}

// GetDigitalAssistantByID 根据ID获取数字助手
func GetDigitalAssistantByID(ctx context.Context, db *gorm.DB, id uint) (*types.DigitalAssistant, error) {
	var entity types.DigitalAssistant
	err := db.WithContext(ctx).First(&entity, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &entity, nil
}

// GetDigitalAssistantByCode 根据Code获取数字助手
func GetDigitalAssistantByCode(ctx context.Context, db *gorm.DB, code string) (*types.DigitalAssistant, error) {
	var entity types.DigitalAssistant
	err := db.WithContext(ctx).Where("code = ?", code).First(&entity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &entity, nil
}

// UpdateDigitalAssistant 更新数字助手
func UpdateDigitalAssistant(ctx context.Context, db *gorm.DB, da *types.DigitalAssistant) error {
	return db.WithContext(ctx).Save(da).Error
}

// DeleteDigitalAssistant 删除数字助手
func DeleteDigitalAssistant(ctx context.Context, db *gorm.DB, id uint) error {
	return db.WithContext(ctx).Delete(&types.DigitalAssistant{}, id).Error
}

// DigitalAssistantCodeExists 检查code是否存在（排除指定ID）
func DigitalAssistantCodeExists(ctx context.Context, db *gorm.DB, code string, excludeID uint) (bool, error) {
	var count int64
	query := db.WithContext(ctx).Model(&types.DigitalAssistant{}).Where("code = ?", code)
	if excludeID > 0 {
		query = query.Where("id != ?", excludeID)
	}
	err := query.Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ListDigitalAssistant 查询数字助手列表
func ListDigitalAssistant(ctx context.Context, db *gorm.DB, opt *DigitalAssistantQuery) ([]*types.DigitalAssistant, int64, error) {
	var entities []*types.DigitalAssistant
	var total int64

	query := db.WithContext(ctx).Model(&types.DigitalAssistant{})

	if opt.OrgID > 0 {
		query = query.Where("org_id = ?", opt.OrgID)
	}
	if opt.OwnerID != nil {
		query = query.Where("owner_id = ?", *opt.OwnerID)
	}
	if opt.Status != nil {
		query = query.Where("status = ?", *opt.Status)
	}
	if opt.Keyword != nil && *opt.Keyword != "" {
		query = query.Where("name LIKE ? OR code LIKE ? OR description LIKE ?", "%"+*opt.Keyword+"%", "%"+*opt.Keyword+"%", "%"+*opt.Keyword+"%")
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	query = query.Order("created_at DESC").Offset(opt.Offset)
	if !opt.ListAll && opt.Limit > 0 {
		query = query.Limit(opt.Limit)
	} else {
		query = query.Limit(150)
	}

	if err := query.Find(&entities).Error; err != nil {
		return nil, 0, err
	}

	return entities, total, nil
}
