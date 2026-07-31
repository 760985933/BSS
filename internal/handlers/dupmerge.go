package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"gorm.io/gorm"

	"bss/internal/pkg/resp"
	"bss/internal/services"
)

// DupMergeHandler 客户查重与合并（限 admin）
type DupMergeHandler struct {
	db *gorm.DB
}

// NewDupMergeHandler 构造
func NewDupMergeHandler(db *gorm.DB) *DupMergeHandler {
	return &DupMergeHandler{db: db}
}

// Duplicates 查询疑似重复客户分组
func (h *DupMergeHandler) Duplicates(w http.ResponseWriter, r *http.Request) {
	groups, err := services.FindDuplicates(r.Context(), h.db)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "查询重复客户失败")
		return
	}
	resp.OK(w, groups)
}

type mergeCustomerReq struct {
	PrimaryID   uint `json:"primary_id"`
	SecondaryID uint `json:"secondary_id"`
}

// Merge 将 secondary 客户合并到 primary
func (h *DupMergeHandler) Merge(w http.ResponseWriter, r *http.Request) {
	var req mergeCustomerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求参数错误")
		return
	}
	if req.PrimaryID == 0 || req.SecondaryID == 0 {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "主客户与从客户均不能为空")
		return
	}
	err := services.MergeCustomers(r.Context(), h.db, req.PrimaryID, req.SecondaryID)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrDupSameCustomer):
			resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
		case errors.Is(err, services.ErrDupCustomerMissing):
			resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
		default:
			resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "合并失败")
		}
		return
	}
	resp.OK(w, map[string]string{"message": "合并成功"})
}
