package projectfile_test

import (
	"testing"

	"github.com/insmtx/Leros/backend/internal/projectfile"
)

func TestResolveUserUploadRelativePath(t *testing.T) {
	path, err := projectfile.ResolveUserUploadRelativePath(
		"myProject/src/main.go",
		"main.go",
		"",
	)
	if err != nil {
		t.Fatalf("ResolveUserUploadRelativePath() error = %v", err)
	}
	if path != "uploads/myProject/src/main.go" {
		t.Fatalf("path = %q, want uploads/myProject/src/main.go", path)
	}
}

func TestApplyTaskUploadScope(t *testing.T) {
	path, err := projectfile.ApplyTaskUploadScope("uploads/demo/file.txt", "task_abc")
	if err != nil {
		t.Fatalf("ApplyTaskUploadScope() error = %v", err)
	}
	want := "uploads/_task/task_abc/demo/file.txt"
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}

func TestNormalizeFolderRelativePath(t *testing.T) {
	path, err := projectfile.NormalizeFolderRelativePath("uploads/demo")
	if err != nil {
		t.Fatalf("NormalizeFolderRelativePath() error = %v", err)
	}
	if path != "uploads/demo/" {
		t.Fatalf("path = %q, want uploads/demo/", path)
	}
}

func TestApplyTaskUploadScopeKeepsDistinctTasks(t *testing.T) {
	first, err := projectfile.ApplyTaskUploadScope("uploads/测试/测试/问题清单.xlsx", "task_a")
	if err != nil {
		t.Fatalf("ApplyTaskUploadScope(task_a) error = %v", err)
	}
	second, err := projectfile.ApplyTaskUploadScope("uploads/测试/新建文件夹/模板.xlsx", "task_b")
	if err != nil {
		t.Fatalf("ApplyTaskUploadScope(task_b) error = %v", err)
	}
	if first == second {
		t.Fatalf("expected distinct scoped paths, got %q", first)
	}
	if first != "uploads/_task/task_a/测试/测试/问题清单.xlsx" {
		t.Fatalf("first path = %q", first)
	}
	if second != "uploads/_task/task_b/测试/新建文件夹/模板.xlsx" {
		t.Fatalf("second path = %q", second)
	}
}

func TestResolveUserUploadRelativePathUsesMetadata(t *testing.T) {
	path, err := projectfile.ResolveUserUploadRelativePath("report.pdf", "report.pdf", "docs/report.pdf")
	if err != nil {
		t.Fatalf("ResolveUserUploadRelativePath() error = %v", err)
	}
	if path != "uploads/docs/report.pdf" {
		t.Fatalf("path = %q, want uploads/docs/report.pdf", path)
	}
}
