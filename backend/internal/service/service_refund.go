package service

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/givetrack/givetrack/internal/constants"
	"github.com/givetrack/givetrack/internal/model"
	"github.com/givetrack/givetrack/internal/repository"
	"github.com/givetrack/givetrack/internal/util"
	"gorm.io/gorm"
)

// RefundService 公益捐赠退款审核服务。
type RefundService struct {
	db           *gorm.DB
	refundRepo   *repository.RefundRepository
	donationRepo *repository.DonationRepository
	projectRepo  *repository.ProjectRepository
	userRepo     *repository.UserRepository
	logger       *slog.Logger
}

func NewRefundService(
	db *gorm.DB,
	refundRepo *repository.RefundRepository,
	donationRepo *repository.DonationRepository,
	projectRepo *repository.ProjectRepository,
	userRepo *repository.UserRepository,
	logger *slog.Logger,
) *RefundService {
	return &RefundService{
		db:           db,
		refundRepo:   refundRepo,
		donationRepo: donationRepo,
		projectRepo:  projectRepo,
		userRepo:     userRepo,
		logger:       logger,
	}
}

// ApplyInput 退款申请入参。
type ApplyInput struct {
	Reason string `json:"reason"`
}

// ReviewInput 退款审核入参。
type ReviewInput struct {
	Status string `json:"status"`
	Note   string `json:"note"`
}

// validateApplyInput 申请参数校验（不依赖数据库，便于单测）。
func validateApplyInput(in ApplyInput) error {
	if strings.TrimSpace(in.Reason) == "" {
		return fmt.Errorf("reason is required")
	}
	if len([]rune(in.Reason)) > 500 {
		return fmt.Errorf("reason too long (max 500 chars)")
	}
	return nil
}

// validateReviewInput 审核参数校验。
func validateReviewInput(in ReviewInput) error {
	if in.Status != constants.RefundApproved && in.Status != constants.RefundRejected {
		return fmt.Errorf("invalid review status: must be approved or rejected")
	}
	if len([]rune(in.Note)) > 255 {
		return fmt.Errorf("review note too long (max 255 chars)")
	}
	return nil
}

// Apply 捐赠人对单笔成功捐赠申请全额退款。
// 规则：捐赠成功后 24 小时内；同一笔捐赠只能有一个申请（待审或已处理）。
// 申请提交后凭证冻结展示，筹款进度与个人累计暂不扣减（审核中不动账）。
func (s *RefundService) Apply(userID, donationID uint, in ApplyInput) (*model.RefundApplication, error) {
	if donationID == 0 {
		return nil, fmt.Errorf("donationId is required")
	}
	if err := validateApplyInput(in); err != nil {
		return nil, err
	}
	reason := strings.TrimSpace(in.Reason)

	var application *model.RefundApplication
	err := s.db.Transaction(func(tx *gorm.DB) error {
		donRepo := repository.NewDonationRepository(tx)
		refundRepo := repository.NewRefundRepository(tx)

		donation, err := donRepo.FindByIDForUpdate(donationID)
		if err != nil {
			return err
		}
		if donation.UserID != userID {
			return util.ErrForbidden
		}
		if donation.PaymentStatus != constants.PaymentSuccess {
			return fmt.Errorf("%w: donation is not refundable", util.ErrConflict)
		}
		if time.Since(donation.CreatedAt) > constants.RefundApplyWindow {
			return fmt.Errorf("%w: refund application window (24h) has expired", util.ErrConflict)
		}

		// 捐赠行已加锁，同一笔捐赠的并发申请在此串行化；唯一索引作为最终防线。
		existing, err := refundRepo.FindByDonationIDForUpdate(donationID)
		if err != nil {
			return err
		}
		if existing != nil {
			return fmt.Errorf("%w: refund application already exists for this donation", util.ErrConflict)
		}

		application = &model.RefundApplication{
			DonationID: donationID,
			UserID:     userID,
			Reason:     reason,
			Status:     constants.RefundPending,
		}
		if err := refundRepo.Create(application); err != nil {
			if isDuplicateKeyErr(err) {
				return fmt.Errorf("%w: refund application already exists for this donation", util.ErrConflict)
			}
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logger.Info("refund applied", "refundId", application.ID, "donationId", donationID, "userId", userID)
	return application, nil
}

// MyApplications 个人中心：我的退款申请及状态。
func (s *RefundService) MyApplications(userID uint, page, pageSize int) ([]model.RefundApplication, int64, error) {
	return s.refundRepo.ListByUser(userID, page, pageSize)
}

// GetApplication 查询单个退款申请（本人或管理员可查）。
func (s *RefundService) GetApplication(requesterID uint, requesterRole string, id uint) (*model.RefundApplication, error) {
	a, err := s.refundRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if requesterRole != constants.RoleAdmin && a.UserID != requesterID {
		return nil, util.ErrForbidden
	}
	return a, nil
}

// Review 管理员审核退款申请，仅可处理一次（批准/驳回）。
// 批准：扣减项目已筹与个人累计，作废凭证；项目跌破目标则恢复募集中。
// 驳回：不做账务变更，凭证恢复原展示。并发审核返回冲突。
func (s *RefundService) Review(adminID, applicationID uint, in ReviewInput) (*model.RefundApplication, error) {
	if applicationID == 0 {
		return nil, fmt.Errorf("applicationId is required")
	}
	if err := validateReviewInput(in); err != nil {
		return nil, err
	}

	var result *model.RefundApplication
	err := s.db.Transaction(func(tx *gorm.DB) error {
		refundRepo := repository.NewRefundRepository(tx)
		donRepo := repository.NewDonationRepository(tx)
		projectRepo := repository.NewProjectRepository(tx)
		userRepo := repository.NewUserRepository(tx)

		application, err := refundRepo.FindByIDForUpdate(applicationID)
		if err != nil {
			return err
		}
		if application.Status != constants.RefundPending {
			return fmt.Errorf("%w: refund application has already been reviewed", util.ErrConflict)
		}

		// 驳回：仅记录审核结果，凭证随后恢复展示。
		if in.Status == constants.RefundRejected {
			now := time.Now()
			application.Status = constants.RefundRejected
			application.ReviewerID = adminID
			application.ReviewNote = in.Note
			application.ReviewedAt = &now
			if err := refundRepo.Update(application); err != nil {
				return err
			}
			result = application
			return nil
		}

		// 批准：锁定捐赠/项目/用户，做一次性账务扣减。
		donation, err := donRepo.FindByIDForUpdate(application.DonationID)
		if err != nil {
			return err
		}
		if donation.PaymentStatus != constants.PaymentSuccess {
			return fmt.Errorf("%w: donation is not refundable", util.ErrConflict)
		}
		project, err := projectRepo.FindByIDForUpdate(donation.ProjectID)
		if err != nil {
			return err
		}
		user, err := userRepo.FindByIDForUpdate(donation.UserID)
		if err != nil {
			return err
		}

		donation.PaymentStatus = constants.PaymentRefunded
		donation.CertificateNo = "" // 作废凭证
		if err := donRepo.Update(donation); err != nil {
			return err
		}

		project.CurrentAmount -= donation.Amount
		if project.CurrentAmount < 0 {
			project.CurrentAmount = 0
		}
		// 已完成项目退款后跌破目标：恢复募集中。
		if project.Status == constants.ProjectCompleted && project.CurrentAmount < project.TargetAmount {
			project.Status = constants.ProjectApproved
		}
		if err := projectRepo.Update(project); err != nil {
			return err
		}

		user.TotalDonation -= donation.Amount
		if user.TotalDonation < 0 {
			user.TotalDonation = 0
		}
		if err := userRepo.Update(user); err != nil {
			return err
		}

		now := time.Now()
		application.Status = constants.RefundApproved
		application.ReviewerID = adminID
		application.ReviewNote = in.Note
		application.ReviewedAt = &now
		if err := refundRepo.Update(application); err != nil {
			return err
		}
		result = application
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.logger.Info("refund reviewed", "refundId", applicationID, "status", in.Status, "reviewerId", adminID)
	return result, nil
}

// ListApplications 管理端退款申请列表（status 为空查全部）。
func (s *RefundService) ListApplications(status string, page, pageSize int) ([]model.RefundApplication, int64, error) {
	if status != "" && status != constants.RefundPending &&
		status != constants.RefundApproved && status != constants.RefundRejected {
		return nil, 0, fmt.Errorf("invalid status filter")
	}
	return s.refundRepo.List(status, page, pageSize)
}

// isDuplicateKeyErr 识别 MySQL 唯一索引冲突（Error 1062 / Duplicate entry）。
func isDuplicateKeyErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "1062") || strings.Contains(msg, "Duplicate entry")
}
