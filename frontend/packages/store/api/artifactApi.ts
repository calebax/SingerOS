import { apiClient } from "./client";
import { fetchFileDownload } from "./fileApi";
import type { BackendArtifact, BackendArtifactDetail, BackendDataResponse } from "./types";

const publishFileIdCache = new Map<string, string>();

function readPublishFileId(detail: BackendArtifactDetail): string {
	const publishFileId = detail.publish_file_id?.trim() ?? detail["publish-file_id"]?.trim() ?? "";
	return publishFileId;
}

async function resolveArtifactPublishFileId(
	artifactId: string,
	options?: { signal?: AbortSignal },
): Promise<string> {
	const normalizedArtifactId = artifactId.trim();
	if (!normalizedArtifactId) {
		throw new Error("artifact_id is required");
	}

	const cached = publishFileIdCache.get(normalizedArtifactId);
	if (cached) return cached;

	const response = await apiClient.post<BackendDataResponse<BackendArtifactDetail>>(
		"/GetArtifact",
		{ artifact_id: normalizedArtifactId },
		{ signal: options?.signal },
	);
	const publishFileId = readPublishFileId(response.data.data ?? {});
	if (!publishFileId) {
		throw new Error("GetArtifact 未返回 publish_file_id");
	}

	publishFileIdCache.set(normalizedArtifactId, publishFileId);
	return publishFileId;
}

export async function fetchArtifactDownload(
	artifactId: string,
	options?: { signal?: AbortSignal },
): Promise<Response> {
	const publishFileId = await resolveArtifactPublishFileId(artifactId, options);
	return fetchFileDownload(publishFileId, options);
}

export const artifactApi = {
	fetchDownload: fetchArtifactDownload,
	listTaskArtifacts: (taskId: string) =>
		apiClient.get<BackendDataResponse<BackendArtifact[]>>(
			`/tasks/${encodeURIComponent(taskId)}/artifacts`,
		),
};
