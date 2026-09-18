package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/givetrack/givetrack/internal/config"
	"github.com/givetrack/givetrack/internal/constants"
	"github.com/givetrack/givetrack/internal/model"
	"github.com/givetrack/givetrack/internal/repository"
	"github.com/givetrack/givetrack/internal/service"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type refundAPITestEnv struct {
	engine     *gin.Engine
	db         *gorm.DB
	userToken  string
	adminToken string
	donationID uint
	authSvc    *service.AuthService
}

// refundDBCounter 保证同一测试内多次建环境使用不同内存库。
var refundDBCounter atomic.Uint64

func setupRefundAPITest(t *testing.T, donationAge time.Duration) *refundAPITestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// 每个测试环境使用独立的内存数据库，避免共享缓存相互干扰。
	seq := refundDBCounter.Add(1)
	dsn := fmt.Sprintf("file:%s_%d?mode=memory&cache=shared&_busy_timeout=5000", t.Name(), seq)
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

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	userRepo := repository.NewUserRepository(db)
	orgRepo := repository.NewOrganizationRepository(db)
	projectRepo := repository.NewProjectRepository(db)
	updateRepo := repository.NewProjectUpdateRepository(db)
	donationRepo := repository.NewDonationRepository(db)
	reviewRepo := repository.NewAdminReviewRepository(db)
	refundRepo := repository.NewRefundRepository(db)

	authSvc := service.NewAuthService(userRepo, orgRepo, "test-secret-test-secret-test-secret", 72, log)
	projectSvc := service.NewProjectService(projectRepo, updateRepo, orgRepo, donationRepo, log)
	donationSvc := service.NewDonationService(db, donationRepo, projectRepo, userRepo, log)
	rankingSvc := service.NewRankingService(userRepo, log)
	adminSvc := service.NewAdminService(projectRepo, orgRepo, reviewRepo, log)
	refundSvc := service.NewRefundService(db, refundRepo, donationRepo, projectRepo, userRepo, log)

	cfg := &config.Config{AuthRateLimit: 100, AuthRateWindowSecs: 60, JWTSecret: "test-secret-test-secret-test-secret"}
	engine := Setup(db, authSvc, projectSvc, donationSvc, rankingSvc, adminSvc, refundSvc, cfg, log)

	// 种子：普通用户、管理员、已批准项目、成功捐赠。
	donor := &model.User{Username: "api_donor", Email: "api_donor@example.com", PasswordHash: "x", Role: constants.RoleUser}
	if err := db.Create(donor).Error; err != nil {
		t.Fatal(err)
	}
	admin := &model.User{Username: "api_admin", Email: "api_admin@example.com", PasswordHash: "x", Role: constants.RoleAdmin}
	if err := db.Create(admin).Error; err != nil {
		t.Fatal(err)
	}
	proj := &model.Project{OrganizationID: 1, Title: "接口测试项目", Category: "education", TargetAmount: 1000, CurrentAmount: 100, Status: constants.ProjectApproved}
	if err := db.Create(proj).Error; err != nil {
		t.Fatal(err)
	}
	donation := &model.Donation{UserID: donor.ID, ProjectID: proj.ID, Amount: 100, PaymentStatus: constants.PaymentSuccess, CertificateNo: "CERT-API", TransactionID: "TXN-API"}
	if err := db.Create(donation).Error; err != nil {
		t.Fatal(err)
	}
	created := time.Now().Add(-donationAge)
	if err := db.Model(&model.Donation{}).Where("id = ?", donation.ID).Update("created_at", created).Error; err != nil {
		t.Fatal(err)
	}

	userToken, err := authSvc.GenerateToken(donor)
	if err != nil {
		t.Fatal(err)
	}
	adminToken, err := authSvc.GenerateToken(admin)
	if err != nil {
		t.Fatal(err)
	}

	return &refundAPITestEnv{
		engine: engine, db: db, userToken: userToken, adminToken: adminToken,
		donationID: donation.ID, authSvc: authSvc,
	}
}

func (e *refundAPITestEnv) do(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return w
}

func apiCode(t *testing.T, w *httptest.ResponseRecorder) int {
	t.Helper()
	var resp struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response %q: %v", w.Body.String(), err)
	}
	return resp.Code
}

// 全链路：申请 -> 冻结凭证 -> 管理端列表 -> 批准 -> 再次审核冲突/凭证作废。
func TestRefundAPI_ApplyReviewFlow(t *testing.T) {
	env := setupRefundAPITest(t, time.Hour)

	// 未认证申请被拒。
	if w := env.do(t, http.MethodPost, "/api/v1/donations/"+itoa(env.donationID)+"/refund", "", map[string]string{"reason": "x"}); w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated apply status = %d, want 401", w.Code)
	}

	// 提交申请。
	w := env.do(t, http.MethodPost, "/api/v1/donations/"+itoa(env.donationID)+"/refund", env.userToken, map[string]string{"reason": "误捐了"})
	if w.Code != http.StatusCreated {
		t.Fatalf("apply status = %d body = %s", w.Code, w.Body.String())
	}
	var applyResp struct {
		Data struct {
			Refund model.RefundApplication `json:"refund"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &applyResp); err != nil {
		t.Fatal(err)
	}
	refundID := applyResp.Data.Refund.ID
	if applyResp.Data.Refund.Status != constants.RefundPending {
		t.Fatalf("status = %s, want pending", applyResp.Data.Refund.Status)
	}

	// 重复申请 -> 409。
	if w := env.do(t, http.MethodPost, "/api/v1/donations/"+itoa(env.donationID)+"/refund", env.userToken, map[string]string{"reason": "再试一次"}); w.Code != http.StatusConflict || apiCode(t, w) != constants.CodeConflict {
		t.Fatalf("duplicate apply status = %d, want 409/40900", w.Code)
	}

	// 审核中凭证冻结 -> 409。
	if w := env.do(t, http.MethodGet, "/api/v1/donations/"+itoa(env.donationID)+"/certificate", env.userToken, nil); w.Code != http.StatusConflict {
		t.Fatalf("certificate during review status = %d, want 409", w.Code)
	}

	// 我的退款申请列表。
	if w := env.do(t, http.MethodGet, "/api/v1/refunds/my?limit=10", env.userToken, nil); w.Code != http.StatusOK {
		t.Fatalf("my refunds status = %d, want 200", w.Code)
	}

	// 普通用户不能访问管理端列表 -> 403。
	if w := env.do(t, http.MethodGet, "/api/v1/admin/refunds", env.userToken, nil); w.Code != http.StatusForbidden {
		t.Fatalf("non-admin list status = %d, want 403", w.Code)
	}

	// 管理员看到待审核申请。
	w = env.do(t, http.MethodGet, "/api/v1/admin/refunds?status=pending", env.adminToken, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("admin list status = %d body = %s", w.Code, w.Body.String())
	}
	var listResp struct {
		Data struct {
			Refunds []model.RefundApplication `json:"refunds"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Data.Refunds) != 1 {
		t.Fatalf("pending refunds = %d, want 1", len(listResp.Data.Refunds))
	}

	// 非法审核动作 -> 400/422。
	if w := env.do(t, http.MethodPost, "/api/v1/admin/refunds/"+itoa(refundID)+"/review", env.adminToken, map[string]string{"status": "maybe"}); w.Code == http.StatusOK {
		t.Fatal("invalid review status should not succeed")
	}

	// 批准。
	w = env.do(t, http.MethodPost, "/api/v1/admin/refunds/"+itoa(refundID)+"/review", env.adminToken, map[string]string{"status": "approved", "note": "同意"})
	if w.Code != http.StatusOK {
		t.Fatalf("approve status = %d body = %s", w.Code, w.Body.String())
	}

	// 并发/二次审核 -> 409。
	if w := env.do(t, http.MethodPost, "/api/v1/admin/refunds/"+itoa(refundID)+"/review", env.adminToken, map[string]string{"status": "rejected"}); w.Code != http.StatusConflict {
		t.Fatalf("second review status = %d, want 409", w.Code)
	}

	// 批准后凭证作废 -> 409。
	if w := env.do(t, http.MethodGet, "/api/v1/donations/"+itoa(env.donationID)+"/certificate", env.userToken, nil); w.Code != http.StatusConflict {
		t.Fatalf("certificate after approval status = %d, want 409", w.Code)
	}

	// 项目已筹被扣减。
	var proj model.Project
	if err := env.db.First(&proj, 1).Error; err != nil {
		t.Fatal(err)
	}
	if proj.CurrentAmount != 0 || proj.Status != constants.ProjectApproved {
		t.Fatalf("project after refund: amount=%v status=%s, want 0/approved", proj.CurrentAmount, proj.Status)
	}
}

// 超时申请返回冲突，驳回后凭证恢复。
func TestRefundAPI_ExpiredAndReject(t *testing.T) {
	// 超时申请。
	env := setupRefundAPITest(t, 25*time.Hour)
	if w := env.do(t, http.MethodPost, "/api/v1/donations/"+itoa(env.donationID)+"/refund", env.userToken, map[string]string{"reason": "晚了"}); w.Code != http.StatusConflict {
		t.Fatalf("expired apply status = %d, want 409", w.Code)
	}

	// 窗口内申请并驳回。
	env2 := setupRefundAPITest(t, 2*time.Hour)
	w := env2.do(t, http.MethodPost, "/api/v1/donations/"+itoa(env2.donationID)+"/refund", env2.userToken, map[string]string{"reason": "不想要了"})
	if w.Code != http.StatusCreated {
		t.Fatalf("apply status = %d body = %s", w.Code, w.Body.String())
	}
	var applyResp struct {
		Data struct {
			Refund model.RefundApplication `json:"refund"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &applyResp); err != nil {
		t.Fatal(err)
	}
	if w := env2.do(t, http.MethodPost, "/api/v1/admin/refunds/"+itoa(applyResp.Data.Refund.ID)+"/review", env2.adminToken, map[string]string{"status": "rejected", "note": "不符合条件"}); w.Code != http.StatusOK {
		t.Fatalf("reject status = %d body = %s", w.Code, w.Body.String())
	}
	// 驳回恢复凭证展示。
	if w := env2.do(t, http.MethodGet, "/api/v1/donations/"+itoa(env2.donationID)+"/certificate", env2.userToken, nil); w.Code != http.StatusOK {
		t.Fatalf("certificate after rejection status = %d, want 200", w.Code)
	}
	// 驳回后同一笔不能再次申请。
	if w := env2.do(t, http.MethodPost, "/api/v1/donations/"+itoa(env2.donationID)+"/refund", env2.userToken, map[string]string{"reason": "再次申请"}); w.Code != http.StatusConflict {
		t.Fatalf("re-apply after rejection status = %d, want 409", w.Code)
	}
}

func itoa(u uint) string {
	return strconv.FormatUint(uint64(u), 10)
}
