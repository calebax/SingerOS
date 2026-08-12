package service

import (
	"fmt"
	"strings"

	"github.com/insmtx/Leros/backend/internal/api/contract"
	"github.com/insmtx/Leros/backend/types"
)

// ErrInvalidNewMessage 表示 CreateInitialMessage 请求参数不合法。
var ErrInvalidNewMessage = fmt.Errorf("invalid new message")

// NormalizeMessageScene 将 scene 规范化为空（普通）或已知场景值。
func NormalizeMessageScene(scene string) (string, error) {
	normalized := strings.TrimSpace(strings.ToLower(scene))
	switch normalized {
	case "", string(types.MessageSceneNormal):
		return "", nil
	case string(types.MessageSceneBidComparison):
		return string(types.MessageSceneBidComparison), nil
	default:
		return "", fmt.Errorf("%w: unsupported scene %q", ErrInvalidNewMessage, scene)
	}
}

// ValidateNewMessageScene 校验新建任务场景、输出格式与附件角色约定。
func ValidateNewMessageScene(req *contract.NewMessageRequest) (string, error) {
	if req == nil {
		return "", fmt.Errorf("%w: request is required", ErrInvalidNewMessage)
	}
	scene, err := NormalizeMessageScene(req.Scene)
	if err != nil {
		return "", err
	}
	outputFormat := strings.TrimSpace(strings.ToLower(req.OutputFormat))
	normalizeAttachmentRoles(req.Attachments)
	if scene == string(types.MessageSceneBidComparison) {
		if err := validateBidComparisonOutputFormat(outputFormat); err != nil {
			return "", err
		}
		if err := validateBidComparisonAttachments(req.Attachments); err != nil {
			return "", err
		}
	}
	req.OutputFormat = outputFormat
	return scene, nil
}

func validateBidComparisonOutputFormat(outputFormat string) error {
	switch outputFormat {
	case string(types.OutputFormatDOCX),
		string(types.OutputFormatPDF),
		string(types.OutputFormatPPTX),
		string(types.OutputFormatMarkdown):
		return nil
	default:
		return fmt.Errorf("%w: bid_comparison requires output_format docx, pdf, pptx, or md", ErrInvalidNewMessage)
	}
}

func normalizeAttachmentRoles(attachments []types.MessageAttachment) {
	for i := range attachments {
		attachments[i].AttachmentRole = strings.TrimSpace(strings.ToLower(attachments[i].AttachmentRole))
	}
}

func validateBidComparisonAttachments(attachments []types.MessageAttachment) error {
	mainCount := 0
	compareCount := 0
	for _, attachment := range attachments {
		switch attachment.AttachmentRole {
		case string(types.AttachmentRoleMain):
			mainCount++
		case string(types.AttachmentRoleCompare):
			compareCount++
		}
	}
	if mainCount != 1 {
		return fmt.Errorf("%w: bid_comparison requires exactly 1 main attachment, got %d", ErrInvalidNewMessage, mainCount)
	}
	if compareCount < 1 || compareCount > 10 {
		return fmt.Errorf("%w: bid_comparison requires 1-10 compare attachments, got %d", ErrInvalidNewMessage, compareCount)
	}
	return nil
}
