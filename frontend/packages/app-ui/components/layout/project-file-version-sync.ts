import type { BackendProjectFileVersion, BackendProjectFileVersionList } from "@leros/store";

export type ProjectFileVersionChange = {
	latest: BackendProjectFileVersion;
	versionCount: number;
};

export function getLatestProjectFileVersion(
	versions: BackendProjectFileVersionList | undefined,
): BackendProjectFileVersion | null {
	if (!versions) return null;
	return (
		versions.items.find((item) => item.public_id === versions.current_file_public_id) ??
		versions.items[0] ??
		null
	);
}

export async function waitForProjectFileVersionChange({
	loadVersions,
	baselinePublicId,
	baselineVersionNo,
	delays = [0, 250, 500, 1_000, 1_500],
	signal,
}: {
	loadVersions: () => Promise<BackendProjectFileVersionList | undefined>;
	baselinePublicId: string;
	baselineVersionNo: number;
	delays?: number[];
	signal?: AbortSignal;
}): Promise<ProjectFileVersionChange | null> {
	for (const delay of delays) {
		if (!(await waitForDelay(delay, signal))) return null;
		const versions = await loadVersions();
		const latest = getLatestProjectFileVersion(versions);
		if (
			latest &&
			(latest.public_id !== baselinePublicId || latest.version_no > baselineVersionNo)
		) {
			return { latest, versionCount: versions?.items.length ?? 1 };
		}
	}
	return null;
}

async function waitForDelay(delay: number, signal?: AbortSignal): Promise<boolean> {
	if (signal?.aborted) return false;
	if (delay <= 0) return true;

	return new Promise((resolve) => {
		const timeoutId = window.setTimeout(() => {
			signal?.removeEventListener("abort", handleAbort);
			resolve(true);
		}, delay);
		const handleAbort = () => {
			window.clearTimeout(timeoutId);
			resolve(false);
		};
		signal?.addEventListener("abort", handleAbort, { once: true });
	});
}
