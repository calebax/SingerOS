"use client";

import { type PluginListItem, pluginApi } from "@leros/store";
import { Button } from "@leros/ui/components/ui/button";
import { Search, Server, SlidersHorizontal } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";

const PAGE_SIZE = 90;

type ConnectorFilter = "all" | "connected" | "disconnected";

const FILTERS: Array<{ value: ConnectorFilter; label: string }> = [
	{ value: "all", label: "全部" },
	{ value: "connected", label: "已连接" },
	{ value: "disconnected", label: "未连接" },
];

function isConnected(connector: PluginListItem) {
	return connector.status === "active";
}

function connectorOriginLabel(origin: string) {
	switch (origin) {
		case "builtin":
		case "builtin_worker":
			return "内置";
		case "marketplace":
			return "官方";
		case "manual":
			return "自定义";
		default:
			return "组织接入";
	}
}

export function McpConnectorPanel({ isAuthenticated = true }: { isAuthenticated?: boolean }) {
	const [connectors, setConnectors] = useState<PluginListItem[]>([]);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [keyword, setKeyword] = useState("");
	const [filter, setFilter] = useState<ConnectorFilter>("all");

	useEffect(() => {
		if (!isAuthenticated) {
			setConnectors([]);
			setLoading(false);
			setError(null);
			return;
		}
		let cancelled = false;
		const fetchConnectors = async () => {
			setLoading(true);
			setError(null);
			try {
				const response = await pluginApi.list({
					kind: "mcp",
					limit: PAGE_SIZE,
				});
				if (!cancelled) setConnectors(response.data.data.plugins ?? []);
			} catch (requestError: any) {
				if (!cancelled) {
					setError(requestError?.response?.data?.message ?? requestError?.message ?? "加载失败");
				}
			} finally {
				if (!cancelled) setLoading(false);
			}
		};
		void fetchConnectors();
		return () => {
			cancelled = true;
		};
	}, [isAuthenticated]);

	const connectorCounts = useMemo(
		() => ({
			all: connectors.length,
			connected: connectors.filter(isConnected).length,
			disconnected: connectors.filter((connector) => !isConnected(connector)).length,
		}),
		[connectors],
	);

	const visibleConnectors = useMemo(() => {
		const normalizedKeyword = keyword.trim().toLocaleLowerCase();
		return connectors.filter((connector) => {
			const matchesKeyword =
				!normalizedKeyword ||
				[connector.name, connector.code, connector.description]
					.filter(Boolean)
					.join(" ")
					.toLocaleLowerCase()
					.includes(normalizedKeyword);
			const connected = isConnected(connector);
			const matchesFilter =
				filter === "all" ||
				(filter === "connected" && connected) ||
				(filter === "disconnected" && !connected);
			return matchesKeyword && matchesFilter;
		});
	}, [connectors, filter, keyword]);

	const handleConfigureCustom = () => {
		toast.message("自定义 MCP 配置功能即将开放");
	};

	const handleConnectorAction = (connector: PluginListItem) => {
		toast.message(isConnected(connector) ? "连接管理功能即将开放" : "连接配置功能即将开放");
	};

	return (
		<div className="flex min-h-0 flex-1 flex-col">
			<section className="shrink-0 border-b border-[var(--leros-control-border)] bg-white px-5 pb-4 pt-5">
				<div className="mx-auto w-full max-w-[1480px]">
					<h2 className="text-xl font-semibold tracking-tight text-[var(--leros-text-strong)]">
						MCP 连接器
					</h2>
					<p className="mt-1 text-xs text-[var(--leros-text-muted)]">
						统一管理外部系统连接，连接后可在任务和项目中按需调用。连接仅限当前组织使用。
					</p>

					<div className="mt-4 flex flex-wrap items-center gap-2">
						<div className="relative w-[420px] min-w-[260px] max-w-full flex-none">
							<Search className="absolute left-3 top-1/2 size-3.5 -translate-y-1/2 text-[var(--leros-text-subtle)]" />
							<input
								type="search"
								aria-label="搜索 MCP 连接器"
								placeholder="搜索连接器"
								value={keyword}
								onChange={(event) => setKeyword(event.target.value)}
								className="h-9 w-full rounded-md border border-[var(--leros-control-border)] bg-[var(--leros-surface-soft)] pl-9 pr-3 text-xs text-[var(--leros-text)] outline-none transition-colors placeholder:text-[var(--leros-text-subtle)] focus:border-[var(--leros-primary)] focus:bg-white"
							/>
						</div>

						<fieldset className="flex items-center gap-2">
							<legend className="sr-only">连接状态筛选</legend>
							{FILTERS.map((item) => (
								<button
									key={item.value}
									type="button"
									aria-pressed={filter === item.value}
									onClick={() => setFilter(item.value)}
									className="h-9 rounded-md border border-[var(--leros-control-border)] bg-white px-3 text-xs font-medium text-[var(--leros-text-muted)] transition-colors hover:text-[var(--leros-text-strong)] aria-pressed:border-[var(--leros-primary)] aria-pressed:bg-[var(--leros-primary-soft)] aria-pressed:text-[var(--leros-primary)]"
								>
									{item.label}
									<span className="ml-1.5 opacity-60">{connectorCounts[item.value]}</span>
								</button>
							))}
						</fieldset>
					</div>
				</div>
			</section>

			<section className="min-h-0 flex-1 overflow-y-auto bg-[var(--leros-surface-soft)] px-5 py-4">
				<div className="mx-auto w-full max-w-[1480px]">
					<div className="flex items-center justify-between gap-4 rounded-lg border border-[var(--leros-primary-soft)] bg-white px-4 py-3">
						<div className="flex min-w-0 items-center gap-3">
							<div className="flex size-9 shrink-0 items-center justify-center rounded-md bg-[var(--leros-primary-soft)] text-[var(--leros-primary)]">
								<SlidersHorizontal className="size-4" />
							</div>
							<div className="min-w-0">
								<h3 className="text-sm font-semibold text-[var(--leros-text-strong)]">
									自定义 MCP 服务
								</h3>
								<p className="mt-0.5 text-xs text-[var(--leros-text-muted)]">
									通过 MCP JSON 配置组织服务，配置内容仅对当前组织可见和可用。
								</p>
							</div>
						</div>
						<Button
							type="button"
							variant="outline"
							disabled={!isAuthenticated}
							onClick={handleConfigureCustom}
							className="h-8 shrink-0 rounded-md px-3 text-xs"
						>
							配置自定义 MCP
						</Button>
					</div>

					<div className="mb-3 mt-5 flex items-center justify-between">
						<h3 className="text-sm font-semibold text-[var(--leros-text-strong)]">MCP 连接器</h3>
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
						<div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-[var(--leros-control-border)] bg-white py-16 text-center">
							<div className="flex size-10 items-center justify-center rounded-lg bg-[var(--leros-primary-soft)] text-[var(--leros-primary)]">
								<Server className="size-5" />
							</div>
							<p className="mt-3 text-sm font-medium text-[var(--leros-text-strong)]">
								{!isAuthenticated
									? "登录后查看组织连接器"
									: keyword || filter !== "all"
										? "暂无符合条件的连接器"
										: "暂无 MCP 连接器"}
							</p>
							<p className="mt-1 text-xs text-[var(--leros-text-muted)]">
								{isAuthenticated ? "可通过上方入口配置自定义 MCP 服务" : "连接器配置按当前组织隔离"}
							</p>
						</div>
					) : (
						<div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
							{visibleConnectors.map((connector) => {
								const connected = isConnected(connector);
								const displayName = connector.name || connector.code;
								return (
									<article
										key={connector.public_id}
										className="flex min-h-[168px] flex-col rounded-lg border border-[var(--leros-control-border)] bg-white p-4"
									>
										<div className="flex items-start gap-3">
											<div className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-[var(--leros-surface-soft)] text-sm font-semibold text-[var(--leros-text-strong)]">
												{displayName.charAt(0).toUpperCase()}
											</div>
											<div className="min-w-0 flex-1">
												<h4 className="truncate text-[13px] font-semibold text-[var(--leros-text-strong)]">
													{displayName}
												</h4>
												<p className="mt-0.5 truncate text-[11px] text-[var(--leros-text-subtle)]">
													{connectorOriginLabel(connector.origin)}
												</p>
											</div>
											<span
												className={
													connected
														? "shrink-0 text-[11px] font-medium text-emerald-600"
														: "shrink-0 text-[11px] text-[var(--leros-text-subtle)]"
												}
											>
												<span
													aria-hidden="true"
													className={`mr-1.5 inline-block size-1.5 rounded-full ${
														connected
															? "bg-emerald-500"
															: "border border-[var(--leros-text-subtle)] bg-white"
													}`}
												/>
												{connected ? "已连接" : "未连接"}
											</span>
										</div>

										<p className="mt-3 line-clamp-2 min-h-10 text-xs leading-5 text-[var(--leros-text-muted)]">
											{connector.description || "暂无连接器说明"}
										</p>

										<div className="mt-auto pt-3">
											<Button
												type="button"
												variant={connected ? "outline" : "default"}
												onClick={() => handleConnectorAction(connector)}
												className="h-8 rounded-md px-3 text-xs"
											>
												{connected ? "管理连接" : "立即连接"}
											</Button>
										</div>
									</article>
								);
							})}
						</div>
					)}
				</div>
			</section>
		</div>
	);
}
