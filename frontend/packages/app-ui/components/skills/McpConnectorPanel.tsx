"use client";

import {
	type MCPPlatform,
	type MCPPluginConfig,
	type PluginListItem,
	pluginApi,
} from "@leros/store";
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
} from "@leros/ui/components/ui/alert-dialog";
import { Button } from "@leros/ui/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@leros/ui/components/ui/dialog";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuTrigger,
} from "@leros/ui/components/ui/dropdown-menu";
import { Ellipsis, Loader2, Plus, Search, Server, SlidersHorizontal, Trash2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { MCPConnectorIcon } from "../common/MCPConnectorIcon";

const PAGE_SIZE = 90;

type MCPForm = {
	name: string;
	url: string;
	bearerToken: string;
	headers: MCPHeaderRow[];
};

type MCPHeaderRow = {
	id: number;
	key: string;
	value: string;
};

let nextHeaderRowID = 1;

function newHeaderRow(key = "", value = ""): MCPHeaderRow {
	return { id: nextHeaderRowID++, key, value };
}

function emptyForm(): MCPForm {
	return {
		name: "",
		url: "",
		bearerToken: "",
		headers: [newHeaderRow()],
	};
}

function requestErrorMessage(error: unknown, fallback: string) {
	if (error && typeof error === "object" && "response" in error) {
		const response = error.response;
		if (response && typeof response === "object" && "data" in response) {
			const data = response.data;
			if (
				data &&
				typeof data === "object" &&
				"message" in data &&
				typeof data.message === "string"
			) {
				return data.message;
			}
		}
	}
	if (error instanceof Error && error.message) return error.message;
	return fallback;
}

function parseHeaders(rows: MCPHeaderRow[]): Record<string, string> {
	const result: Record<string, string> = {};
	const seen = new Set<string>();
	for (const row of rows) {
		const key = row.key.trim();
		const value = row.value.trim();
		if (!key && !value) continue;
		if (!key || !value) {
			throw new Error("标头的键和值需要同时填写");
		}
		const normalizedKey = key.toLocaleLowerCase();
		if (seen.has(normalizedKey)) {
			throw new Error(`标头 ${key} 重复`);
		}
		seen.add(normalizedKey);
		result[key] = row.value;
	}
	return result;
}

function headerRows(headers?: Record<string, string>): MCPHeaderRow[] {
	const rows = Object.entries(headers ?? {}).map(([key, value]) => newHeaderRow(key, value));
	return rows.length > 0 ? rows : [newHeaderRow()];
}

function formConfig(form: MCPForm): MCPPluginConfig {
	return {
		name: form.name.trim(),
		url: form.url.trim(),
		bearer_token: form.bearerToken.trim(),
		headers: parseHeaders(form.headers),
	};
}

export function McpConnectorPanel({ isAuthenticated = true }: { isAuthenticated?: boolean }) {
	const [connectors, setConnectors] = useState<PluginListItem[]>([]);
	const [platforms, setPlatforms] = useState<MCPPlatform[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [keyword, setKeyword] = useState("");
	const [dialogOpen, setDialogOpen] = useState(false);
	const [editingPluginID, setEditingPluginID] = useState<string | null>(null);
	const [form, setForm] = useState<MCPForm>(emptyForm);
	const [saving, setSaving] = useState(false);
	const [connectingPlatform, setConnectingPlatform] = useState<string | null>(null);
	const [disconnectingPlatform, setDisconnectingPlatform] = useState<MCPPlatform | null>(null);
	const [disconnecting, setDisconnecting] = useState(false);
	const [deletingConnector, setDeletingConnector] = useState<PluginListItem | null>(null);
	const [deleting, setDeleting] = useState(false);

	const fetchConnectors = useCallback(async () => {
		if (!isAuthenticated) {
			setConnectors([]);
			setPlatforms([]);
			setLoading(false);
			setError(null);
			return;
		}
		setLoading(true);
		setError(null);
		try {
			const connectorResponse = await pluginApi.list({
				kind: "mcp",
				status: "active",
				limit: PAGE_SIZE,
			});
			setConnectors(connectorResponse.data.data.plugins ?? []);
			const platformResponse = await pluginApi.listMCPPlatforms();
			setPlatforms(platformResponse.data.data.platforms ?? []);
		} catch (requestError) {
			setError(requestErrorMessage(requestError, "加载失败"));
		} finally {
			setLoading(false);
		}
	}, [isAuthenticated]);

	useEffect(() => {
		void fetchConnectors();
	}, [fetchConnectors]);

	const visibleConnectors = useMemo(() => {
		const platformPluginIDs = new Set(
			platforms
				.map((platform) => platform.plugin_id)
				.filter((pluginID): pluginID is string => Boolean(pluginID)),
		);
		const customConnectors = connectors.filter(
			(connector) => !platformPluginIDs.has(connector.public_id),
		);
		const query = keyword.trim().toLocaleLowerCase();
		if (!query) return customConnectors;
		return customConnectors.filter((connector) =>
			[connector.name, connector.code, connector.description]
				.filter(Boolean)
				.join(" ")
				.toLocaleLowerCase()
				.includes(query),
		);
	}, [connectors, keyword, platforms]);

	const visiblePlatforms = useMemo(() => {
		const query = keyword.trim().toLocaleLowerCase();
		if (!query) return platforms;
		return platforms.filter((platform) =>
			[platform.name, platform.code, platform.description]
				.join(" ")
				.toLocaleLowerCase()
				.includes(query),
		);
	}, [keyword, platforms]);

	const openCreateDialog = () => {
		setEditingPluginID(null);
		setForm(emptyForm());
		setDialogOpen(true);
	};

	const openManageDialog = async (connector: PluginListItem) => {
		try {
			const response = await pluginApi.get(connector.public_id);
			const definition = response.data.data.definition;
			if (definition?.transport !== "http") {
				throw new Error("当前 MCP 配置不可编辑");
			}
			setEditingPluginID(connector.public_id);
			setForm({
				name: connector.name,
				url: definition.url,
				bearerToken: definition.bearer_token ?? "",
				headers: headerRows(definition.headers),
			});
			setDialogOpen(true);
		} catch (requestError) {
			toast.error(requestErrorMessage(requestError, "加载 MCP 配置失败"));
		}
	};

	const testSavedConnector = async (connector: PluginListItem) => {
		try {
			const response = await pluginApi.get(connector.public_id);
			const definition = response.data.data.definition;
			if (!definition) throw new Error("MCP 配置不存在");
			const result = await pluginApi.testMCP({
				url: definition.url,
				bearer_token: definition.bearer_token,
				headers: definition.headers,
			});
			toast.success(`连接成功，发现 ${result.data.data.tool_count} 个工具`);
		} catch (requestError) {
			toast.error(requestErrorMessage(requestError, "连接测试失败"));
		}
	};

	const testPlatform = async (platform: MCPPlatform) => {
		if (!platform.plugin_id) return;
		const connector = connectors.find((item) => item.public_id === platform.plugin_id);
		await testSavedConnector(
			connector ?? {
				public_id: platform.plugin_id,
				code: platform.code,
				kind: "mcp",
				name: platform.name,
				status: "active",
				origin: "org",
				current_revision: 1,
			},
		);
	};

	const connectPlatform = async (platform: MCPPlatform) => {
		if (!isAuthenticated || platform.connected || !platform.auto_connect_supported) return;
		setConnectingPlatform(platform.code);
		try {
			const response = await pluginApi.connectMCPPlatform(platform.code);
			toast.success(`${platform.name} 连接成功，发现 ${response.data.data.tool_count} 个工具`);
			await fetchConnectors();
		} catch (requestError) {
			toast.error(requestErrorMessage(requestError, `${platform.name} 连接失败`));
		} finally {
			setConnectingPlatform(null);
		}
	};

	const disconnectPlatform = async () => {
		if (!disconnectingPlatform?.plugin_id) return;
		const platformName = disconnectingPlatform.name;
		setDisconnecting(true);
		try {
			await pluginApi.delete(disconnectingPlatform.plugin_id);
			toast.success(`${platformName} 已断开`);
			setDisconnectingPlatform(null);
			await fetchConnectors();
		} catch (requestError) {
			toast.error(requestErrorMessage(requestError, `断开 ${platformName} 失败`));
		} finally {
			setDisconnecting(false);
		}
	};

	const deleteConnector = async () => {
		if (!deletingConnector) return;
		setDeleting(true);
		try {
			await pluginApi.delete(deletingConnector.public_id);
			toast.success("MCP 连接已删除");
			setDeletingConnector(null);
			await fetchConnectors();
		} catch (requestError) {
			toast.error(requestErrorMessage(requestError, "删除 MCP 连接失败"));
		} finally {
			setDeleting(false);
		}
	};

	const saveConfig = async () => {
		setSaving(true);
		try {
			const config = formConfig(form);
			if (editingPluginID) {
				await pluginApi.updateMCP(editingPluginID, config);
				toast.success("MCP 配置已更新");
			} else {
				await pluginApi.addMCP(config);
				toast.success("MCP 配置已创建");
			}
			setDialogOpen(false);
			await fetchConnectors();
		} catch (requestError) {
			toast.error(requestErrorMessage(requestError, "保存 MCP 配置失败"));
		} finally {
			setSaving(false);
		}
	};

	const updateHeader = (id: number, field: "key" | "value", value: string) => {
		setForm((current) => ({
			...current,
			headers: current.headers.map((header) =>
				header.id === id ? { ...header, [field]: value } : header,
			),
		}));
	};

	const removeHeader = (id: number) => {
		setForm((current) => {
			const headers = current.headers.filter((header) => header.id !== id);
			return { ...current, headers: headers.length > 0 ? headers : [newHeaderRow()] };
		});
	};

	return (
		<div className="flex min-h-0 flex-1 flex-col">
			<section className="shrink-0 border-b border-[var(--leros-control-border)] bg-white px-5 pb-4 pt-5">
				<div className="mx-auto w-full max-w-[1480px]">
					<div className="flex items-start justify-between gap-4">
						<div>
							<h2 className="text-xl font-semibold tracking-tight text-[var(--leros-text-strong)]">
								MCP 连接器
							</h2>
							<p className="mt-1 text-xs text-[var(--leros-text-muted)]">
								管理你添加的外部系统连接，通过项目关联决定 Worker 运行时可用的 MCP。
							</p>
						</div>
						<Button
							type="button"
							variant="outline"
							disabled={!isAuthenticated}
							onClick={openCreateDialog}
							className="h-8 shrink-0 rounded-md px-3 text-xs"
						>
							配置自定义 MCP
						</Button>
					</div>
					<div className="mt-4 flex items-center gap-3">
						<div className="relative w-[420px] max-w-full">
							<Search className="absolute left-3 top-1/2 size-3.5 -translate-y-1/2 text-[var(--leros-text-subtle)]" />
							<input
								type="search"
								aria-label="搜索 MCP 连接器"
								placeholder="搜索连接器"
								value={keyword}
								onChange={(event) => setKeyword(event.target.value)}
								className="h-9 w-full rounded-md border border-[var(--leros-control-border)] bg-[var(--leros-surface-soft)] pl-9 pr-3 text-xs outline-none focus:border-[var(--leros-primary)]"
							/>
						</div>
					</div>
				</div>
			</section>

			<section className="min-h-0 flex-1 overflow-y-auto bg-[var(--leros-surface-soft)] px-5 py-4">
				<div className="mx-auto w-full max-w-[1480px]">
					{visiblePlatforms.length > 0 ? (
						<div className="mb-5">
							<div className="mb-3 flex items-center justify-between">
								<h3 className="text-sm font-semibold">平台连接器</h3>
								<span className="text-xs text-[var(--leros-text-subtle)]">选择平台快速连接</span>
							</div>
							<div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
								{visiblePlatforms.map((platform) => {
									const connecting = connectingPlatform === platform.code;
									const isManagedCoreKG = platform.code === "corekg";
									const connectionDisabled =
										!isAuthenticated ||
										connecting ||
										platform.connected ||
										!platform.auto_connect_supported;
									return (
										<article
											key={platform.code}
											className="flex min-h-[76px] items-center gap-3 rounded-lg border border-[var(--leros-control-border)] bg-white px-4 py-3 transition-colors hover:border-[var(--leros-primary-soft)]"
										>
											<MCPConnectorIcon
												code={platform.code}
												name={platform.name}
												className="size-10"
											/>
											<div className="min-w-0 flex-1">
												<div className="flex items-center gap-2">
													<h4 className="truncate text-[13px] font-semibold">{platform.name}</h4>
													{platform.connected ? (
														<span className="rounded-full bg-emerald-50 px-2 py-0.5 text-[10px] font-medium text-emerald-600">
															已连接
														</span>
													) : connecting ? (
														<span className="inline-flex items-center gap-1 rounded-full bg-blue-50 px-2 py-0.5 text-[10px] font-medium text-blue-600">
															<Loader2 className="size-3 animate-spin" />
															连接中
														</span>
													) : null}
												</div>
												<p className="mt-0.5 truncate text-xs text-[var(--leros-text-muted)]">
													{platform.description}
												</p>
												{!platform.auto_connect_supported && !platform.connected ? (
													<p className="mt-1 text-[10px] text-amber-600">
														当前版本暂不支持自动授权
													</p>
												) : null}
											</div>
											{!isManagedCoreKG && platform.connected ? (
												<DropdownMenu>
													<DropdownMenuTrigger
														render={(props) => (
															<Button
																{...props}
																type="button"
																variant="ghost"
																aria-label={`管理 ${platform.name}`}
																className="size-8 shrink-0 rounded-md p-0"
															>
																<Ellipsis className="size-4" />
															</Button>
														)}
													/>
													<DropdownMenuContent align="end" className="w-32">
														<DropdownMenuItem onClick={() => void testPlatform(platform)}>
															测试连接
														</DropdownMenuItem>
														<DropdownMenuItem
															className="text-red-600 focus:text-red-600"
															onClick={() => setDisconnectingPlatform(platform)}
														>
															断开连接
														</DropdownMenuItem>
													</DropdownMenuContent>
												</DropdownMenu>
											) : !isManagedCoreKG ? (
												<Button
													type="button"
													variant="ghost"
													aria-label={`连接 ${platform.name}`}
													title={
														platform.auto_connect_supported
															? `连接 ${platform.name}`
															: "当前版本暂不支持自动授权"
													}
													disabled={connectionDisabled}
													onClick={() => void connectPlatform(platform)}
													className="size-8 shrink-0 rounded-md p-0"
												>
													{connecting ? (
														<Loader2 className="size-4 animate-spin" />
													) : (
														<Plus className="size-4" />
													)}
												</Button>
											) : null}
										</article>
									);
								})}
							</div>
						</div>
					) : null}

					<div className="mb-3 flex items-center justify-between border-t border-[var(--leros-control-border)] pt-4">
						<h3 className="text-sm font-semibold">自定义连接器</h3>
						<span className="text-xs text-[var(--leros-text-subtle)]">
							共 {visibleConnectors.length} 个
						</span>
					</div>

					{loading ? (
						<div className="py-20 text-center text-sm text-[var(--leros-text-subtle)]">
							加载中...
						</div>
					) : error ? (
						<div className="py-20 text-center text-sm text-red-500">{error}</div>
					) : visibleConnectors.length === 0 ? (
						<div className="flex flex-col items-center justify-center rounded-lg border border-dashed bg-white py-16 text-center">
							<Server className="size-5 text-[var(--leros-primary)]" />
							<p className="mt-3 text-sm font-medium">
								{!isAuthenticated
									? "登录后查看你的连接器"
									: keyword
										? "暂无符合条件的连接器"
										: "暂无 MCP 连接器"}
							</p>
						</div>
					) : (
						<div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
							{visibleConnectors.map((connector) => {
								const displayName = connector.name || connector.code;
								return (
									<article
										key={connector.public_id}
										className="flex min-h-[76px] items-center gap-3 rounded-lg border border-[var(--leros-control-border)] bg-white px-4 py-3 transition-colors hover:border-[var(--leros-primary-soft)]"
									>
										<div className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-[var(--leros-surface-soft)] text-sm font-semibold">
											{displayName.charAt(0).toUpperCase()}
										</div>
										<div className="min-w-0 flex-1">
											<h4 className="truncate text-[13px] font-semibold">{displayName}</h4>
											<p className="mt-0.5 truncate text-xs text-[var(--leros-text-muted)]">工具</p>
										</div>
										<DropdownMenu>
											<DropdownMenuTrigger
												render={(props) => (
													<Button
														{...props}
														type="button"
														variant="ghost"
														aria-label={`管理 ${displayName}`}
														className="size-8 shrink-0 rounded-md p-0"
													>
														<Ellipsis className="size-4" />
													</Button>
												)}
											/>
											<DropdownMenuContent align="end" className="w-32">
												<DropdownMenuItem onClick={() => void testSavedConnector(connector)}>
													测试连接
												</DropdownMenuItem>
												<DropdownMenuItem onClick={() => void openManageDialog(connector)}>
													管理连接
												</DropdownMenuItem>
												<DropdownMenuItem
													className="text-red-600 focus:text-red-600"
													onClick={() => setDeletingConnector(connector)}
												>
													删除连接
												</DropdownMenuItem>
											</DropdownMenuContent>
										</DropdownMenu>
									</article>
								);
							})}
						</div>
					)}
				</div>
			</section>

			<Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
				<DialogContent className="flex max-h-[min(88dvh,680px)] max-w-[min(94vw,600px)] flex-col gap-0 overflow-hidden border-[var(--leros-control-border)] p-0 shadow-[0_24px_70px_rgba(15,23,42,0.16)] sm:rounded-2xl">
					<DialogHeader className="shrink-0 flex-row items-center gap-3 border-b border-[var(--leros-control-border)] px-5 py-3.5 text-left">
						<div className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-[var(--leros-primary-soft)] text-[var(--leros-primary)]">
							<SlidersHorizontal className="size-3.5" />
						</div>
						<div className="min-w-0">
							<DialogTitle className="text-[15px] leading-5">
								{editingPluginID ? "管理 MCP" : "自定义 MCP"}
							</DialogTitle>
							<DialogDescription className="mt-0.5 text-[11px] leading-4">
								配置组织可用的远程服务，保存后可关联到项目
							</DialogDescription>
						</div>
					</DialogHeader>
					<div className="min-h-0 flex-1 overflow-y-auto bg-white px-5 py-5">
						<MCPField
							label="名称"
							value={form.name}
							placeholder="MCP server name"
							onChange={(name) => setForm((current) => ({ ...current, name }))}
						/>

						<div className="mt-5 flex items-center justify-between">
							<p className="text-[13px] font-semibold text-[var(--leros-text-strong)]">类型</p>
							<div className="flex w-[220px] items-center rounded-lg bg-[var(--leros-surface-soft)] p-1">
								<button
									type="button"
									disabled
									title="暂未开放"
									className="h-8 flex-1 rounded-md text-xs text-[var(--leros-text-subtle)] disabled:cursor-not-allowed"
								>
									STDIO
								</button>
								<button
									type="button"
									aria-pressed="true"
									className="h-8 flex-1 rounded-md bg-[var(--leros-primary-soft)] text-xs font-medium text-[var(--leros-primary)] shadow-[0_1px_2px_rgba(79,70,229,0.08)]"
								>
									流式 HTTP
								</button>
							</div>
						</div>

						<div className="my-5 border-t border-[var(--leros-control-border)]" />

						<MCPField
							label="URL"
							value={form.url}
							placeholder="https://mcp.example.com/mcp"
							onChange={(url) => setForm((current) => ({ ...current, url }))}
						/>

						<div className="mt-5">
							<label
								htmlFor="mcp-bearer-token"
								className="text-[13px] font-semibold text-[var(--leros-text-strong)]"
							>
								Bearer 令牌
							</label>
							<input
								id="mcp-bearer-token"
								aria-label="Bearer 令牌"
								type="password"
								value={form.bearerToken}
								onChange={(event) =>
									setForm((current) => ({
										...current,
										bearerToken: event.target.value,
									}))
								}
								placeholder="请输入 Bearer Token"
								className="mt-2 h-10 w-full rounded-lg border border-[var(--leros-control-border)] bg-[var(--leros-surface-soft)] px-3 text-xs outline-none transition-colors placeholder:text-[var(--leros-text-subtle)] focus:border-[var(--leros-primary)] focus:bg-white"
							/>
							<p className="mt-1.5 text-[11px] text-[var(--leros-text-subtle)]">
								请求时将自动添加 Authorization: Bearer 标头。
							</p>
						</div>

						<div className="my-5 border-t border-[var(--leros-control-border)]" />

						<div>
							<h4 className="text-[13px] font-semibold text-[var(--leros-text-strong)]">标头</h4>
							<div className="mt-2 space-y-2">
								{form.headers.map((header, index) => (
									<div key={header.id} className="flex items-center gap-2">
										<input
											aria-label={`标头键 ${index + 1}`}
											value={header.key}
											placeholder="键"
											onChange={(event) => updateHeader(header.id, "key", event.target.value)}
											className="h-10 min-w-0 flex-1 rounded-lg border border-[var(--leros-control-border)] bg-[var(--leros-surface-soft)] px-3 text-xs outline-none transition-colors placeholder:text-[var(--leros-text-subtle)] focus:border-[var(--leros-primary)] focus:bg-white"
										/>
										<input
											aria-label={`标头值 ${index + 1}`}
											value={header.value}
											placeholder="值"
											onChange={(event) => updateHeader(header.id, "value", event.target.value)}
											className="h-10 min-w-0 flex-1 rounded-lg border border-[var(--leros-control-border)] bg-[var(--leros-surface-soft)] px-3 text-xs outline-none transition-colors placeholder:text-[var(--leros-text-subtle)] focus:border-[var(--leros-primary)] focus:bg-white"
										/>
										<button
											type="button"
											aria-label={`删除标头 ${index + 1}`}
											onClick={() => removeHeader(header.id)}
											className="flex size-8 shrink-0 items-center justify-center rounded-md text-[var(--leros-text-subtle)] transition-colors hover:bg-[var(--leros-surface-soft)] hover:text-red-500"
										>
											<Trash2 className="size-4" />
										</button>
									</div>
								))}
							</div>
							<button
								type="button"
								onClick={() =>
									setForm((current) => ({
										...current,
										headers: [...current.headers, newHeaderRow()],
									}))
								}
								className="mt-3 flex h-10 w-full items-center justify-center gap-1.5 rounded-lg border border-dashed border-[var(--leros-control-border)] bg-white text-xs font-medium text-[var(--leros-text-muted)] transition-colors hover:border-[var(--leros-primary)] hover:bg-[var(--leros-primary-soft)]/30 hover:text-[var(--leros-primary)]"
							>
								<Plus className="size-4" />
								添加标头
							</button>
						</div>
					</div>
					<DialogFooter className="shrink-0 border-t border-[var(--leros-control-border)] bg-white px-5 py-4">
						<Button
							type="button"
							disabled={saving}
							onClick={() => void saveConfig()}
							className="h-9 rounded-lg px-6 text-xs shadow-[0_6px_18px_rgba(15,23,42,0.16)]"
						>
							{saving ? "保存中..." : "保存"}
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			<AlertDialog
				open={disconnectingPlatform !== null}
				onOpenChange={(open) => {
					if (!open && !disconnecting) setDisconnectingPlatform(null);
				}}
			>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>断开 {disconnectingPlatform?.name}？</AlertDialogTitle>
						<AlertDialogDescription>
							断开后，该 MCP 连接会从所有关联项目中移除，且无法恢复。
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel disabled={disconnecting}>取消</AlertDialogCancel>
						<AlertDialogAction
							variant="destructive"
							disabled={disconnecting}
							onClick={() => void disconnectPlatform()}
						>
							{disconnecting ? "断开中..." : "确认断开"}
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>

			<AlertDialog
				open={deletingConnector !== null}
				onOpenChange={(open) => {
					if (!open && !deleting) setDeletingConnector(null);
				}}
			>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>
							删除 {deletingConnector?.name || deletingConnector?.code}？
						</AlertDialogTitle>
						<AlertDialogDescription>
							删除后，该 MCP 连接会从所有关联项目中移除，且无法恢复。
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel disabled={deleting}>取消</AlertDialogCancel>
						<AlertDialogAction
							variant="destructive"
							disabled={deleting}
							onClick={() => void deleteConnector()}
						>
							{deleting ? "删除中..." : "确认删除"}
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</div>
	);
}

function MCPField({
	label,
	value,
	placeholder,
	disabled,
	onChange,
}: {
	label: string;
	value: string;
	placeholder?: string;
	disabled?: boolean;
	onChange: (value: string) => void;
}) {
	return (
		<label className="grid gap-1.5 text-[13px] font-semibold text-[var(--leros-text-strong)]">
			{label}
			<input
				value={value}
				placeholder={placeholder}
				disabled={disabled}
				onChange={(event) => onChange(event.target.value)}
				className="h-10 rounded-lg border border-[var(--leros-control-border)] bg-[var(--leros-surface-soft)] px-3 text-xs font-normal outline-none transition-colors placeholder:text-[var(--leros-text-subtle)] focus:border-[var(--leros-primary)] focus:bg-white disabled:opacity-60"
			/>
		</label>
	);
}
