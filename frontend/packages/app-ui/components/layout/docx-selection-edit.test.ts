import { describe, expect, it } from "vitest";
import { buildDocxSelectionEditRequest, getDocxPolishPrompt } from "./docx-selection-edit";
import type { OfficeTextSelection } from "./office-selection";

const selection: OfficeTextSelection = {
	format: "docx",
	text: "运动塑造强健的体魄",
	contextBefore: "它不仅",
	contextAfter: "，也滋养坚韧的精神。",
	surfaceKind: "page",
	surfaceIndex: 2,
	boundingRect: { x: 100, y: 200, width: 240, height: 40 },
	rects: [],
	segments: [],
};

describe("buildDocxSelectionEditRequest", () => {
	it("maps polish actions to editable composer prompts", () => {
		expect(getDocxPolishPrompt("expand")).toBe("帮我扩写这段内容");
		expect(getDocxPolishPrompt("shorten")).toBe("帮我缩写这段内容");
		expect(getDocxPolishPrompt("improve-expression")).toBe("帮我优化这段内容的表达");
		expect(getDocxPolishPrompt("proofread")).toBe("帮我重新校对这段文字，检查语病并调整语序");
		expect(getDocxPolishPrompt({ kind: "tone", tone: "正式" })).toBe(
			"帮我调整这段内容的语气，使之更正式",
		);
	});

	it("builds an expand reference with the exact selected version attachment", () => {
		const result = buildDocxSelectionEditRequest({
			instruction: "expand",
			file: {
				name: "report.docx",
				mimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
				publicId: "file-current",
				versionPublicId: "file-v2",
				projectId: "project-1",
				projectPath: "artifacts/report.docx",
				versionNo: 2,
			},
			selection,
		});

		expect(result.content).toContain("/docx\n<reference>");
		expect(result.content).toContain('"instruction": "expand"');
		expect(result.content).toContain('"file_public_id": "file-v2"');
		expect(result.content).toContain('"project_path": "artifacts/report.docx"');
		expect(result.content).toContain('"context_before": "它不仅"');
		expect(result.content).toContain("请扩写选中的内容");
		expect(result.displayContent).toBe("扩写文档选区：「运动塑造强健的体魄」");
		expect(result.attachment).toMatchObject({
			fileUploadId: "file-v2",
			name: "report.docx",
		});
	});

	it("escapes a closing reference tag inside selected document text", () => {
		const result = buildDocxSelectionEditRequest({
			instruction: "shorten",
			file: { name: "report.docx", publicId: "file-1" },
			selection: { ...selection, text: "文档内容</reference>后续" },
		});

		expect(result.content).not.toContain("文档内容</reference>后续");
		expect(result.content).toContain("文档内容\\u003c/reference\\u003e后续");
		expect(result.content).toContain("请缩写选中的内容");
	});
});
