// Package vision_analyze provides a run-scoped direct multimodal model call.
package vision_analyze

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/insmtx/Leros/backend/tools"
)

const (
	// ToolName is the stable name exposed to Skills.
	ToolName = "vision_analyze"
	// ToolDescription explains that the tool sends image content to the configured model.
	ToolDescription = `Call the configured vision model with images or PDF pages.
PDF files are rendered page by page with the worker's Python PyMuPDF runtime before being sent as images.
Use this for image/PDF understanding before deterministic business calculations.
Files and optional output files must be inside the current project repository. When output_file is provided, the complete model response is written there and the tool returns compact metadata instead of the response body. The tool never exposes the API key.`
	maxFileBytes = 100 * 1024 * 1024
	maxPDFPages  = 50
)

type renderedPage struct {
	name string
	data []byte
}

var renderPDFPages = renderPDF

type Tool struct {
	tools.BaseTool
	client *http.Client
}

type analyzeInput struct {
	Files      []string `json:"files"`
	Prompt     string   `json:"prompt"`
	Protocol   string   `json:"protocol,omitempty"`
	JSONOnly   bool     `json:"json_only,omitempty"`
	OutputFile string   `json:"output_file,omitempty"`
}

// NewTool creates the direct vision model tool.
func NewTool() *Tool {
	return &Tool{
		BaseTool: tools.NewBaseTool(
			ToolName,
			ToolDescription,
			tools.Schema{
				Type:     "object",
				Required: []string{"files", "prompt"},
				Properties: map[string]*tools.Property{
					"files": {
						Type:        "array",
						Description: "Absolute or repository-relative image/PDF paths.",
						Items:       &tools.Property{Type: "string"},
					},
					"prompt": {
						Type:        "string",
						Description: "Precise extraction instructions. Ask for evidence and structured JSON when appropriate.",
					},
					"protocol": {
						Type:        "string",
						Description: "Optional provider protocol: chat_completions (default) or responses.",
						Enum:        []string{"chat_completions", "responses"},
					},
					"json_only": {
						Type:        "boolean",
						Description: "Ask the model to return JSON only.",
					},
					"output_file": {
						Type:        "string",
						Description: "Optional repository-relative path. When set, save the model JSON directly to this file and return only its path and size.",
					},
				},
			},
		),
		client: &http.Client{Timeout: 120 * time.Second},
	}
}

func (t *Tool) Validate(raw json.RawMessage) error {
	input, err := tools.DecodeInput(raw)
	if err != nil {
		return err
	}
	parsed, err := parseInput(input)
	if err != nil {
		return err
	}
	if len(parsed.Files) == 0 {
		return fmt.Errorf("files must not be empty")
	}
	return nil
}

func (t *Tool) Execute(ctx context.Context, raw json.RawMessage) (string, error) {
	input, err := tools.DecodeInput(raw)
	if err != nil {
		return "", err
	}
	parsed, err := parseInput(input)
	if err != nil {
		return "", err
	}
	toolCtx, err := tools.RequireToolContext(ctx)
	if err != nil {
		return "", err
	}
	model := toolCtx.Metadata.Model
	if strings.TrimSpace(model.APIKey) == "" {
		return "", fmt.Errorf("selected model API key is unavailable")
	}
	if strings.TrimSpace(model.BaseURL) == "" || strings.TrimSpace(model.Model) == "" {
		return "", fmt.Errorf("selected model base URL and model are required")
	}
	if !model.Vision {
		return "", fmt.Errorf("selected model %q is not marked as vision capable; enable vision in its model configuration", model.Model)
	}
	protocol := strings.ToLower(strings.TrimSpace(parsed.Protocol))
	prompt := parsed.Prompt
	prompt += "\nReturn compact JSON only. Do not add explanations, Markdown fences, repeated table headers, or duplicate records. Keep one concise record per person. Encode daily marks as a compact date-to-symbol object or string, omit empty/unknown fields, and preserve only required page/row evidence."
	if parsed.JSONOnly {
		prompt += "\nReturn valid JSON only, without Markdown fences."
	}
	baseParts := []map[string]any{{"type": "text", "text": prompt}}
	pageParts := make([][]map[string]any, 0)
	for _, file := range parsed.Files {
		path, err := resolveRepositoryFile(toolCtx.Metadata.RepoDir, file)
		if err != nil {
			return "", err
		}
		if strings.EqualFold(filepath.Ext(path), ".pdf") {
			pages, err := renderPDFPages(ctx, path)
			if err != nil {
				return "", err
			}
			for _, page := range pages {
				parts := appendVisionPage(baseParts, page)
				parts[0] = map[string]any{
					"type": "text",
					"text": prompt + "\nProcess only this PDF page segment. Return only the people and evidence visible in this segment; do not wait for or describe other pages. For compactness, do not output normal daily marks for every date. Return actual_work_days, leave/absence totals, weekend_overtime_days or holiday_overtime_days, and only exceptional symbols/dates plus page/row evidence.",
				}
				pageParts = append(pageParts, parts)
			}
			continue
		}
		data, mimeType, err := readAttachment(path)
		if err != nil {
			return "", err
		}
		if strings.HasPrefix(mimeType, "image/") {
			baseParts = append(baseParts, map[string]any{
				"type":      "image_url",
				"image_url": map[string]any{"url": dataURL(mimeType, data)},
			})
			continue
		}
		return "", fmt.Errorf("unsupported vision attachment type: %s", mimeType)
	}

	responses := make([]string, 0, len(pageParts))
	if len(pageParts) == 0 {
		pageParts = append(pageParts, baseParts)
	}
	for index, parts := range pageParts {
		text, err := t.callModel(ctx, model, protocol, parts)
		if err != nil {
			return "", fmt.Errorf("vision page %d: %w", index+1, err)
		}
		responses = append(responses, text)
	}
	text, err := mergeVisionResponses(responses)
	if err != nil {
		return "", err
	}
	if parsed.OutputFile != "" {
		return saveVisionOutput(toolCtx.Metadata.RepoDir, parsed.OutputFile, text)
	}
	return text, nil
}

func appendVisionPage(base []map[string]any, page renderedPage) []map[string]any {
	parts := make([]map[string]any, 0, len(base)+2)
	parts = append(parts, base...)
	parts = append(parts, map[string]any{
		"type": "text",
		"text": "The following image is PDF page " + page.name + ". Preserve this page number in evidence.",
	})
	parts = append(parts, imagePart(page.name, page.data))
	return parts
}

func (t *Tool) callModel(
	ctx context.Context,
	model tools.ToolModelMetadata,
	protocol string,
	parts []map[string]any,
) (string, error) {
	var body any
	outputLimit := model.OutputLimit
	if outputLimit <= 0 {
		outputLimit = model.MaxTokens
	}
	if protocol == "" || protocol == "chat_completions" {
		body = map[string]any{
			"model":           model.Model,
			"enable_thinking": false,
			"messages": []map[string]any{{
				"role":    "user",
				"content": parts,
			}},
		}
		if outputLimit > 0 {
			body.(map[string]any)["max_tokens"] = outputLimit
		}
	} else if protocol == "responses" {
		body = map[string]any{
			"model":           model.Model,
			"enable_thinking": false,
			"input": []map[string]any{{
				"role":    "user",
				"content": responseParts(parts),
			}},
		}
		if outputLimit > 0 {
			body.(map[string]any)["max_output_tokens"] = outputLimit
		}
	} else {
		return "", fmt.Errorf("unsupported protocol %q", protocol)
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("encode vision request: %w", err)
	}
	endpoint := modelEndpoint(model.BaseURL, protocol)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("create vision request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+model.APIKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := t.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("call vision model: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 20*1024*1024))
	if err != nil {
		return "", fmt.Errorf("read vision response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("vision model returned HTTP %d", response.StatusCode)
	}
	text, err := extractResponseText(responseBody, protocol)
	if err != nil {
		return "", err
	}
	return text, nil
}

func mergeVisionResponses(responses []string) (string, error) {
	merged := map[string]any{"records": make([]any, 0)}
	records := merged["records"].([]any)
	for index, response := range responses {
		var payload any
		if err := json.Unmarshal([]byte(response), &payload); err != nil {
			return "", fmt.Errorf("vision page %d returned invalid JSON: %w", index+1, err)
		}
		switch value := payload.(type) {
		case []any:
			records = append(records, value...)
		case map[string]any:
			for _, field := range []string{"month", "project", "holiday_dates"} {
				if _, exists := merged[field]; !exists {
					if candidate, ok := value[field]; ok {
						merged[field] = candidate
					}
				}
			}
			pageRecords, ok := visionRecordArray(value)
			if ok {
				records = append(records, pageRecords...)
			} else {
				return "", fmt.Errorf("vision page %d JSON has no records array", index+1)
			}
		default:
			return "", fmt.Errorf("vision page %d JSON must be an object or array", index+1)
		}
	}
	merged["records"] = records
	data, err := json.Marshal(merged)
	if err != nil {
		return "", fmt.Errorf("merge vision JSON: %w", err)
	}
	return string(data), nil
}

func visionRecordArray(payload map[string]any) ([]any, bool) {
	for _, field := range []string{
		"records", "people", "personnel", "employees", "attendance", "data",
		"人员", "人员记录", "考勤记录", "员工", "人员信息",
	} {
		value, ok := payload[field]
		if !ok {
			continue
		}
		if records, ok := value.([]any); ok {
			return records, true
		}
		if nested, ok := value.(map[string]any); ok {
			if records, ok := visionRecordArray(nested); ok {
				return records, true
			}
		}
	}
	return nil, false
}

func saveVisionOutput(repoDir, input, text string) (string, error) {
	outputPath, err := resolveOutputFile(repoDir, input)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return "", fmt.Errorf("create vision output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(text), 0o600); err != nil {
		return "", fmt.Errorf("write vision JSON: %w", err)
	}
	relative, _ := filepath.Rel(repoDir, outputPath)
	return tools.JSONString(map[string]any{
		"ok":          true,
		"output_file": filepath.ToSlash(relative),
		"bytes":       len(text),
	})
}

func parseInput(input map[string]any) (analyzeInput, error) {
	if input == nil {
		return analyzeInput{}, fmt.Errorf("input is required")
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return analyzeInput{}, fmt.Errorf("encode input: %w", err)
	}
	var parsed analyzeInput
	if err := json.Unmarshal(encoded, &parsed); err != nil {
		return analyzeInput{}, fmt.Errorf("decode input: %w", err)
	}
	if len(parsed.Files) == 0 || strings.TrimSpace(parsed.Prompt) == "" {
		return analyzeInput{}, fmt.Errorf("files and prompt are required")
	}
	return parsed, nil
}

func imagePart(name string, data []byte) map[string]any {
	return map[string]any{
		"type": "image_url",
		"image_url": map[string]any{
			"url":    dataURL("image/png", data),
			"detail": "high",
		},
	}
}

func dataURL(mimeType string, data []byte) string {
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func renderPDF(ctx context.Context, path string) ([]renderedPage, error) {
	tempDir, err := os.MkdirTemp("", "leros-vision-pdf-*")
	if err != nil {
		return nil, fmt.Errorf("create PDF render directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	python, err := pythonExecutable()
	if err != nil {
		return nil, fmt.Errorf("PDF rendering requires Python with PyMuPDF: %w", err)
	}
	script := `import os, sys
try:
    import fitz
except ImportError as exc:
    raise SystemExit("PyMuPDF is not installed in the worker Python environment") from exc
source, target, limit = sys.argv[1], sys.argv[2], int(sys.argv[3])
document = fitz.open(source)
if len(document) > limit:
    raise RuntimeError("PDF exceeds page limit")
for index, page in enumerate(document, 1):
    height = page.rect.height
    width = page.rect.width
    for part in range(2):
        top = height * part / 2
        bottom = height * (part + 1) / 2
        clip = fitz.Rect(0, top, width, bottom)
        pixmap = page.get_pixmap(dpi=144, clip=clip, alpha=False)
        pixmap.save(os.path.join(target, f"page-{index}-part-{part + 1}.png"))
`
	cmd := exec.CommandContext(ctx, python, "-c", script, path, tempDir, strconv.Itoa(maxPDFPages))
	if output, runErr := cmd.CombinedOutput(); runErr != nil {
		return nil, fmt.Errorf("render PDF pages with PyMuPDF: %w: %s", runErr, strings.TrimSpace(string(output)))
	}
	return readRenderedPages(tempDir)
}

func pythonExecutable() (string, error) {
	var lastErr error
	for _, name := range []string{"python", "python3"} {
		path, err := exec.LookPath(name)
		if err != nil {
			lastErr = err
			continue
		}
		if err := exec.Command(path, "-c", "import fitz").Run(); err == nil {
			return path, nil
		} else {
			lastErr = err
		}
	}
	if lastErr != nil {
		return "", fmt.Errorf("python with PyMuPDF executable not found: %w", lastErr)
	}
	return "", fmt.Errorf("python with PyMuPDF executable not found")
}

func readRenderedPages(dir string) ([]renderedPage, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read rendered PDF pages: %w", err)
	}
	pages := make([]renderedPage, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".png") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		if readErr != nil {
			return nil, fmt.Errorf("read rendered page %q: %w", entry.Name(), readErr)
		}
		pages = append(pages, renderedPage{name: entry.Name(), data: data})
	}
	sort.Slice(pages, func(left, right int) bool {
		return pageNumber(pages[left].name) < pageNumber(pages[right].name)
	})
	if len(pages) == 0 {
		return nil, fmt.Errorf("PDF produced no renderable pages")
	}
	return pages, nil
}

func pageNumber(name string) int {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	index := strings.LastIndexByte(base, '-')
	if index < 0 {
		return 0
	}
	number, _ := strconv.Atoi(base[index+1:])
	return number
}

func resolveRepositoryFile(repoDir, input string) (string, error) {
	if strings.TrimSpace(repoDir) == "" {
		return "", fmt.Errorf("repository workspace is unavailable")
	}
	repoAbs, err := filepath.Abs(repoDir)
	if err != nil {
		return "", fmt.Errorf("resolve repository: %w", err)
	}
	path := input
	if !filepath.IsAbs(path) {
		path = filepath.Join(repoAbs, filepath.FromSlash(path))
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve attachment path: %w", err)
	}
	relative, err := filepath.Rel(repoAbs, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("attachment must be inside the project repository")
	}
	return path, nil
}

func resolveOutputFile(repoDir, input string) (string, error) {
	path, err := resolveRepositoryFile(repoDir, input)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(repoDir, path)
	if err != nil || relative == "." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("vision output must be inside the project repository")
	}
	return path, nil
}

func readAttachment(path string) ([]byte, string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, "", fmt.Errorf("stat attachment: %w", err)
	}
	if info.IsDir() || info.Size() <= 0 {
		return nil, "", fmt.Errorf("attachment must be a non-empty file")
	}
	if info.Size() > maxFileBytes {
		return nil, "", fmt.Errorf("attachment exceeds %d bytes", maxFileBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read attachment: %w", err)
	}
	mimeType := http.DetectContentType(data)
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".pdf" {
		mimeType = "application/pdf"
	}
	if !strings.HasPrefix(mimeType, "image/") && mimeType != "application/pdf" {
		return nil, "", fmt.Errorf("unsupported vision attachment type: %s", mimeType)
	}
	return data, mimeType, nil
}

func modelEndpoint(baseURL, protocol string) string {
	endpoint := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	for _, suffix := range []string{"/chat/completions", "/responses"} {
		if strings.HasSuffix(endpoint, suffix) {
			return endpoint
		}
	}
	if protocol == "responses" {
		return endpoint + "/responses"
	}
	return endpoint + "/chat/completions"
}

func responseParts(parts []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(parts))
	for _, part := range parts {
		switch part["type"] {
		case "text":
			result = append(result, map[string]any{"type": "input_text", "text": part["text"]})
		case "image_url":
			image := part["image_url"].(map[string]any)
			result = append(result, map[string]any{"type": "input_image", "image_url": image["url"]})
		case "file":
			file := part["file"].(map[string]any)
			result = append(result, map[string]any{
				"type":      "input_file",
				"filename":  file["filename"],
				"file_data": file["file_data"],
			})
		}
	}
	return result
}

func extractResponseText(data []byte, protocol string) (string, error) {
	var response map[string]any
	if err := json.Unmarshal(data, &response); err != nil {
		return "", fmt.Errorf("decode vision response: %w", err)
	}
	if protocol == "responses" {
		if status, _ := response["status"].(string); status == "incomplete" {
			return "", fmt.Errorf("vision response was truncated by the model output limit")
		}
		if text, ok := response["output_text"].(string); ok && strings.TrimSpace(text) != "" {
			return text, nil
		}
		if output, ok := response["output"].([]any); ok {
			var texts []string
			for _, item := range output {
				message, _ := item.(map[string]any)
				content, _ := message["content"].([]any)
				for _, part := range content {
					partMap, _ := part.(map[string]any)
					if text, ok := partMap["text"].(string); ok && strings.TrimSpace(text) != "" {
						texts = append(texts, text)
					}
				}
			}
			if len(texts) > 0 {
				return strings.Join(texts, "\n"), nil
			}
		}
	}
	choices, ok := response["choices"].([]any)
	if !ok || len(choices) == 0 {
		return "", fmt.Errorf("vision response contains no message text")
	}
	choice, _ := choices[0].(map[string]any)
	if finishReason, _ := choice["finish_reason"].(string); finishReason == "length" {
		return "", fmt.Errorf("vision response was truncated by the model output limit")
	}
	message, _ := choice["message"].(map[string]any)
	if text, ok := message["content"].(string); ok && strings.TrimSpace(text) != "" {
		return text, nil
	}
	if content, ok := message["content"].([]any); ok {
		var texts []string
		for _, part := range content {
			partMap, _ := part.(map[string]any)
			if text, ok := partMap["text"].(string); ok && strings.TrimSpace(text) != "" {
				texts = append(texts, text)
			}
		}
		if len(texts) > 0 {
			return strings.Join(texts, "\n"), nil
		}
	}
	return "", fmt.Errorf("vision response contains no message text")
}
