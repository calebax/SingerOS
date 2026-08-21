import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ProjectMemberPickerDialog } from "./ProjectMemberPickerDialog";

const mocks = vi.hoisted(() => ({
	assistants: [] as Record<string, unknown>[],
	fetchAssistants: vi.fn(),
	listHumanMembers: vi.fn(),
}));

vi.mock("@leros/store", async (importOriginal) => {
	const actual = await importOriginal<typeof import("@leros/store")>();
	return {
		...actual,
		projectMemberApi: {
			...actual.projectMemberApi,
			listHumanMembers: mocks.listHumanMembers,
		},
		useDAStore: (selector: (state: unknown) => unknown) =>
			selector({ assistants: mocks.assistants, fetchAssistants: mocks.fetchAssistants }),
	};
});

vi.mock("../auth", () => ({
	useAuth: () => ({ user: { publicId: "current-user" } }),
}));

if (typeof window !== "undefined" && !("ResizeObserver" in window)) {
	class ResizeObserverStub {
		observe() {
			// noop：jsdom 未实现 ResizeObserver
		}
		unobserve() {
			// noop：jsdom 未实现 ResizeObserver
		}
		disconnect() {
			// noop：jsdom 未实现 ResizeObserver
		}
	}
	// @ts-expect-error jsdom does not expose ResizeObserver
	window.ResizeObserver = ResizeObserverStub;
}

if (typeof Element !== "undefined" && !Element.prototype.scrollIntoView) {
	Element.prototype.scrollIntoView = () => {
		// noop：jsdom 未实现 scrollIntoView
	};
}

function assistant(name: string, publicId: string) {
	return {
		id: publicId === "assistant-old" ? 1 : 2,
		publicId,
		name,
		roleName: "",
		description: "",
		avatar: "",
		status: "active",
		visibility: "public",
		systemPrompt: "",
		expertise: [],
		deploymentPublicId: `deployment-${publicId}`,
		deploymentStatus: "ready",
		deploymentError: "",
		version: 1,
		createdAt: 1,
		updatedAt: 1,
		source: "",
	};
}

function renderDialog(open: boolean) {
	return render(
		<ProjectMemberPickerDialog
			open={open}
			onOpenChange={vi.fn()}
			selectedMembers={[]}
			onConfirm={vi.fn()}
		/>,
	);
}

describe("ProjectMemberPickerDialog AI 队友刷新", () => {
	beforeEach(() => {
		mocks.assistants = [assistant("旧队友", "assistant-old")];
		mocks.fetchAssistants.mockReset();
		mocks.listHumanMembers.mockReset();
		mocks.listHumanMembers.mockResolvedValue([]);
	});

	afterEach(() => {
		cleanup();
	});

	it("即使已有缓存，每次打开也刷新并在返回前隐藏旧候选", async () => {
		let resolveFetch!: (succeeded: boolean) => void;
		mocks.fetchAssistants.mockReturnValue(
			new Promise<boolean>((resolve) => {
				resolveFetch = resolve;
			}),
		);
		const view = renderDialog(false);

		view.rerender(
			<ProjectMemberPickerDialog
				open
				onOpenChange={vi.fn()}
				selectedMembers={[]}
				onConfirm={vi.fn()}
			/>,
		);

		await screen.findByText("加载中...");
		expect(mocks.fetchAssistants).toHaveBeenCalledTimes(1);
		expect(screen.queryByText("旧队友")).not.toBeInTheDocument();

		mocks.assistants = [assistant("新队友", "assistant-new")];
		resolveFetch(true);

		await screen.findByText("新队友");
		expect(screen.queryByText("旧队友")).not.toBeInTheDocument();
	});

	it("关闭后重新打开会再次请求，失败时不展示旧候选", async () => {
		let resolveFetch!: (succeeded: boolean) => void;
		mocks.fetchAssistants.mockReturnValueOnce(
			new Promise<boolean>((resolve) => {
				resolveFetch = resolve;
			}),
		);
		const view = renderDialog(true);

		await screen.findByText("加载中...");
		resolveFetch(false);
		await screen.findByText("AI 队友加载失败，请关闭后重试");
		expect(screen.queryByText("旧队友")).not.toBeInTheDocument();

		mocks.assistants = [assistant("新队友", "assistant-new")];
		mocks.fetchAssistants.mockResolvedValueOnce(true);
		view.rerender(
			<ProjectMemberPickerDialog
				open={false}
				onOpenChange={vi.fn()}
				selectedMembers={[]}
				onConfirm={vi.fn()}
			/>,
		);
		view.rerender(
			<ProjectMemberPickerDialog
				open
				onOpenChange={vi.fn()}
				selectedMembers={[]}
				onConfirm={vi.fn()}
			/>,
		);

		await waitFor(() => expect(mocks.fetchAssistants).toHaveBeenCalledTimes(2));
		await screen.findByText("新队友");
	});
});
