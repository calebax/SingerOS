"use client";

import type { ProjectArtifact } from "@leros/store";
import type { MessageAttachment } from "@leros/store/types/chat";
import { useSyncExternalStore } from "react";
import {
	artifactToFilePreviewItem,
	type FilePreviewItem,
	toFilePreviewItemFromAttachment,
} from "./file-preview-utils";
import type { ProjectFileNode } from "./project-files";

type FilePreviewSnapshot = {
	file: FilePreviewItem | null;
};

let snapshot: FilePreviewSnapshot = { file: null };
const listeners = new Set<() => void>();

function emit() {
	for (const listener of listeners) {
		listener();
	}
}

function setSnapshot(next: FilePreviewSnapshot) {
	snapshot = next;
	emit();
}

export const filePreviewActions = {
	open(file: FilePreviewItem) {
		setSnapshot({ file });
	},
	close() {
		setSnapshot({ file: null });
	},
};

export function useFilePreviewStore() {
	const state = useSyncExternalStore(
		(onStoreChange) => {
			listeners.add(onStoreChange);
			return () => listeners.delete(onStoreChange);
		},
		() => snapshot,
		() => snapshot,
	);

	return {
		file: state.file,
		isOpen: state.file !== null,
		openFilePreview: filePreviewActions.open,
		closeFilePreview: filePreviewActions.close,
	};
}

export function openProjectFilePreview(
	projectId: string,
	file: ProjectFileNode,
	options?: { openHistory?: boolean; taskId?: string },
) {
	filePreviewActions.open({
		name: file.name,
		title: file.name,
		mimeType: file.mimeType,
		storageUri: file.storageUri,
		publicId: file.publicId,
		projectId,
		projectPath: file.path,
		versionLabel: file.versionLabel,
		versionNo: file.versionNo,
		versionCount: file.versionCount,
		openHistory: options?.openHistory,
		...(options?.taskId ? { taskId: options.taskId } : {}),
	});
}

export function openProjectArtifactPreview(artifact: ProjectArtifact, projectId?: string) {
	filePreviewActions.open(artifactToFilePreviewItem(artifact, projectId));
}

export function openMessageAttachmentPreview(attachment: MessageAttachment) {
	filePreviewActions.open(toFilePreviewItemFromAttachment(attachment));
}

export function openPlanPreview(fileUploadId: string) {
	filePreviewActions.open({
		name: "计划.md",
		title: "计划.md",
		mimeType: "text/markdown",
		publicId: fileUploadId,
	});
}
