package service

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/givetrack/givetrack/internal/constants"
	"github.com/givetrack/givetrack/internal/model"
	"github.com/givetrack/givetrack/internal/repository"
	"gorm.io/gorm"
)

// RefundApplyWindow 捐赠成功后可申请全额退款的时间窗口。
const RefundApplyWindow = 24 * time.Hour

// ErrRefundConflict 退款业务冲突（重复申请、超时、并发审核等），映射为 HTTP 409。
var ErrRefundConflict = repository.ErrConflict

// RefundService 公益捐赠退款审核服务。
type RefundService struct {
	db           *gorm.DB
	refundRepo   *repository.RefundRepository
	donationRepo *repository.DonationRepository
	logger       *slog.Logger
}

func NewRefundService(
	db *gorm.DB,
	refundRepo *repository.RefundRepository,
	donationRepo *repository.DonationRepository,
	logger *slog.Logger,
) *RefundService {
	return &RefundService{
		db:           db,
		refundRepo:   refundRepo,
		donationRepo: donationRepo,
		logger:       logger,
	}
}

// ApplyInput 退款申请入参。
type ApplyInput struct {
	Reason string `json:"reason" binding:"required,max=255"`
}

// Apply 捐赠人对单笔成功捐赠申请全额退款。
// 规则：仅本人、24 小时内、未退款、同一笔只能有一个待审申请；重复/超时/并发申请返回冲突。
// 申请被驳回后，若仍在 24 小时时限内可再次申请。
func (s *RefundService) Apply(userID, donationID uint, reason string) (*model.RefundApplication, error) {
	application := &model.RefundApplication{
		DonationID: donationID,
		UserID:     userID,
		Reason:     reason,
		Status:     constants.RefundPending,
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		donationRepo := repository.NewDonationRepository(tx)
		refundRepo := repository.NewRefundRepository(tx)

		// 锁定捐赠行，串行化同一笔捐赠的并发退款申请/审核。
		d, err := donationRepo.FindByIDForUpdate(donationID)
		if err != nil {
			return err
		}
		if d.UserID != userID {
			return fmt.Errorf("%w: 无权对他人捐赠申请退款", ErrRefundConflict)
		}
		if err := validateRefundApply(d, time.Now()); err != nil {
			return err
		}
		application.ProjectID = d.ProjectID
		application.Amount = d.Amount

		// 同一笔捐赠只能有一个待审申请；驳回后允许再次申请，已批准则禁止。
		// 捐赠行锁已保证并发申请/审核串行化，此处普通读取即可。
		if latest, ferr := refundRepo.FindLatestByDonationID(donationID); ferr == nil {
			switch latest.Status {
			case constants.RefundPending:
				return fmt.Errorf("%w: 该笔捐赠已有待审核退款申请，请勿重复提交", ErrRefundConflict)
			case constants.RefundApproved:
				return fmt.Errorf("%w: 该笔捐赠已退款，不能再次申请", ErrRefundConflict)
			}
		} else if !errors.Is(ferr, repository.ErrNotFound) {
			return ferr
		}
		return refundRepo.Create(application)
	})
	if err != nil {
		return nil, err
	}
	s.logger.Info("refund applied", "refundId", application.ID, "donationId", donationID, "userId", userID, "amount", application.Amount)
	return application, nil
}

// validateRefundApply 校验单笔捐赠是否可发起退款：必须成功支付且在 24 小时时限内。
func validateRefundApply(d *model.Donation, now time.Time) error {
	if d.PaymentStatus != constants.PaymentSuccess {
		return fmt.Errorf("%w: 该捐赠已退款，凭证已作废", ErrRefundConflict)
	}
	if now.Sub(d.CreatedAt) > RefundApplyWindow {
		return fmt.Errorf("%w: 已超过捐赠成功后 24 小时退款申请时限", ErrRefundConflict)
	}
	return nil
}

// MyApplications 查看本人退款申请。
func (s *RefundService) MyApplications(userID uint, page, pageSize int) ([]model.RefundApplication, int64, error) {
	return s.refundRepo.ListByUser(userID, page, pageSize)
}

// StatusByDonation 查询某笔捐赠的退款申请状态（供个人中心展示）。
func (s *RefundService) StatusByDonation(userID, donationID uint) (*model.RefundApplication, error) {
	d, err := s.donationRepo.FindByID(donationID)
	if err != nil {
		return nil, err
	}
	if d.UserID != userID {
		return nil, fmt.Errorf("%w: 无权查看他人捐赠退款状态", ErrRefundConflict)
	}
	a, err := s.refundRepo.FindLatestByDonationID(donationID)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// PendingList 管理员待审核退款申请列表。
func (s *RefundService) PendingList(page, pageSize int) ([]model.RefundApplication, int64, error) {
	return s.refundRepo.ListPending(page, pageSize)
}

// Review 管理员审核退款申请，只能批准或驳回一次。
// 批准：扣减项目已筹与个人累计、作废凭证，跌破目标则项目恢复募集中。
// 驳回：仅更新申请状态，凭证与筹款金额恢复原展示。
func (s *RefundService) Review(adminID, applicationID uint, approve bool, reviewReason string) (*model.RefundApplication, error) {
	now := time.Now()
	err := s.db.Transaction(func(tx *gorm.DB) error {
		refundRepo := repository.NewRefundRepository(tx)
		donationRepo := repository.NewDonationRepository(tx)
		projectRepo := repository.NewProjectRepository(tx)
		userRepo := repository.NewUserRepository(tx)

		// 先读取申请拿到捐赠 ID（无锁），随后按“捐赠行 → 申请行”的统一顺序加锁，
		// 与 Apply 的加锁顺序保持一致，避免并发下的 ABBA 死锁。
		app, err := refundRepo.FindByID(applicationID)
		if err != nil {
			return err
		}
		if _, err := donationRepo.FindByIDForUpdate(app.DonationID); err != nil {
			return err
		}
		application, err := refundRepo.FindByIDForUpdate(applicationID)
		if err != nil {
			return err
		}
		// 行锁串行化：已被处理的申请再次审核直接冲突。
		if application.Status != constants.RefundPending {
			return fmt.Errorf("%w: 该退款申请已审核，不能重复处理", ErrRefundConflict)
		}

		if approve {
			// 条件更新确保捐赠只退款一次，并发审核下只有一个事务影响行数为 1。
			affected, err := donationRepo.MarkRefunded(application.DonationID)
			if err != nil {
				return err
			}
			if affected == 0 {
				return fmt.Errorf("%w: 捐赠已退款或状态异常，审核冲突", ErrRefundConflict)
			}
			if err := projectRepo.DeductAmount(application.ProjectID, application.Amount); err != nil {
				return err
			}
			if err := userRepo.DeductTotalDonation(application.UserID, application.Amount); err != nil {
				return err
			}
			application.Status = constants.RefundApproved
		} else {
			application.Status = constants.RefundRejected
		}
		application.ReviewReason = reviewReason
		application.ReviewerID = adminID
		application.ReviewedAt = &now
		return refundRepo.Save(application)
	})
	if err != nil {
		return nil, err
	}
	s.logger.Info("refund reviewed",
		"refundId", applicationID, "reviewerId", adminID, "approved", approve)
	return s.refundRepo.FindByID(applicationID)
}
