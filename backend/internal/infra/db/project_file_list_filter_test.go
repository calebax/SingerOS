package db

import (
	"testing"

	"github.com/insmtx/Leros/backend/types"
)

func TestMatchesProjectFileExtGroup(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path   string
		group  string
		wantOK bool
	}{
		{"uploads/report.pdf", "pdf", true},
		{"uploads/report.PDF", "pdf", true},
		{"uploads/report.docx", "pdf", false},
		{"uploads/data.xlsx", "xlsx", true},
		{"uploads/data.xls", "xlsx", true},
		{"uploads/data.csv", "xlsx", true},
		{"artifacts/readme.md", "md", true},
		{"artifacts/readme.markdown", "md", true},
		{"uploads/photo.jpg", "image", true},
		{"uploads/config.json", "text", true},
	}

	for _, tc := range cases {
		got := matchesProjectFileExtGroup(tc.path, tc.group)
		if got != tc.wantOK {
			t.Fatalf("matchesProjectFileExtGroup(%q, %q) = %v, want %v", tc.path, tc.group, got, tc.wantOK)
		}
	}
}

func TestFolderPathsForFile(t *testing.T) {
	t.Parallel()

	paths := folderPathsForFile("uploads/demo/report.pdf")
	want := []string{"uploads/", "uploads/demo/"}
	if len(paths) != len(want) {
		t.Fatalf("folderPathsForFile len = %d, want %d (%v)", len(paths), len(want), paths)
	}
	for i, path := range paths {
		if path != want[i] {
			t.Fatalf("folderPathsForFile[%d] = %q, want %q", i, path, want[i])
		}
	}
}

func TestFilterFoldersForFiles(t *testing.T) {
	t.Parallel()

	folders := []types.ProjectFile{
		{RelativePath: "uploads/"},
		{RelativePath: "uploads/demo/"},
		{RelativePath: "uploads/other/"},
		{RelativePath: "artifacts/"},
	}
	files := []types.ProjectFile{
		{RelativePath: "uploads/demo/report.pdf"},
	}

	filtered := filterFoldersForFiles(folders, files)
	if len(filtered) != 2 {
		t.Fatalf("filterFoldersForFiles len = %d, want 2 (%#v)", len(filtered), filtered)
	}
	if filtered[0].RelativePath != "uploads/" || filtered[1].RelativePath != "uploads/demo/" {
		t.Fatalf("filterFoldersForFiles = %#v", filtered)
	}
}

func TestValidProjectFileExtFilter(t *testing.T) {
	t.Parallel()

	if !ValidProjectFileExtFilter("pdf") || ValidProjectFileExtFilter("unknown") {
		t.Fatal("ValidProjectFileExtFilter returned unexpected values")
	}
}

func TestFilterFilesUnderFolderPaths(t *testing.T) {
	t.Parallel()

	folders := []types.ProjectFile{
		{RelativePath: "uploads/"},
		{RelativePath: "uploads/demo/"},
	}
	files := []types.ProjectFile{
		{RelativePath: "uploads/demo/report.pdf"},
		{RelativePath: "uploads/notes.docx"},
		{RelativePath: "artifacts/summary.md"},
	}

	filtered := filterFilesUnderFolderPaths(files, folders)
	if len(filtered) != 2 {
		t.Fatalf("filterFilesUnderFolderPaths len = %d, want 2 (%#v)", len(filtered), filtered)
	}
}
