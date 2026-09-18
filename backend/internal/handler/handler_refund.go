package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/givetrack/givetrack/internal/constants"
	"github.com/givetrack/givetrack/internal/service"
	"github.com/givetrack/givetrack/internal/util"
)

// RefundHandler 公益捐赠退款审核处理器。
type RefundHandler struct {
	refundSvc *service.RefundService
}

func NewRefundHandler(refundSvc *service.RefundService) *RefundHandler {
	return &RefundHandler{refundSvc: refundSvc}
}

// Apply 捐赠人申请退款。
func (h *RefundHandler) Apply(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "invalid donation id")
		return
	}
	var req service.ApplyInput
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeValidation, err.Error())
		return
	}
	a, err := h.refundSvc.Apply(c.GetUint("user_id"), uint(id), req)
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.Created(c, gin.H{
		"message": "退款申请已提交，等待审核",
		"refund":  a,
	})
}

// My 我的退款申请列表。
func (h *RefundHandler) My(c *gin.Context) {
	page, ps := util.NormalizePage(atoi(c.Query("page")), atoi(c.Query("page_size")))
	if c.Query("limit") != "" {
		ps = atoi(c.Query("limit"))
	}
	list, total, err := h.refundSvc.MyApplications(c.GetUint("user_id"), page, ps)
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, gin.H{"refunds": list, "total": total, "page": page, "limit": ps})
}

// Detail 查询单个退款申请。
func (h *RefundHandler) Detail(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "invalid refund id")
		return
	}
	a, err := h.refundSvc.GetApplication(c.GetUint("user_id"), c.GetString("role"), uint(id))
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, gin.H{"refund": a})
}

// List 管理端退款申请列表（可选 status 过滤）。
func (h *RefundHandler) List(c *gin.Context) {
	page, ps := util.NormalizePage(atoi(c.Query("page")), atoi(c.Query("page_size")))
	if c.Query("limit") != "" {
		ps = atoi(c.Query("limit"))
	}
	list, total, err := h.refundSvc.ListApplications(c.Query("status"), page, ps)
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, gin.H{"refunds": list, "total": total, "page": page, "limit": ps})
}

// Review 管理员批准/驳回退款申请（仅一次）。
func (h *RefundHandler) Review(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "invalid refund id")
		return
	}
	var req service.ReviewInput
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeValidation, err.Error())
		return
	}
	a, err := h.refundSvc.Review(c.GetUint("user_id"), uint(id), req)
	if err != nil {
		util.FailError(c, err)
		return
	}
	message := "退款申请已驳回"
	if req.Status == constants.RefundApproved {
		message = "退款申请已批准，已扣减筹款并作废凭证"
	}
	util.OK(c, gin.H{"message": message, "refund": a})
}
