import type { ApiError } from "@leros/ui/lib/request";
import { apiClient } from "./client";
import { fetchFileDownload } from "./fileApi";
import type { BackendArtifact, BackendArtifactDetail, BackendDataResponse } from "./types";

function isNotFoundError(error: unknown): boolean {
	return (
		typeof error === "object" &&
		error !== null &&
		"status" in error &&
		(error as ApiError).status === 404
	);
}

/** Lists task artifacts; prefers deployed RPC route, falls back to REST GET for local dev. */
async function listTaskArtifacts(taskId: string) {
	const normalizedTaskId = taskId.trim();
	if (!normalizedTaskId) {
		throw new Error("task_id is required");
	}

	try {
		return await apiClient.post<BackendDataResponse<BackendArtifact[]>>("/ListTaskArtifacts", {
			task_id: normalizedTaskId,
		});
	} catch (error) {
		if (!isNotFoundError(error)) throw error;
		return apiClient.get<BackendDataResponse<BackendArtifact[]>>(
			`/tasks/${encodeURIComponent(normalizedTaskId)}/artifacts`,
		);
	}
}

async function resolveArtifactFileID(
	artifactId: string,
	options?: { signal?: AbortSignal },
): Promise<string> {
	const normalizedArtifactId = artifactId.trim();
	if (!normalizedArtifactId) {
		throw new Error("artifact_id is required");
	}

	const response = await apiClient.post<BackendDataResponse<BackendArtifactDetail>>(
		"/GetArtifact",
		{ artifact_id: normalizedArtifactId },
		{ signal: options?.signal },
	);
	const fileID = response.data.data?.file_public_id?.trim();
	if (!fileID) {
		throw new Error("GetArtifact 未返回 file_public_id");
	}
	return fileID;
}

export async function fetchArtifactDownload(
	artifactId: string,
	options?: { signal?: AbortSignal },
): Promise<Response> {
	const fileID = await resolveArtifactFileID(artifactId, options);
	return fetchFileDownload(fileID, options);
}

export const artifactApi = {
	fetchDownload: fetchArtifactDownload,
	listTaskArtifacts,
};
