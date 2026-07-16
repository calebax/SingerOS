package db

import (
	"testing"

	"github.com/insmtx/Leros/backend/internal/consts"
)

func TestMatchesProjectFileExtGroup(t *testing.T) {
	t.Parallel()

	uploads := consts.RepoDirUploads
	artifacts := consts.RepoDirArtifacts

	cases := []struct {
		path   string
		group  string
		wantOK bool
	}{
		{uploads + "/report.pdf", "pdf", true},
		{uploads + "/report.PDF", "pdf", true},
		{uploads + "/report.docx", "pdf", false},
		{uploads + "/data.xlsx", "xlsx", true},
		{uploads + "/data.xls", "xlsx", true},
		{uploads + "/data.csv", "xlsx", true},
		{artifacts + "/readme.md", "md", true},
		{artifacts + "/readme.markdown", "md", true},
		{uploads + "/photo.jpg", "image", true},
		{uploads + "/config.json", "text", true},
	}

	for _, tc := range cases {
		got := matchesProjectFileExtGroup(tc.path, tc.group)
		if got != tc.wantOK {
			t.Fatalf("matchesProjectFileExtGroup(%q, %q) = %v, want %v", tc.path, tc.group, got, tc.wantOK)
		}
	}
}

func TestValidProjectFileExtFilter(t *testing.T) {
	t.Parallel()

	if !ValidProjectFileExtFilter("pdf") || ValidProjectFileExtFilter("unknown") {
		t.Fatal("ValidProjectFileExtFilter returned unexpected values")
	}
}
