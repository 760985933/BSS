package handlers

import (
	"net/http"

	"gorm.io/gorm"

	"bss/internal/pkg/resp"
	"bss/internal/services"
)

// AnalysisHandler 只读分析（报表类，M4-2）
type AnalysisHandler struct {
	db *gorm.DB
}

// NewAnalysisHandler 构造
func NewAnalysisHandler(db *gorm.DB) *AnalysisHandler {
	return &AnalysisHandler{db: db}
}

// LostDeals 商单输单分析（按原因 / 负责人 / 月份聚合）
func (h *AnalysisHandler) LostDeals(w http.ResponseWriter, r *http.Request) {
	res, err := services.LostDealAnalysis(r.Context(), h.db)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "分析失败")
		return
	}
	resp.OK(w, res)
}
