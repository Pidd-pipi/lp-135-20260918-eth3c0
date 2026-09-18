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

// Apply 捐赠人申请单笔捐赠全额退款。
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
	a, err := h.refundSvc.Apply(c.GetUint("user_id"), uint(id), req.Reason)
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.Created(c, gin.H{
		"message":  "退款申请已提交，等待审核",
		"refundId": a.ID,
		"status":   a.Status,
	})
}

// My 查看本人退款申请列表与状态。
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

// Status 查询某笔捐赠的退款申请状态。
func (h *RefundHandler) Status(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "invalid donation id")
		return
	}
	a, err := h.refundSvc.StatusByDonation(c.GetUint("user_id"), uint(id))
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, gin.H{"refund": a})
}

// Pending 管理员待审核退款申请列表。
func (h *RefundHandler) Pending(c *gin.Context) {
	page, ps := util.NormalizePage(atoi(c.Query("page")), atoi(c.Query("page_size")))
	if c.Query("limit") != "" {
		ps = atoi(c.Query("limit"))
	}
	list, total, err := h.refundSvc.PendingList(page, ps)
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, gin.H{"refunds": list, "total": total, "page": page, "limit": ps})
}

// Review 管理员批准或驳回退款申请（仅可处理一次）。
func (h *RefundHandler) Review(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeBadRequest, "invalid refund id")
		return
	}
	var req struct {
		// action: approve 批准 / reject 驳回
		Action string `json:"action" binding:"required,oneof=approve reject"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		util.Fail(c, http.StatusBadRequest, constants.CodeValidation, err.Error())
		return
	}
	a, err := h.refundSvc.Review(c.GetUint("user_id"), uint(id), req.Action == "approve", req.Reason)
	if err != nil {
		util.FailError(c, err)
		return
	}
	util.OK(c, gin.H{"refund": a})
}
