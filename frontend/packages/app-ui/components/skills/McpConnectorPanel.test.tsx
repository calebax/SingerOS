import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { McpConnectorPanel } from "./McpConnectorPanel";

const { mockPluginList } = vi.hoisted(() => ({
	mockPluginList: vi.fn(),
}));

vi.mock("@leros/store", () => ({
	pluginApi: {
		list: mockPluginList,
	},
}));

describe("McpConnectorPanel", () => {
	afterEach(cleanup);

	beforeEach(() => {
		vi.clearAllMocks();
		mockPluginList.mockResolvedValue({
			data: {
				data: {
					plugins: [
						{
							public_id: "plugin_mcp",
							code: "browser",
							kind: "mcp",
							name: "浏览器连接器",
							description: "连接浏览器服务",
							status: "active",
							origin: "manual",
							current_revision: 2,
						},
						{
							public_id: "plugin_mcp_inactive",
							code: "documents",
							kind: "mcp",
							name: "文档连接器",
							description: "连接文档服务",
							status: "inactive",
							origin: "marketplace",
							current_revision: 1,
						},
					],
				},
			},
		});
	});

	it("loads and renders organization MCP plugins", async () => {
		render(<McpConnectorPanel />);

		expect(await screen.findByText("浏览器连接器")).toBeInTheDocument();
		expect(screen.getByText("文档连接器")).toBeInTheDocument();
		expect(screen.getByText("自定义 MCP 服务")).toBeInTheDocument();
		expect(screen.getByRole("button", { name: "管理连接" })).toBeInTheDocument();
		expect(screen.getByRole("button", { name: "立即连接" })).toBeInTheDocument();
		await waitFor(() =>
			expect(mockPluginList).toHaveBeenCalledWith({
				kind: "mcp",
				limit: 90,
			}),
		);
	});

	it("filters connectors by connection state and search keyword", async () => {
		render(<McpConnectorPanel />);

		expect(await screen.findByText("浏览器连接器")).toBeInTheDocument();
		fireEvent.click(screen.getByRole("button", { name: "未连接1" }));
		expect(screen.queryByText("浏览器连接器")).not.toBeInTheDocument();
		expect(screen.getByText("文档连接器")).toBeInTheDocument();

		fireEvent.change(screen.getByRole("searchbox", { name: "搜索 MCP 连接器" }), {
			target: { value: "不存在" },
		});
		expect(screen.getByText("暂无符合条件的连接器")).toBeInTheDocument();
	});

	it("does not request organization connectors before login", async () => {
		render(<McpConnectorPanel isAuthenticated={false} />);

		expect(await screen.findByText("登录后查看组织连接器")).toBeInTheDocument();
		expect(screen.getByRole("button", { name: "配置自定义 MCP" })).toBeDisabled();
		expect(mockPluginList).not.toHaveBeenCalled();
	});
});
