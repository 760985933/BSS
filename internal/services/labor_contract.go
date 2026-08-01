package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"bss/internal/models"
	"bss/internal/pkg/code"

	"gorm.io/gorm"
)

// 劳动合同相关错误（ErrEmployeeMissing / ErrCandidateMissing 复用 employee / recruitment 服务声明）
var (
	ErrLaborContractMissing      = errors.New("劳动合同不存在")
	ErrLCStatusTerminal          = errors.New("劳动合同已为终态，不可变更")
	ErrLCTransitionForceReq      = errors.New("该状态流转需二次确认")
	ErrLCInvalidTransition       = errors.New("非法的状态流转")
	ErrLCTerminateReasonRequired = errors.New("解除劳动合同时必须填写终止原因")
	ErrOnboardingMissing         = errors.New("入职记录不存在")
)

// 合同状态机：仅允许的正向流转
var lcForward = map[string][]string{
	models.LCStatusDraft:  {models.LCStatusActive},
	models.LCStatusActive: {models.LCStatusExpired, models.LCStatusRenewed, models.LCStatusTerminated},
}

// 允许在 force 下进行的"回退/特殊"流转
var lcForceAllowed = map[string][]string{
	models.LCStatusActive: {models.LCStatusDraft},  // 生效中回退草稿
	models.LCStatusDraft:  {models.LCStatusTerminated}, // 草稿直接作废
}

// LaborContractInput 劳动合同可写字段
type LaborContractInput struct {
	EmployeeID      uint   `json:"employee_id,string"`
	Type            string `json:"type"`
	StartDate       string `json:"start_date"`
	EndDate         string `json:"end_date"`
	SignDate        string `json:"sign_date"`
	ProbationMonths int    `json:"probation_months"`
	SalaryCent      *int64 `json:"salary_cent,omitempty"` // 月薪（分），可选
}

func getEmployee(db *gorm.DB, id uint) (*models.Employee, error) {
	var e models.Employee
	if err := db.First(&e, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrEmployeeMissing
		}
		return nil, err
	}
	return &e, nil
}

func parseDate(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, fmt.Errorf("日期格式应为 YYYY-MM-DD: %w", err)
	}
	return &t, nil
}

// CreateLaborContract 新建劳动合同（默认 draft 状态，生成 LC- 编号）
func CreateLaborContract(ctx context.Context, db *gorm.DB, gen *code.Generator, in LaborContractInput, ownerID uint) (*models.LaborContract, error) {
	if in.EmployeeID == 0 {
		return nil, ErrEmployeeMissing
	}
	if _, err := getEmployee(db, in.EmployeeID); err != nil {
		return nil, err
	}
	if in.Type == "" {
		in.Type = models.LCTypeFixed
	}
	sd, err := parseDate(in.StartDate)
	if err != nil {
		return nil, err
	}
	ed, err := parseDate(in.EndDate)
	if err != nil {
		return nil, err
	}
	sg, err := parseDate(in.SignDate)
	if err != nil {
		return nil, err
	}
	c, err := gen.Next(ctx, code.PrefixLaborContract)
	if err != nil {
		return nil, err
	}
	lc := models.LaborContract{
		Code:            c,
		EmployeeID:      in.EmployeeID,
		Type:            in.Type,
		StartDate:       sd,
		EndDate:         ed,
		SignDate:        sg,
		ProbationMonths: in.ProbationMonths,
		Status:          models.LCStatusDraft,
		OwnerID:         ownerID,
	}
	if in.SalaryCent != nil {
		lc.SalaryCent = *in.SalaryCent
	}
	if err := db.WithContext(ctx).Create(&lc).Error; err != nil {
		return nil, err
	}
	return GetLaborContract(ctx, db, lc.ID)
}

// GetLaborContract 详情（预加载员工/录入人）
func GetLaborContract(ctx context.Context, db *gorm.DB, id uint) (*models.LaborContract, error) {
	var lc models.LaborContract
	if err := db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).
		Preload("Employee").Preload("Owner").First(&lc).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrLaborContractMissing
		}
		return nil, err
	}
	return &lc, nil
}

// ListLaborContracts 列表（关键字/状态/员工过滤）
func ListLaborContracts(ctx context.Context, db *gorm.DB, keyword, status, employeeID string) ([]models.LaborContract, error) {
	q := db.WithContext(ctx).Where("deleted_at IS NULL").Preload("Employee").Preload("Owner")
	if keyword != "" {
		q = q.Where("code LIKE ? OR terminate_reason LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if employeeID != "" {
		q = q.Where("employee_id = ?", employeeID)
	}
	var list []models.LaborContract
	if err := q.Order("id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// UpdateLaborContract 编辑合同（仅非终态可改核心字段）
func UpdateLaborContract(ctx context.Context, db *gorm.DB, id uint, in LaborContractInput) (*models.LaborContract, error) {
	var lc models.LaborContract
	if err := db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&lc).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrLaborContractMissing
		}
		return nil, err
	}
	if lc.IsTerminal() {
		return nil, ErrLCStatusTerminal
	}
	if in.EmployeeID != 0 && in.EmployeeID != lc.EmployeeID {
		if _, err := getEmployee(db, in.EmployeeID); err != nil {
			return nil, err
		}
	}
	sd, err := parseDate(in.StartDate)
	if err != nil {
		return nil, err
	}
	ed, err := parseDate(in.EndDate)
	if err != nil {
		return nil, err
	}
	sg, err := parseDate(in.SignDate)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{
		"start_date":       sd,
		"end_date":         ed,
		"sign_date":        sg,
		"probation_months": in.ProbationMonths,
	}
	if in.SalaryCent != nil {
		updates["salary_cent"] = *in.SalaryCent
	}
	if in.EmployeeID != 0 {
		updates["employee_id"] = in.EmployeeID
	}
	if in.Type != "" {
		updates["type"] = in.Type
	}
	if err := db.WithContext(ctx).Model(&lc).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetLaborContract(ctx, db, id)
}

// DeleteLaborContract 软删
func DeleteLaborContract(ctx context.Context, db *gorm.DB, id uint) error {
	res := db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).Delete(&models.LaborContract{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrLaborContractMissing
	}
	return nil
}

// TransitionLaborContract 合同状态流转
// to = terminated 时 reason 必填；回退/草稿作废需 force 二次确认。
func TransitionLaborContract(ctx context.Context, db *gorm.DB, id uint, to, reason string, force bool) (*models.LaborContract, error) {
	var lc models.LaborContract
	if err := db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&lc).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrLaborContractMissing
		}
		return nil, err
	}
	if lc.IsTerminal() {
		return nil, ErrLCStatusTerminal
	}
	if to == lc.Status {
		return GetLaborContract(ctx, db, id)
	}
	direct := false
	for _, s := range lcForward[lc.Status] {
		if s == to {
			direct = true
			break
		}
	}
	forceOK := false
	for _, s := range lcForceAllowed[lc.Status] {
		if s == to {
			forceOK = true
			break
		}
	}
	if !direct && !forceOK {
		return nil, ErrLCInvalidTransition
	}
	if !direct && forceOK && !force {
		return nil, ErrLCTransitionForceReq
	}
	if to == models.LCStatusTerminated && strings.TrimSpace(reason) == "" {
		return nil, ErrLCTerminateReasonRequired
	}
	updates := map[string]any{"status": to}
	if to == models.LCStatusTerminated {
		updates["terminate_reason"] = strings.TrimSpace(reason)
	}
	if err := db.WithContext(ctx).Model(&lc).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetLaborContract(ctx, db, id)
}

// OnboardingInput 入职可写字段
type OnboardingInput struct {
	EmployeeID   uint  `json:"employee_id,string"`
	CandidateID  *uint `json:"candidate_id,string,omitempty"`
	StepProfile  string `json:"step_profile"`
	StepEquip    string `json:"step_equip"`
	StepTraining string `json:"step_training"`
	StepProbation string `json:"step_probation"`
}

func (in OnboardingInput) normStep(s string) string {
	if s == models.OBStepDone {
		return models.OBStepDone
	}
	return models.OBStepPending
}

// computeStatus 四步全 done 即 completed，否则 in_progress
func (in OnboardingInput) computeStatus() string {
	if in.normStep(in.StepProfile) == models.OBStepDone &&
		in.normStep(in.StepEquip) == models.OBStepDone &&
		in.normStep(in.StepTraining) == models.OBStepDone &&
		in.normStep(in.StepProbation) == models.OBStepDone {
		return models.OBStatusCompleted
	}
	return models.OBStatusInProgress
}

// CreateOnboarding 新建入职记录（默认四步 pending / in_progress）
func CreateOnboarding(ctx context.Context, db *gorm.DB, gen *code.Generator, in OnboardingInput, ownerID uint) (*models.Onboarding, error) {
	if in.EmployeeID == 0 {
		return nil, ErrEmployeeMissing
	}
	if _, err := getEmployee(db, in.EmployeeID); err != nil {
		return nil, err
	}
	if in.CandidateID != nil {
		if _, err := GetCandidate(ctx, db, *in.CandidateID); err != nil {
			return nil, ErrCandidateMissing
		}
	}
	c, err := gen.Next(ctx, code.PrefixOnboarding)
	if err != nil {
		return nil, err
	}
	ob := models.Onboarding{
		Code:         c,
		EmployeeID:   in.EmployeeID,
		CandidateID:  in.CandidateID,
		StepProfile:  in.normStep(in.StepProfile),
		StepEquip:    in.normStep(in.StepEquip),
		StepTraining: in.normStep(in.StepTraining),
		StepProbation: in.normStep(in.StepProbation),
		Status:       models.OBStatusInProgress,
		OwnerID:      ownerID,
	}
	if err := db.WithContext(ctx).Create(&ob).Error; err != nil {
		return nil, err
	}
	return GetOnboarding(ctx, db, ob.ID)
}

// GetOnboarding 详情（预加载员工/候选人/录入人）
func GetOnboarding(ctx context.Context, db *gorm.DB, id uint) (*models.Onboarding, error) {
	var ob models.Onboarding
	if err := db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).
		Preload("Employee").Preload("Candidate").Preload("Owner").First(&ob).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOnboardingMissing
		}
		return nil, err
	}
	return &ob, nil
}

// ListOnboardings 列表（关键字/状态/员工过滤）
func ListOnboardings(ctx context.Context, db *gorm.DB, keyword, status, employeeID string) ([]models.Onboarding, error) {
	q := db.WithContext(ctx).Where("deleted_at IS NULL").Preload("Employee").Preload("Candidate").Preload("Owner")
	if keyword != "" {
		q = q.Where("code LIKE ?", "%"+keyword+"%")
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if employeeID != "" {
		q = q.Where("employee_id = ?", employeeID)
	}
	var list []models.Onboarding
	if err := q.Order("id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// UpdateOnboarding 编辑入职（重算总状态）
func UpdateOnboarding(ctx context.Context, db *gorm.DB, id uint, in OnboardingInput) (*models.Onboarding, error) {
	var ob models.Onboarding
	if err := db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&ob).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOnboardingMissing
		}
		return nil, err
	}
	if in.EmployeeID != 0 && in.EmployeeID != ob.EmployeeID {
		if _, err := getEmployee(db, in.EmployeeID); err != nil {
			return nil, err
		}
	}
	if in.CandidateID != nil {
		if ob.CandidateID == nil || *in.CandidateID != *ob.CandidateID {
			if _, err := GetCandidate(ctx, db, *in.CandidateID); err != nil {
				return nil, ErrCandidateMissing
			}
		}
	}
	updates := map[string]any{
		"step_profile":  in.normStep(in.StepProfile),
		"step_equip":    in.normStep(in.StepEquip),
		"step_training": in.normStep(in.StepTraining),
		"step_probation": in.normStep(in.StepProbation),
		"status":        in.computeStatus(),
		"candidate_id":  in.CandidateID,
	}
	if in.EmployeeID != 0 {
		updates["employee_id"] = in.EmployeeID
	}
	if err := db.WithContext(ctx).Model(&ob).Updates(updates).Error; err != nil {
		return nil, err
	}
	return GetOnboarding(ctx, db, id)
}

// DeleteOnboarding 软删
func DeleteOnboarding(ctx context.Context, db *gorm.DB, id uint) error {
	res := db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).Delete(&models.Onboarding{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrOnboardingMissing
	}
	return nil
}

// GetActiveContractForEmployee 取员工当前生效中的劳动合同（无则 nil, nil）
func GetActiveContractForEmployee(ctx context.Context, db *gorm.DB, employeeID uint) (*models.LaborContract, error) {
	var lc models.LaborContract
	err := db.WithContext(ctx).Where("employee_id = ? AND status = ? AND deleted_at IS NULL",
		employeeID, models.LCStatusActive).Order("id DESC").First(&lc).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &lc, nil
}

// ScanLaborContractReminders 扫描生效中劳动合同并生成提醒：
//   - 30 天内到期 → NotifLaborExpiring
//   - 试用期临近转正（7 天内）→ NotifLaborProbation
//
// 返回新建的通知列表（由 ScanReminders 统一去重外发）。
func ScanLaborContractReminders(ctx context.Context, db *gorm.DB, now time.Time) ([]models.Notification, error) {
	today := now.Format("2006-01-02")
	limit := now.AddDate(0, 0, 30).Format("2006-01-02")
	probWindow := now.AddDate(0, 0, 7).Format("2006-01-02")
	var out []models.Notification

	// 1) 到期提醒
	var expiring []models.LaborContract
	if err := db.Where("status = ? AND end_date >= ? AND end_date <= ? AND deleted_at IS NULL",
		models.LCStatusActive, today, limit).Find(&expiring).Error; err != nil {
		return nil, err
	}
	for _, c := range expiring {
		if c.OwnerID == 0 || c.EndDate == nil {
			continue
		}
		ed := c.EndDate.Format("2006-01-02")
		key := fmt.Sprintf("%s|%d|%s", NotifLaborExpiring, c.ID, ed)
		if existsNotification(db, key) {
			continue
		}
		n := models.Notification{
			UserID:     c.OwnerID,
			Type:       NotifLaborExpiring,
			Title:      fmt.Sprintf("劳动合同 %s 将于 %s 到期", c.Code, ed),
			Content:    fmt.Sprintf("员工 %s 的劳动合同（%s）到期日 %s，请关注续签。", lookupEmpName(db, c.EmployeeID), c.Code, ed),
			EntityType: "labor_contract",
			EntityID:   c.ID,
			DedupKey:   key,
		}
		if err := db.Create(&n).Error; err != nil {
			return out, err
		}
		out = append(out, n)
	}

	// 2) 试用期转正提醒
	var active []models.LaborContract
	if err := db.Where("status = ? AND probation_months > 0 AND start_date IS NOT NULL AND deleted_at IS NULL",
		models.LCStatusActive).Find(&active).Error; err != nil {
		return nil, err
	}
	for _, c := range active {
		if c.OwnerID == 0 || c.StartDate == nil {
			continue
		}
		reg := c.StartDate.AddDate(0, c.ProbationMonths, 0)
		regS := reg.Format("2006-01-02")
		if regS < today || regS > probWindow {
			continue
		}
		key := fmt.Sprintf("%s|%d|%s", NotifLaborProbation, c.ID, regS)
		if existsNotification(db, key) {
			continue
		}
		emp := lookupEmpName(db, c.EmployeeID)
		n := models.Notification{
			UserID:     c.OwnerID,
			Type:       NotifLaborProbation,
			Title:      fmt.Sprintf("员工 %s 试用期将于 %s 届满", emp, regS),
			Content:    fmt.Sprintf("员工 %s 的劳动合同（%s）试用期将于 %s 届满，请准备转正评估。", emp, c.Code, regS),
			EntityType: "labor_contract",
			EntityID:   c.ID,
			DedupKey:   key,
		}
		if err := db.Create(&n).Error; err != nil {
			return out, err
		}
		out = append(out, n)
	}
	return out, nil
}

func lookupEmpName(db *gorm.DB, id uint) string {
	if id == 0 {
		return "未知"
	}
	var e models.Employee
	if err := db.First(&e, id).Error; err != nil {
		return "未知"
	}
	if e.Name == "" {
		return "未知"
	}
	return e.Name
}
