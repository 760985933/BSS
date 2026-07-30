package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bss/internal/middleware"
	"bss/internal/models"
	"bss/internal/pkg/resp"
	"bss/internal/services"

	"gorm.io/gorm"
)

// ContractHandler 合同 + 附件
type ContractHandler struct {
	db        *gorm.DB
	svc       *services.ContractService
	uploadDir string // 物理附件根目录（data/uploads）
}

func NewContractHandler(db *gorm.DB, uploadDir string) *ContractHandler {
	return &ContractHandler{db: db, svc: services.NewContractService(db), uploadDir: uploadDir}
}

// List GET /api/v1/contracts —— ScopeOwner + 筛选 + 分页
func (h *ContractHandler) List(w http.ResponseWriter, r *http.Request) {
	page, size := pageParams(r)
	q := r.URL.Query()
	list, total, err := h.svc.List(r.Context(), page, size, q.Get("keyword"), q.Get("status"), q.Get("customer_id"))
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "查询合同失败")
		return
	}
	resp.OKPage(w, list, total, page, size)
}

// Create POST /api/v1/contracts（body 含 deal_ids:[]）
func (h *ContractHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in services.ContractInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求格式错误")
		return
	}
	c := middleware.UserFrom(r.Context())
	ct, err := h.svc.Create(r.Context(), in, c.UserID)
	if err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, ct)
}

// Get GET /api/v1/contracts/:id
func (h *ContractHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok || !h.canAccess(w, r, id) {
		return
	}
	ct, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, ct)
}

// Update PUT /api/v1/contracts/:id
func (h *ContractHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok || !h.canAccess(w, r, id) {
		return
	}
	var in services.ContractInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求格式错误")
		return
	}
	if err := h.svc.Update(r.Context(), id, in); err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, map[string]string{"message": "已保存"})
}

// ChangeStatus POST /api/v1/contracts/:id/status {to, terminate_reason}
func (h *ContractHandler) ChangeStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok || !h.canAccess(w, r, id) {
		return
	}
	var req struct {
		To              string `json:"to"`
		TerminateReason string `json:"terminate_reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求格式错误")
		return
	}
	ct, err := h.svc.ChangeStatus(r.Context(), id, req.To, req.TerminateReason)
	if err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, ct)
}

// ReplaceDeals PUT /api/v1/contracts/:id/deals {deal_ids:[]}
func (h *ContractHandler) ReplaceDeals(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok || !h.canAccess(w, r, id) {
		return
	}
	var req struct {
		DealIDs []uint `json:"deal_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求格式错误")
		return
	}
	if err := h.svc.ReplaceDeals(r.Context(), id, req.DealIDs); err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, map[string]string{"message": "已更新关联商单"})
}

// Delete DELETE /api/v1/contracts/:id
func (h *ContractHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok || !h.canAccess(w, r, id) {
		return
	}
	if err := h.svc.Delete(r.Context(), id); err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, map[string]string{"message": "已删除"})
}

// ---------- 附件 ----------

// ListAttachments GET /api/v1/contracts/:id/attachments
func (h *ContractHandler) ListAttachments(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok || !h.canAccess(w, r, id) {
		return
	}
	list, err := h.svc.ListAttachments(r.Context(), "contract", id)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "查询附件失败")
		return
	}
	resp.OK(w, list)
}

// UploadAttachment POST /api/v1/contracts/:id/attachments（multipart, 白名单 + 20MB）
func (h *ContractHandler) UploadAttachment(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok || !h.canAccess(w, r, id) {
		return
	}
	if _, err := h.svc.Get(r.Context(), id); err != nil {
		h.failSvc(w, err)
		return
	}
	if err := r.ParseMultipartForm(services.MaxAttachmentSize + 1<<20); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求格式错误（需 multipart/form-data）")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请选择要上传的文件")
		return
	}
	defer file.Close()
	if err := services.ValidateAttachment(header.Filename, header.Size); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
		return
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	randBytes := make([]byte, 8)
	if _, err := rand.Read(randBytes); err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "生成文件名失败")
		return
	}
	relDir := time.Now().UTC().Format("2006/01")
	storedName := fmt.Sprintf("%d%s%s", time.Now().UTC().UnixNano(), hex.EncodeToString(randBytes), ext)
	relPath := filepath.Join(relDir, storedName)
	absPath := filepath.Join(h.uploadDir, relPath)
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "创建上传目录失败")
		return
	}
	out, err := os.Create(absPath)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "写入文件失败")
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		_ = out.Close()
		_ = os.Remove(absPath)
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "写入文件失败")
		return
	}
	_ = out.Close()

	c := middleware.UserFrom(r.Context())
	a, err := h.svc.CreateAttachment(r.Context(), models.Attachment{
		EntityType: "contract", EntityID: id, FileName: header.Filename,
		FilePath: relPath, FileSize: header.Size, Mime: header.Header.Get("Content-Type"),
		UploadedBy: c.UserID,
	})
	if err != nil {
		_ = os.Remove(absPath)
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "保存附件记录失败")
		return
	}
	resp.OK(w, a)
}

// DownloadAttachment GET /api/v1/attachments/:id/download（鉴权组内，非登录不可下载）
func (h *ContractHandler) DownloadAttachment(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	a, err := h.svc.GetAttachment(r.Context(), id)
	if err != nil {
		h.failSvc(w, err)
		return
	}
	if !h.canAccess(w, r, a.EntityID) {
		return
	}
	abs := filepath.Join(h.uploadDir, a.FilePath)
	if _, err := os.Stat(abs); err != nil {
		resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, "附件文件不存在")
		return
	}
	w.Header().Set("Content-Type", a.Mime)
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename*=UTF-8''%s", url.QueryEscape(a.FileName)))
	http.ServeFile(w, r, abs)
}

// DeleteAttachment DELETE /api/v1/attachments/:id
func (h *ContractHandler) DeleteAttachment(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	a, err := h.svc.GetAttachment(r.Context(), id)
	if err != nil {
		h.failSvc(w, err)
		return
	}
	if !h.canAccess(w, r, a.EntityID) {
		return
	}
	if err := h.svc.DeleteAttachment(r.Context(), id); err != nil {
		h.failSvc(w, err)
		return
	}
	resp.OK(w, map[string]string{"message": "已删除"})
}

// ---------- 辅助 ----------

func (h *ContractHandler) canAccess(w http.ResponseWriter, r *http.Request, id uint) bool {
	ownerID, err := h.svc.OwnerOf(r.Context(), id)
	if err != nil {
		resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, "合同不存在")
		return false
	}
	allowed, err := services.CanAccessOwner(h.db, r.Context(), ownerID)
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "数据范围校验失败")
		return false
	}
	if !allowed {
		resp.Fail(w, http.StatusForbidden, resp.CodeForbidden, "无权访问该合同（不在你的数据范围内）")
		return false
	}
	return true
}

func (h *ContractHandler) failSvc(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, services.ErrContractInvalidTransition), errors.Is(err, services.ErrTerminateReasonRequired):
		resp.Fail(w, http.StatusUnprocessableEntity, resp.CodeInvalidStateTransit, err.Error())
	case errors.Is(err, services.ErrCrossCustomerLink):
		resp.Fail(w, http.StatusUnprocessableEntity, resp.CodeCrossCustomerLink, err.Error())
	case errors.Is(err, services.ErrContractFieldLocked):
		resp.Fail(w, http.StatusUnprocessableEntity, resp.CodeFieldLocked, err.Error())
	case errors.Is(err, services.ErrContractLocked):
		resp.Fail(w, http.StatusUnprocessableEntity, resp.CodeContractLocked, err.Error())
	case errors.Is(err, services.ErrContractHasChildren):
		resp.Fail(w, http.StatusUnprocessableEntity, resp.CodeContractHasChildren, err.Error())
	case errors.Is(err, services.ErrContractMissing), errors.Is(err, services.ErrAttachmentMissing):
		resp.Fail(w, http.StatusNotFound, resp.CodeNotFound, err.Error())
	default:
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
	}
}
