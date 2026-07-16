import type { Attachment, MessageAttachment } from "../types/chat";

export type OutgoingMessageAttachment = {
	file_upload_id: string;
	name: string;
	mime_type: string;
	size: number;
};

export function mapComposerAttachments(
	attachments?: Attachment[],
): MessageAttachment[] | undefined {
	const mapped = attachments
		?.flatMap((attachment) => mapComposerAttachment(attachment))
		.filter((attachment): attachment is MessageAttachment => attachment !== undefined);
	return mapped?.length ? mapped : undefined;
}

function mapComposerAttachment(attachment: Attachment): MessageAttachment[] {
	if (attachment.type === "folder" && attachment.folderFiles?.length) {
		return [
			{
				id: attachment.id,
				fileUploadId: attachment.folderFiles[0]?.fileUploadId ?? attachment.id,
				name: attachment.name,
				mimeType: "application/x-directory",
				size: attachment.size,
				createdAt: Date.now(),
				attachmentType: "folder",
			},
		];
	}

	const fileUploadId = attachment.fileUploadId?.trim();
	if (!fileUploadId) return [];

	return [
		{
			id: attachment.id,
			fileUploadId,
			name: attachment.name,
			mimeType: attachment.mimeType || attachment.file?.type || "application/octet-stream",
			size: attachment.size,
			createdAt: Date.now(),
			url: attachment.url,
			storageUri: attachment.storageUri,
		},
	];
}

export function mapOutgoingAttachments(
	attachments?: Attachment[],
): OutgoingMessageAttachment[] | undefined {
	const mapped: OutgoingMessageAttachment[] = [];

	for (const attachment of attachments ?? []) {
		if (attachment.type === "folder" && attachment.folderFiles?.length) {
			for (const file of attachment.folderFiles) {
				const fileUploadId = file.fileUploadId.trim();
				if (!fileUploadId) continue;
				mapped.push({
					file_upload_id: fileUploadId,
					name: file.name,
					mime_type: file.mimeType || "application/octet-stream",
					size: file.size,
				});
			}
			continue;
		}

		const fileUploadId = attachment.fileUploadId?.trim();
		if (!fileUploadId) continue;
		mapped.push({
			file_upload_id: fileUploadId,
			name: attachment.name,
			mime_type: attachment.mimeType || attachment.file?.type || "application/octet-stream",
			size: attachment.size,
		});
	}

	return mapped.length ? mapped : undefined;
}
