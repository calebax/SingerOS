package vision_analyze

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insmtx/Leros/backend/tools"
)

func TestExecuteRendersPDFBeforeChatCompletions(t *testing.T) {
	repo := t.TempDir()
	pdfPath := filepath.Join(repo, "uploads", "attendance.pdf")
	if err := os.MkdirAll(filepath.Dir(pdfPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pdfPath, []byte("%PDF-test"), 0o600); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/chat/completions" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		messages := body["messages"].([]any)
		content := messages[0].(map[string]any)["content"].([]any)
		imagePart := content[2].(map[string]any)
		if imagePart["type"] != "image_url" {
			t.Fatalf("image part = %#v", imagePart)
		}
		image := imagePart["image_url"].(map[string]any)
		if !strings.HasPrefix(image["url"].(string), "data:image/png;base64,") {
			t.Fatalf("rendered PDF image payload = %#v", image)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"{\"records\":[{\"name\":\"测试\",\"actual_work_days\":20}]}"}}]}`))
	}))
	defer server.Close()

	tool := NewTool()
	tool.client = server.Client()
	previousRenderer := renderPDFPages
	renderPDFPages = func(_ context.Context, _ string) ([]renderedPage, error) {
		return []renderedPage{{name: "page-1.png", data: []byte("fake-png")}}, nil
	}
	defer func() { renderPDFPages = previousRenderer }()
	ctx := tools.ContextWithToolContext(context.Background(), tools.ToolContext{
		Metadata: tools.ToolMetadata{
			RepoDir: repo,
			Model: tools.ToolModelMetadata{
				Model:   "tokenplan/qwen3.7-plus",
				APIKey:  "test-key",
				BaseURL: server.URL,
				Vision:  true,
			},
		},
	})
	output, err := tool.Execute(ctx, tools.JSONInput(map[string]any{
		"files":    []string{"uploads/attendance.pdf"},
		"prompt":   "extract attendance",
		"protocol": "chat_completions",
	}))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if output != `{"records":[{"actual_work_days":20,"holiday_attendance_dates":[],"name":"测试","weekend_attendance_dates":[]}]}` {
		t.Fatalf("output = %q", output)
	}

	output, err = tool.Execute(ctx, tools.JSONInput(map[string]any{
		"files":       []string{"uploads/attendance.pdf"},
		"prompt":      "extract attendance",
		"protocol":    "chat_completions",
		"output_file": "outputs/attendance-vision.json",
	}))
	if err != nil {
		t.Fatalf("Execute() with output_file error = %v", err)
	}
	if !strings.Contains(output, `"output_file":"outputs/attendance-vision.json"`) {
		t.Fatalf("output_file response = %q", output)
	}
	data, err := os.ReadFile(filepath.Join(repo, "outputs", "attendance-vision.json"))
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if string(data) != `{"records":[{"actual_work_days":20,"holiday_attendance_dates":[],"name":"测试","weekend_attendance_dates":[]}]}` {
		t.Fatalf("saved output = %q", data)
	}
}

func TestExecuteRejectsOutsideRepositoryFile(t *testing.T) {
	tool := NewTool()
	ctx := tools.ContextWithToolContext(context.Background(), tools.ToolContext{
		Metadata: tools.ToolMetadata{
			RepoDir: t.TempDir(),
			Model: tools.ToolModelMetadata{
				Model:   "model",
				APIKey:  "key",
				BaseURL: "http://127.0.0.1",
				Vision:  true,
			},
		},
	})
	_, err := tool.Execute(ctx, tools.JSONInput(map[string]any{
		"files":  []string{"../outside.pdf"},
		"prompt": "extract",
	}))
	if err == nil || !strings.Contains(err.Error(), "inside the project repository") {
		t.Fatalf("error = %v", err)
	}
}

func TestMergeVisionResponsesAcceptsPeopleAlias(t *testing.T) {
	output, err := mergeVisionResponses([]string{
		`{"month":"2026-06","people":[{"name":"甲","actual_attendance":26}]}`,
		`{"employees":[{"name":"乙"}]}`,
	})
	if err != nil {
		t.Fatalf("mergeVisionResponses() error = %v", err)
	}
	var payload struct {
		Records []map[string]any `json:"records"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("decode merged output: %v", err)
	}
	if len(payload.Records) != 2 {
		t.Fatalf("records = %#v, want two records", payload.Records)
	}
	if payload.Records[0]["actual_work_days"] != float64(26) {
		t.Fatalf("normalized attendance = %#v", payload.Records[0])
	}
}

func TestMergeVisionResponsesAllowsEmptySegment(t *testing.T) {
	output, err := mergeVisionResponses([]string{
		`{"month":"2026-06","project":"瀚阅府","summary":"header only"}`,
		`{"records":[{"name":"程军虎","actual_attendance":26}]}`,
	})
	if err != nil {
		t.Fatalf("mergeVisionResponses() error = %v", err)
	}
	if !strings.Contains(output, `"segment_warnings":["segment 1 returned no records array"]`) {
		t.Fatalf("warnings = %q", output)
	}
	if !strings.Contains(output, `"actual_work_days":26`) {
		t.Fatalf("normalized output = %q", output)
	}
}
