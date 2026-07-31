package handlers

import (
	"net/http"
	"strings"

	"bss/internal/middleware"
	"bss/internal/pkg/resp"
	"bss/internal/services"

	"gorm.io/gorm"
)

// ImportHandler Excel 批量导入（M3-2）
type ImportHandler struct {
	db  *gorm.DB
	svc *services.ImportService
}

func NewImportHandler(db *gorm.DB) *ImportHandler {
	return &ImportHandler{db: db, svc: services.NewImportService(db)}
}

// MaxImportSize 导入文件上限 10MB
const MaxImportSize = 10 << 20

// ImportCustomers POST /api/v1/imports/customers（multipart，字段 file）
func (h *ImportHandler) ImportCustomers(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(MaxImportSize + 1<<20); err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请求格式错误（需 multipart/form-data）")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "请选择要导入的 Excel 文件")
		return
	}
	defer file.Close()

	if !strings.HasSuffix(strings.ToLower(header.Filename), ".xlsx") {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "仅支持 .xlsx 格式（不支持旧版 .xls）")
		return
	}
	if header.Size > MaxImportSize {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, "文件过大（上限 10MB）")
		return
	}

	c := middleware.UserFrom(r.Context())
	res, err := h.svc.ImportCustomers(r.Context(), file, c.UserID, c.Role)
	if err != nil {
		resp.Fail(w, http.StatusBadRequest, resp.CodeBadRequest, err.Error())
		return
	}
	resp.OK(w, res)
}

// Template GET /api/v1/imports/customers/template —— 下载导入模板
func (h *ImportHandler) Template(w http.ResponseWriter, r *http.Request) {
	data, err := h.svc.GenerateTemplate()
	if err != nil {
		resp.Fail(w, http.StatusInternalServerError, resp.CodeInternal, "生成模板失败")
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''customers_import_template.xlsx")
	_, _ = w.Write(data)
}
