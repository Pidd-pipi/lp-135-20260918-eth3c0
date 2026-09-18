package repository

import (
	"errors"
	"fmt"

	"github.com/givetrack/givetrack/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RefundRepository 退款申请数据访问。
type RefundRepository struct {
	db *gorm.DB
}

func NewRefundRepository(db *gorm.DB) *RefundRepository {
	return &RefundRepository{db: db}
}

func (r *RefundRepository) Create(a *model.RefundApplication) error {
	if err := r.db.Create(a).Error; err != nil {
		return fmt.Errorf("create refund application: %w", err)
	}
	return nil
}

// FindByDonationID 按捐赠记录查询退款申请（不存在返回 nil, nil）。
func (r *RefundRepository) FindByDonationID(donationID uint) (*model.RefundApplication, error) {
	var a model.RefundApplication
	err := r.db.Where("donation_id = ?", donationID).First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find refund by donation: %w", err)
	}
	return &a, nil
}

// FindByDonationIDForUpdate 行级锁查询（须在事务内调用），用于串行化并发申请。
func (r *RefundRepository) FindByDonationIDForUpdate(donationID uint) (*model.RefundApplication, error) {
	var a model.RefundApplication
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("donation_id = ?", donationID).First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find refund by donation for update: %w", err)
	}
	return &a, nil
}

// FindByID 查询退款申请详情并预load关联。
func (r *RefundRepository) FindByID(id uint) (*model.RefundApplication, error) {
	var a model.RefundApplication
	err := r.db.Preload("Donation").Preload("Donation.Project").Preload("User").Preload("Reviewer").First(&a, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find refund by id: %w", err)
	}
	return &a, nil
}

// FindByIDForUpdate 行级锁查询申请（须在事务内调用），用于串行化并发审核。
func (r *RefundRepository) FindByIDForUpdate(id uint) (*model.RefundApplication, error) {
	var a model.RefundApplication
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&a, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find refund by id for update: %w", err)
	}
	return &a, nil
}

// Update 保存申请变更。
func (r *RefundRepository) Update(a *model.RefundApplication) error {
	if err := r.db.Save(a).Error; err != nil {
		return fmt.Errorf("update refund application: %w", err)
	}
	return nil
}

// ListByUser 用户的退款申请（分页）。
func (r *RefundRepository) ListByUser(userID uint, page, pageSize int) ([]model.RefundApplication, int64, error) {
	var list []model.RefundApplication
	var total int64
	q := r.db.Model(&model.RefundApplication{}).Preload("Donation").Preload("Donation.Project").
		Where("user_id = ?", userID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count refund applications: %w", err)
	}
	if err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, fmt.Errorf("list refund applications by user: %w", err)
	}
	return list, total, nil
}

// List 管理端退款申请列表，status 为空时返回全部。
func (r *RefundRepository) List(status string, page, pageSize int) ([]model.RefundApplication, int64, error) {
	var list []model.RefundApplication
	var total int64
	q := r.db.Model(&model.RefundApplication{}).
		Preload("Donation").Preload("Donation.Project").Preload("User").Preload("Reviewer")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count refund applications: %w", err)
	}
	if err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, fmt.Errorf("list refund applications: %w", err)
	}
	return list, total, nil
}
