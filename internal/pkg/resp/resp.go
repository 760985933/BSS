// Package resp 统一 API 响应格式与业务错误码（TECH_DESIGN §5）
package resp

import (
	"encoding/json"
	"net/http"
)

// 业务错误码段：1xxx 通用，2xxx 客户/商单，3xxx 合同/回款，4xxx 权限
const (
	CodeOK = 0

	CodeBadRequest   = 1001 // 参数错误
	CodeUnauthorized = 1002 // 未登录或凭证失效
	CodeForbidden    = 1003 // 无权限
	CodeNotFound     = 1004 // 资源不存在
	CodeConflict     = 1009 // 冲突（唯一约束、非法状态流转等）
	CodeInternal     = 1500 // 服务器内部错误

	CodeCustomerHasChildren = 2001 // 客户名下存在下游数据，禁止删除
	CodeInvalidStateTransit = 2002 // 非法状态流转
	CodeExitCriteriaUnmet   = 2003 // 阶段退出标准未满足（软校验，可 force）

	CodePlanAmountExceed  = 3001 // 回款计划总额超合同额
	CodePlanLocked        = 3002 // 计划已被核销，禁改禁删
	CodeCrossCustomerLink = 3003 // 商单与合同不同客户，禁止关联
	CodeFieldLocked       = 3004 // 终态锁定字段不可修改
	CodeContractHasChildren = 3005 // 合同存在回款计划，禁止删除
	CodeContractLocked      = 3006 // 合同已签约/终态，禁止删除

	// 40xx 审批流
	CodeApprovalInvalid    = 4001 // 审批单非法（类型/对象状态不对/驳回缺原因）
	CodeApprovalNotPending = 4002 // 审批单非待审状态，无法操作

	// 41xx 开票
	CodeInvoiceAmountExceed = 4101 // 开票累计金额超过合同额
	CodeInvoiceInvalidState = 4102 // 开票状态流转非法
)

type Body struct {
	Code    int    `json:"code"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
	Warning bool   `json:"warning,omitempty"` // 软校验提示（前端弹确认后可 force 重发）
}

// PageData 列表统一分页结构
type PageData struct {
	List any   `json:"list"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Size  int   `json:"size"`
}

func write(c *bodyCtx, httpStatus int, b Body) {
	c.w.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.w.WriteHeader(httpStatus)
	_ = json.NewEncoder(c.w).Encode(b)
}

type bodyCtx struct{ w http.ResponseWriter }

// OK 成功响应
func OK(w http.ResponseWriter, data any) {
	write(&bodyCtx{w}, http.StatusOK, Body{Code: CodeOK, Data: data})
}

// OKPage 分页成功响应
func OKPage(w http.ResponseWriter, list any, total int64, page, size int) {
	OK(w, PageData{List: list, Total: total, Page: page, Size: size})
}

// Fail 失败响应（message 必须是用户可读信息）
func Fail(w http.ResponseWriter, httpStatus, code int, message string) {
	write(&bodyCtx{w}, httpStatus, Body{Code: code, Message: message})
}

// FailWarning 软校验失败：前端弹确认框，用户确认后带 force=true 重发
func FailWarning(w http.ResponseWriter, code int, message string) {
	write(&bodyCtx{w}, http.StatusUnprocessableEntity, Body{Code: code, Message: message, Warning: true})
}
