import type { Attachment } from "@leros/store/types/chat";
import type { FilePreviewItem } from "./file-preview-utils";
import type { OfficeTextSelection } from "./office-selection";

export type DocxSelectionInstruction = "expand" | "shorten";

export type DocxSelectionEditRequest = {
	content: string;
	displayContent: string;
	attachment?: Attachment;
};

const DOCX_MIME_TYPE = "application/vnd.openxmlformats-officedocument.wordprocessingml.document";
export const DOCX_SELECTION_TEXT_LIMIT = 20_000;
const REFERENCE_EXECUTION_RULES =
	"请先读取 reference：优先使用 file.attachment_path 对应的所选版本作为源文件；存在 file.project_path 时将结果写回该项目路径，否则直接编辑附件文件。使用 selection.text 和前后文共同定位；如果不能唯一定位，停止修改并说明原因，不要猜测。";

const instructionCopy: Record<DocxSelectionInstruction, { label: string; prompt: string }> = {
	expand: {
		label: "扩写",
		prompt:
			"请扩写选中的内容，在保持原意、事实准确性和上下文语气的前提下补充必要细节，并将修改写回原 DOCX 文件。不要修改选区之外的内容。",
	},
	shorten: {
		label: "缩写",
		prompt:
			"请缩写选中的内容，在保留核心信息、事实准确性和上下文语气的前提下删除冗余表达，并将修改写回原 DOCX 文件。不要修改选区之外的内容。",
	},
};

export function buildDocxSelectionEditRequest({
	instruction,
	file,
	selection,
}: {
	instruction: DocxSelectionInstruction;
	file: FilePreviewItem;
	selection: OfficeTextSelection;
}): DocxSelectionEditRequest {
	const copy = instructionCopy[instruction];
	const filePublicId = file.versionPublicId?.trim() || file.publicId?.trim();
	const safeName = baseName(file.name);
	const selectedText = selection.text;
	const reference = {
		version: 1,
		kind: "docx_selection",
		instruction,
		file: {
			file_public_id: filePublicId || undefined,
			project_id: file.projectId || undefined,
			project_path: file.projectPath || undefined,
			attachment_path: filePublicId ? `uploads/${safeName}` : undefined,
			name: safeName,
			mime_type: file.mimeType || DOCX_MIME_TYPE,
			version_no: file.versionNo,
			is_historical_version: Boolean(
				file.versionPublicId && file.publicId && file.versionPublicId !== file.publicId,
			),
		},
		selection: {
			text: selectedText,
			context_before: selection.contextBefore,
			context_after: selection.contextAfter,
			page_index: selection.surfaceIndex,
			offset_encoding: "utf16",
		},
	};
	const serializedReference = JSON.stringify(reference, null, 2)
		.replaceAll("&", "\\u0026")
		.replaceAll("<", "\\u003c")
		.replaceAll(">", "\\u003e");
	const previewText = selection.text.trim().replace(/\s+/g, " ").slice(0, 48);

	return {
		content: `/docx\n<reference>\n${serializedReference}\n</reference>\n\n${REFERENCE_EXECUTION_RULES}\n${copy.prompt}`,
		displayContent: `${copy.label}文档选区：「${previewText}${selection.text.trim().length > 48 ? "…" : ""}」`,
		attachment: filePublicId
			? {
					id: `docx-selection-${filePublicId}`,
					type: "file",
					name: safeName,
					size: 0,
					fileUploadId: filePublicId,
					mimeType: file.mimeType || DOCX_MIME_TYPE,
					storageUri: file.storageUri,
				}
			: undefined,
	};
}

function baseName(value: string): string {
	return value.split(/[\\/]/).filter(Boolean).pop() || "document.docx";
}
