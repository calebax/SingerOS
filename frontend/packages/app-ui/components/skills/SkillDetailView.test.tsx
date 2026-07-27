import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SkillDetailView } from "./SkillDetailView";

const { mockOfficialGet, mockInstall, mockPluginGet, mockInstallationStatus, mockDelete } =
	vi.hoisted(() => ({
		mockOfficialGet: vi.fn(),
		mockInstall: vi.fn(),
		mockPluginGet: vi.fn(),
		mockInstallationStatus: vi.fn(),
		mockDelete: vi.fn(),
	}));

vi.mock("@leros/store", () => ({
	officialPluginMarketplaceApi: {
		get: mockOfficialGet,
		install: mockInstall,
	},
	pluginApi: {
		get: mockPluginGet,
		getInstallationStatus: mockInstallationStatus,
		delete: mockDelete,
	},
}));

vi.mock("../common/MarkdownRenderer", () => ({
	MarkdownRenderer: () => <div>markdown</div>,
}));

vi.mock("./SkillFileTree", () => ({
	SkillFileTree: () => <div>files</div>,
}));

vi.mock("sonner", () => ({
	toast: {
		success: vi.fn(),
		error: vi.fn(),
	},
}));

const officialItem = {
	public_id: "mkt_demo",
	code: "demo",
	kind: "skill",
	name: "Demo",
	description: "Demo skill",
	author: "LeWork",
	version: "2",
	category: "official",
	tags: [],
	verified: true,
	content: null,
};

const organizationPlugin = {
	public_id: "plugin_demo",
	code: "demo",
	kind: "skill",
	name: "Demo",
	description: "Demo skill",
	status: "active",
	origin: "marketplace",
	current_revision: 3,
};

const updateStatus = {
	kind: "skill",
	code: "demo",
	installed: true,
	plugin_id: "plugin_demo",
	current_version: "3",
	marketplace_based: true,
	marketplace_item_id: "mkt_demo",
	installed_marketplace_version: "1",
	marketplace_available: true,
	latest_marketplace_version: "2",
	update_available: true,
};

describe("SkillDetailView installation status", () => {
	afterEach(() => {
		cleanup();
	});

	beforeEach(() => {
		vi.clearAllMocks();
		mockOfficialGet.mockResolvedValue({ data: { data: officialItem } });
		mockPluginGet.mockResolvedValue({
			data: { data: { plugin: organizationPlugin, content: null } },
		});
		mockInstallationStatus.mockResolvedValue({ data: { data: updateStatus } });
		mockInstall.mockResolvedValue({
			data: { data: { operation: "updated", plugin: organizationPlugin } },
		});
	});

	it("uses update as the marketplace action when a newer revision is available", async () => {
		render(<SkillDetailView skillId="mkt_demo" source="official" />);

		expect(await screen.findByText("有更新")).toBeInTheDocument();
		const updateButton = screen.getByRole("button", { name: "更新技能" });
		fireEvent.click(updateButton);

		await waitFor(() => expect(mockInstall).toHaveBeenCalledWith("mkt_demo"));
	});

	it("shows update in the organization detail overflow menu", async () => {
		render(<SkillDetailView skillId="plugin_demo" source="organization" />);

		expect(await screen.findByText("有更新")).toBeInTheDocument();
		fireEvent.click(screen.getByRole("button", { name: "更多操作" }));
		const updateItem = await screen.findByText("更新");
		fireEvent.click(updateItem);

		await waitFor(() => expect(mockInstall).toHaveBeenCalledWith("mkt_demo"));
	});
});
