import "@testing-library/jest-dom/vitest";
import { render, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const storeMocks = vi.hoisted(() => ({
	authenticatedFetch: vi.fn(),
	fetchFilePreviewByPublicId: vi.fn(),
}));

vi.mock("@leros/store", () => ({
	authenticatedFetch: storeMocks.authenticatedFetch,
	fetchFilePreviewByPublicId: storeMocks.fetchFilePreviewByPublicId,
}));

import { ProtectedImage, resetProtectedImageCacheForTests } from "./ProtectedImage";

function createImageResponse() {
	return {
		ok: true,
		blob: async () => new Blob(["avatar"], { type: "image/png" }),
	};
}

describe("ProtectedImage", () => {
	beforeEach(() => {
		resetProtectedImageCacheForTests();
		storeMocks.authenticatedFetch.mockReset();
		storeMocks.fetchFilePreviewByPublicId.mockReset();
		storeMocks.authenticatedFetch.mockResolvedValue(createImageResponse());
		storeMocks.fetchFilePreviewByPublicId.mockResolvedValue(createImageResponse());
		window.localStorage.clear();
	});

	it("loads file public_id through preview API", async () => {
		render(
			<ProtectedImage
				src="file_TwxpykjQhu"
				alt="avatar"
				className="size-7"
				fallback={<span>fallback</span>}
			/>,
		);

		await waitFor(() => {
			expect(storeMocks.fetchFilePreviewByPublicId).toHaveBeenCalledWith("file_TwxpykjQhu");
		});
		expect(storeMocks.authenticatedFetch).not.toHaveBeenCalled();
	});

	it("keeps legacy protected download URL compatibility", async () => {
		const legacyURL = "http://localhost:18080/v1/files/file_TN3691n6qd/download";

		render(
			<ProtectedImage
				src={legacyURL}
				alt="avatar"
				className="size-7"
				fallback={<span>fallback</span>}
			/>,
		);

		await waitFor(() => {
			expect(storeMocks.authenticatedFetch).toHaveBeenCalledWith(legacyURL);
		});
		expect(storeMocks.fetchFilePreviewByPublicId).not.toHaveBeenCalled();
	});

	it("deduplicates concurrent loads for the same file public_id", async () => {
		render(
			<>
				<ProtectedImage
					src="file_TwxpykjQhu"
					alt="avatar-1"
					className="size-7"
					fallback={<span>fallback-1</span>}
				/>
				<ProtectedImage
					src="file_TwxpykjQhu"
					alt="avatar-2"
					className="size-7"
					fallback={<span>fallback-2</span>}
				/>
			</>,
		);

		await waitFor(() => {
			expect(storeMocks.fetchFilePreviewByPublicId).toHaveBeenCalledTimes(1);
		});
		expect(storeMocks.fetchFilePreviewByPublicId).toHaveBeenCalledWith("file_TwxpykjQhu");
	});
});
