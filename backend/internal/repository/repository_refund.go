package repository

import (
	"errors"
	"fmt"

	"github.com/givetrack/givetrack/internal/constants"
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

// Create 创建退款申请。
func (r *RefundRepository) Create(a *model.RefundApplication) error {
	if err := r.db.Create(a).Error; err != nil {
		return fmt.Errorf("create refund application: %w", err)
	}
	return nil
}

// FindLatestByDonationID 查询某笔捐赠最新一条退款申请（驳回后再次申请时返回当前生效的一条）。
// 不存在返回 ErrNotFound。
func (r *RefundRepository) FindLatestByDonationID(donationID uint) (*model.RefundApplication, error) {
	var a model.RefundApplication
	err := r.db.Where("donation_id = ?", donationID).Order("id DESC").First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find refund by donation id: %w", err)
	}
	return &a, nil
}

// FindPendingByDonationID 查询某笔捐赠是否存在待审核退款申请；不存在返回 ErrNotFound。
func (r *RefundRepository) FindPendingByDonationID(donationID uint) (*model.RefundApplication, error) {
	var a model.RefundApplication
	err := r.db.Where("donation_id = ? AND status = ?", donationID, constants.RefundPending).
		Order("id DESC").First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find pending refund by donation id: %w", err)
	}
	return &a, nil
}

// ListLatestByDonationIDs 批量查询多笔捐赠各自最新的一条退款申请。
// 驳回后再次申请时只返回每笔捐赠 id 最大（最新）的一条，供个人中心捐赠列表挂载状态。
func (r *RefundRepository) ListLatestByDonationIDs(donationIDs []uint) ([]model.RefundApplication, error) {
	var list []model.RefundApplication
	if len(donationIDs) == 0 {
		return list, nil
	}
	subQuery := r.db.Model(&model.RefundApplication{}).
		Select("donation_id, MAX(id) AS max_id").
		Where("donation_id IN ?", donationIDs).
		Group("donation_id")
	if err := r.db.Table("refund_applications AS r").
		Select("r.*").
		Joins("INNER JOIN (?) AS m ON r.id = m.max_id", subQuery).
		Scan(&list).Error; err != nil {
		return nil, fmt.Errorf("list latest refunds by donation ids: %w", err)
	}
	return list, nil
}

// FindByID 按主键查询退款申请。
func (r *RefundRepository) FindByID(id uint) (*model.RefundApplication, error) {
	var a model.RefundApplication
	err := r.db.First(&a, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find refund by id: %w", err)
	}
	return &a, nil
}

// FindByIDForUpdate 按主键查询退款申请并加行锁，须在事务中调用。
func (r *RefundRepository) FindByIDForUpdate(id uint) (*model.RefundApplication, error) {
	var a model.RefundApplication
	err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&a, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find refund for update: %w", err)
	}
	return &a, nil
}

// Save 保存退款申请。
func (r *RefundRepository) Save(a *model.RefundApplication) error {
	if err := r.db.Save(a).Error; err != nil {
		return fmt.Errorf("save refund application: %w", err)
	}
	return nil
}

// ListByUser 用户的退款申请列表（分页）。
func (r *RefundRepository) ListByUser(userID uint, page, pageSize int) ([]model.RefundApplication, int64, error) {
	var list []model.RefundApplication
	var total int64
	q := r.db.Model(&model.RefundApplication{}).
		Preload("Project").
		Where("user_id = ?", userID)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count refund applications: %w", err)
	}
	if err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, fmt.Errorf("list refund applications by user: %w", err)
	}
	return list, total, nil
}

// ListPending 管理员待审核退款申请列表。
func (r *RefundRepository) ListPending(page, pageSize int) ([]model.RefundApplication, int64, error) {
	var list []model.RefundApplication
	var total int64
	q := r.db.Model(&model.RefundApplication{}).
		Preload("Donation").Preload("Donation.User").Preload("Project").Preload("Reviewer").
		Where("status = ?", constants.RefundPending)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count pending refunds: %w", err)
	}
	if err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, fmt.Errorf("list pending refunds: %w", err)
	}
	return list, total, nil
}
