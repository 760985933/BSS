package services

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"bss/internal/models"
	"bss/internal/pkg/code"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// uip 返回 uint 指针（构造 *uint 入参）
func uip(x uint) *uint { return &x }

func setupRecruitDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "r.db")), &gorm.Config{
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.Employee{}, &models.JobPost{}, &models.Candidate{}, &models.AuditLog{}); err != nil {
		t.Fatal(err)
	}
	// code_counters 无 GORM 模型（纯 SQL 计数器），测试库需手动建表
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS code_counters (
		prefix TEXT NOT NULL, year INTEGER NOT NULL, seq INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (prefix, year))`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCreateJobPostGeneratesCode(t *testing.T) {
	db := setupRecruitDB(t)
	gen := code.NewGenerator(db)
	ctx := context.Background()
	jp, err := CreateJobPost(ctx, db, gen, JobPostInput{Title: "后端工程师", Dept: "技术部", Headcount: 2}, 1)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if jp.Code == "" || len(jp.Code) < 6 {
		t.Fatalf("单号未生成: %q", jp.Code)
	}
	if jp.Status != models.JobOpen {
		t.Fatalf("默认状态应为 open, got %s", jp.Status)
	}
	// 第二单号应递增
	jp2, _ := CreateJobPost(ctx, db, gen, JobPostInput{Title: "前端"}, 1)
	if jp2.Code == jp.Code {
		t.Fatalf("单号未递增: %s == %s", jp.Code, jp2.Code)
	}
}

func TestCreateJobPostEmptyTitle(t *testing.T) {
	db := setupRecruitDB(t)
	gen := code.NewGenerator(db)
	if _, err := CreateJobPost(context.Background(), db, gen, JobPostInput{Title: ""}, 1); err == nil {
		t.Fatal("空标题应报错")
	}
}

func TestListJobPostsFilters(t *testing.T) {
	db := setupRecruitDB(t)
	gen := code.NewGenerator(db)
	ctx := context.Background()
	CreateJobPost(ctx, db, gen, JobPostInput{Title: "后端工程师", Status: "open"}, 1)
	CreateJobPost(ctx, db, gen, JobPostInput{Title: "产品经理", Status: "closed"}, 1)

	open, _ := ListJobPosts(ctx, db, "", "open")
	if len(open) != 1 || open[0].Status != "open" {
		t.Fatalf("status 过滤失败: %+v", open)
	}
	kw, _ := ListJobPosts(ctx, db, "产品", "")
	if len(kw) != 1 || kw[0].Title != "产品经理" {
		t.Fatalf("keyword 过滤失败: %+v", kw)
	}
}

func TestCreateCandidateDefaultsAndValidates(t *testing.T) {
	db := setupRecruitDB(t)
	gen := code.NewGenerator(db)
	ctx := context.Background()
	jp, _ := CreateJobPost(ctx, db, gen, JobPostInput{Title: "后端"}, 1)

	// 空名报错
	if _, err := CreateCandidate(ctx, db, CandidateInput{Name: ""}, 1); err == nil {
		t.Fatal("空姓名应报错")
	}
	// 关联不存在职位报错
	if _, err := CreateCandidate(ctx, db, CandidateInput{Name: "张三", JobPostID: uip(9999)}, 1); err == nil {
		t.Fatal("关联不存在职位应报错")
	}
	// 正常 + 默认阶段
	c, err := CreateCandidate(ctx, db, CandidateInput{Name: "张三", JobPostID: &jp.ID}, 1)
	if err != nil {
		t.Fatalf("create candidate: %v", err)
	}
	if c.Stage != models.CandApply {
		t.Fatalf("默认阶段应为 apply, got %s", c.Stage)
	}
}

func TestAdvanceCandidateTransitions(t *testing.T) {
	db := setupRecruitDB(t)
	gen := code.NewGenerator(db)
	ctx := context.Background()
	_, _ = CreateJobPost(ctx, db, gen, JobPostInput{Title: "后端"}, 1)
	c, _ := CreateCandidate(ctx, db, CandidateInput{Name: "张三"}, 1)

	// 单步前进 apply->screen
	if err := AdvanceCandidate(ctx, db, c.ID, models.CandScreen, false); err != nil {
		t.Fatalf("单步前进失败: %v", err)
	}
	// 跳级 screen->offer 需 force
	if err := AdvanceCandidate(ctx, db, c.ID, models.CandOffer, false); !errors.Is(err, ErrStageForceRequired) {
		t.Fatalf("跳级无 force 应报 ErrStageForceRequired, got %v", err)
	}
	if err := AdvanceCandidate(ctx, db, c.ID, models.CandOffer, true); err != nil {
		t.Fatalf("跳级带 force 应成功: %v", err)
	}
	// 任意非终态 -> rejected 允许
	if err := AdvanceCandidate(ctx, db, c.ID, models.CandRejected, false); err != nil {
		t.Fatalf("转淘汰失败: %v", err)
	}
	// 终态锁定
	if err := AdvanceCandidate(ctx, db, c.ID, models.CandApply, true); !errors.Is(err, ErrStageTerminal) {
		t.Fatalf("终态应锁定 ErrStageTerminal, got %v", err)
	}
}

func TestFunnelStats(t *testing.T) {
	db := setupRecruitDB(t)
	gen := code.NewGenerator(db)
	ctx := context.Background()
	_, _ = CreateJobPost(ctx, db, gen, JobPostInput{Title: "后端"}, 1)
	CreateCandidate(ctx, db, CandidateInput{Name: "A"}, 1)
	CreateCandidate(ctx, db, CandidateInput{Name: "B"}, 1)
	CreateCandidate(ctx, db, CandidateInput{Name: "C"}, 1)
	// 推进 B 到 screen
	b, _ := ListCandidates(ctx, db, "B", 0, "")
	if err := AdvanceCandidate(ctx, db, b[0].ID, models.CandScreen, false); err != nil {
		t.Fatalf("推进 B 失败: %v", err)
	}

	stats, err := CandidateFunnelStats(ctx, db, 0)
	if err != nil {
		t.Fatalf("funnel: %v", err)
	}
	var applyCnt, screenCnt int64
	for _, s := range stats {
		if s.Stage == models.CandApply {
			applyCnt = s.Count
		}
		if s.Stage == models.CandScreen {
			screenCnt = s.Count
		}
	}
	if applyCnt != 2 || screenCnt != 1 {
		t.Fatalf("漏斗计数错误: apply=%d screen=%d", applyCnt, screenCnt)
	}
}

func TestDeleteJobPostUnlinksCandidates(t *testing.T) {
	db := setupRecruitDB(t)
	gen := code.NewGenerator(db)
	ctx := context.Background()
	jp, _ := CreateJobPost(ctx, db, gen, JobPostInput{Title: "后端"}, 1)
	c, _ := CreateCandidate(ctx, db, CandidateInput{Name: "张三", JobPostID: &jp.ID}, 1)

	if err := DeleteJobPost(ctx, db, jp.ID); err != nil {
		t.Fatalf("delete job post: %v", err)
	}
	// 职位已软删
	if _, err := GetJobPost(ctx, db, jp.ID); !errors.Is(err, ErrJobPostMissing) {
		t.Fatalf("职位应已软删, err=%v", err)
	}
	// 候选人解除关联（job_post_id=NULL）但仍在
	still, err := GetCandidate(ctx, db, c.ID)
	if err != nil {
		t.Fatalf("候选人应仍在: %v", err)
	}
	if still.JobPostID != nil {
		t.Fatalf("候选人应解除职位关联, job_post_id=%v", still.JobPostID)
	}
}
