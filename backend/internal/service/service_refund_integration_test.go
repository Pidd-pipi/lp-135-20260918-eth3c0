package service

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/givetrack/givetrack/internal/constants"
	"github.com/givetrack/givetrack/internal/model"
	"github.com/givetrack/givetrack/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newRefundTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	// 每个测试使用独立的内存数据库。
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared&_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.Organization{}, &model.Project{},
		&model.ProjectUpdate{}, &model.Donation{}, &model.RefundApplication{},
		&model.AdminReview{}, &model.VolunteerService{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM refund_applications")
		db.Exec("DELETE FROM donations")
		db.Exec("DELETE FROM projects")
		db.Exec("DELETE FROM organizations")
		db.Exec("DELETE FROM users")
	})
	return db
}

func setupRefundFixture(t *testing.T, db *gorm.DB, donationAge time.Duration, current, target float64, status string) (*model.User, *model.Project, *model.Donation) {
	t.Helper()
	u := &model.User{Username: "rdonor", Email: "rdonor@example.com", PasswordHash: "x", Role: constants.RoleUser}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	p := &model.Project{OrganizationID: 1, Title: "测试项目", Category: "education", TargetAmount: target, CurrentAmount: current, Status: status}
	if err := db.Create(p).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}
	d := &model.Donation{
		UserID: u.ID, ProjectID: p.ID, Amount: 100, PaymentStatus: constants.PaymentSuccess,
		CertificateNo: "CERT-TEST", TransactionID: "TXN-TEST",
	}
	if err := db.Create(d).Error; err != nil {
		t.Fatalf("create donation: %v", err)
	}
	// 强制捐赠创建时间为指定“年龄”，用于 24 小时窗口判定。
	old := time.Now().Add(-donationAge)
	if err := db.Model(&model.Donation{}).Where("id = ?", d.ID).Update("created_at", old).Error; err != nil {
		t.Fatalf("backdate donation: %v", err)
	}
	d.CreatedAt = old
	return u, p, d
}

func newRefundSvc(db *gorm.DB) *RefundService {
	return NewRefundService(
		db,
		repository.NewRefundRepository(db),
		repository.NewDonationRepository(db),
		repository.NewProjectRepository(db),
		repository.NewUserRepository(db),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
}

func reloadRefundFixture(t *testing.T, db *gorm.DB, donationID uint) (*model.Donation, *model.Project, *model.User) {
	t.Helper()
	var d model.Donation
	if err := db.First(&d, donationID).Error; err != nil {
		t.Fatalf("reload donation: %v", err)
	}
	var p model.Project
	if err := db.First(&p, d.ProjectID).Error; err != nil {
		t.Fatalf("reload project: %v", err)
	}
	var u model.User
	if err := db.First(&u, d.UserID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	return &d, &p, &u
}

// 完整流程：申请 -> 审核中不动账 -> 批准 -> 扣减/作废/恢复募集中。
func TestRefundFlow_ApproveDeductsAndReopens(t *testing.T) {
	db := newRefundTestDB(t)
	svc := newRefundSvc(db)
	_, _, d := setupRefundFixture(t, db, time.Hour, 100, 100, constants.ProjectCompleted)

	app, err := svc.Apply(d.UserID, d.ID, ApplyInput{Reason: "误捐申请退款"})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if app.Status != constants.RefundPending {
		t.Fatalf("new application status = %s, want pending", app.Status)
	}

	// 审核中：筹款进度与个人累计暂不扣减，凭证仍在。
	d2, p2, u2 := reloadRefundFixture(t, db, d.ID)
	if p2.CurrentAmount != 100 || p2.Status != constants.ProjectCompleted {
		t.Fatalf("project should be untouched during review, got amount=%v status=%s", p2.CurrentAmount, p2.Status)
	}
	if u2.TotalDonation != 0 || d2.CertificateNo != "CERT-TEST" {
		t.Fatalf("user/certificate should be untouched during review")
	}

	// 凭证审核中冻结。
	certSvc := NewDonationService(db, repository.NewDonationRepository(db), repository.NewProjectRepository(db), repository.NewUserRepository(db), svc.logger)
	if _, err := certSvc.Certificate(d.UserID, d.ID); err == nil {
		t.Fatal("certificate should be frozen during review")
	}

	if _, err := svc.Review(999, app.ID, ReviewInput{Status: constants.RefundApproved, Note: "同意退款"}); err != nil {
		t.Fatalf("review approve: %v", err)
	}

	d3, p3, u3 := reloadRefundFixture(t, db, d.ID)
	if d3.PaymentStatus != constants.PaymentRefunded {
		t.Fatalf("donation status = %s, want refunded", d3.PaymentStatus)
	}
	if d3.CertificateNo != "" {
		t.Fatalf("certificate should be voided, got %q", d3.CertificateNo)
	}
	if p3.CurrentAmount != 0 {
		t.Fatalf("project current = %v, want 0", p3.CurrentAmount)
	}
	// 跌破目标 -> 恢复募集中。
	if p3.Status != constants.ProjectApproved {
		t.Fatalf("project status = %s, want approved (reopened)", p3.Status)
	}
	if u3.TotalDonation != 0 {
		t.Fatalf("user total = %v, want 0", u3.TotalDonation)
	}
}

// 驳回：不做账务变更，凭证恢复。
func TestRefundFlow_RejectKeepsAmounts(t *testing.T) {
	db := newRefundTestDB(t)
	svc := newRefundSvc(db)
	u, _, d := setupRefundFixture(t, db, 2*time.Hour, 500, 1000, constants.ProjectApproved)
	if err := db.Model(&model.User{}).Where("id = ?", u.ID).Update("total_donation", 100).Error; err != nil {
		t.Fatal(err)
	}

	app, err := svc.Apply(u.ID, d.ID, ApplyInput{Reason: "暂时资金紧张"})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, err := svc.Review(999, app.ID, ReviewInput{Status: constants.RefundRejected, Note: "不符合退款条件"}); err != nil {
		t.Fatalf("review reject: %v", err)
	}

	d2, p2, u2 := reloadRefundFixture(t, db, d.ID)
	if d2.PaymentStatus != constants.PaymentSuccess || d2.CertificateNo != "CERT-TEST" {
		t.Fatalf("rejected donation should stay success with certificate intact")
	}
	if p2.CurrentAmount != 500 || p2.Status != constants.ProjectApproved {
		t.Fatalf("project amounts should remain unchanged")
	}
	if u2.TotalDonation != 100 {
		t.Fatalf("user total should remain 100, got %v", u2.TotalDonation)
	}
	// 凭证恢复可查看。
	if got, err := svcCertificateOrErr(certService(db), u.ID, d.ID); err != nil || got.ID != d.ID {
		t.Fatalf("certificate should be viewable after rejection, got err=%v", err)
	}
}

// 重复申请、超时申请、并发审核均返回冲突。
func TestRefundConflicts(t *testing.T) {
	db := newRefundTestDB(t)
	svc := newRefundSvc(db)
	u, _, d := setupRefundFixture(t, db, time.Hour, 100, 100, constants.ProjectCompleted)

	app, err := svc.Apply(u.ID, d.ID, ApplyInput{Reason: "第一次申请"})
	if err != nil {
		t.Fatalf("first apply: %v", err)
	}

	// 重复申请。
	if _, err := svc.Apply(u.ID, d.ID, ApplyInput{Reason: "重复申请"}); err == nil {
		t.Fatal("duplicate application should conflict")
	}

	// 他人不可申请。
	other := &model.User{Username: "other", Email: "other@example.com", PasswordHash: "x", Role: constants.RoleUser}
	if err := db.Create(other).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Apply(other.ID, d.ID, ApplyInput{Reason: "冒名"}); err == nil {
		t.Fatal("another user applying should fail")
	}

	// 批准后再次审核 -> 冲突（管理员只能处理一次）。
	if _, err := svc.Review(999, app.ID, ReviewInput{Status: constants.RefundApproved}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if _, err := svc.Review(998, app.ID, ReviewInput{Status: constants.RefundRejected}); err == nil {
		t.Fatal("second review should conflict")
	}

	// 已退款的捐赠不可再次申请。
	if _, err := svc.Apply(u.ID, d.ID, ApplyInput{Reason: "退款后再申请"}); err == nil {
		t.Fatal("applying on refunded donation should conflict")
	}
}

// 超过 24 小时窗口不可申请。
func TestRefundExpiredWindow(t *testing.T) {
	db := newRefundTestDB(t)
	svc := newRefundSvc(db)
	u, _, d := setupRefundFixture(t, db, 25*time.Hour, 100, 1000, constants.ProjectApproved)
	if _, err := svc.Apply(u.ID, d.ID, ApplyInput{Reason: "超时了"}); err == nil {
		t.Fatal("application beyond 24h window should fail")
	}
}

// 批准后项目已筹仍达标的项目保持已完成；仅跌破目标才恢复募集中。
func TestRefundApprove_StillCompletedWhenAboveTarget(t *testing.T) {
	db := newRefundTestDB(t)
	svc := newRefundSvc(db)
	// 当前 200、目标 100，退款 100 后仍达标，状态保持 completed。
	u, _, d := setupRefundFixture(t, db, time.Hour, 200, 100, constants.ProjectCompleted)
	if err := db.Model(&model.User{}).Where("id = ?", u.ID).Update("total_donation", 200).Error; err != nil {
		t.Fatal(err)
	}
	app, err := svc.Apply(u.ID, d.ID, ApplyInput{Reason: "退款"})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, err := svc.Review(1, app.ID, ReviewInput{Status: constants.RefundApproved}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	_, p, u2 := reloadRefundFixture(t, db, d.ID)
	if p.CurrentAmount != 100 || p.Status != constants.ProjectCompleted {
		t.Fatalf("project should remain completed above target, got amount=%v status=%s", p.CurrentAmount, p.Status)
	}
	if u2.TotalDonation != 100 {
		t.Fatalf("user total = %v, want 100", u2.TotalDonation)
	}
}

func certService(db *gorm.DB) *DonationService {
	return NewDonationService(db, repository.NewDonationRepository(db), repository.NewProjectRepository(db), repository.NewUserRepository(db), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func svcCertificateOrErr(svc *DonationService, userID, donationID uint) (*model.Donation, error) {
	return svc.Certificate(userID, donationID)
}
