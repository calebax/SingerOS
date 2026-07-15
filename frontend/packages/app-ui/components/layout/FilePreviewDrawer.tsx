"use client";

import { type BackendProjectFileVersion, projectFileApi } from "@leros/store";
import { cn } from "@leros/ui/lib/utils";
import {
	ChevronsLeftRightEllipsis,
	Download,
	FileText,
	History,
	LoaderCircle,
	RotateCcw,
	ShieldCheck,
	X,
} from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { toast } from "sonner";
import { MarkdownRenderer } from "../common/MarkdownRenderer";
import { filePreviewActions } from "./file-preview-store";
import {
	detectFilePreviewKind,
	downloadFilePreviewContent,
	FILE_PREVIEW_DRAWER_DEFAULT_WIDTH,
	FILE_PREVIEW_DRAWER_MAX_WIDTH,
	FILE_PREVIEW_DRAWER_MIN_WIDTH,
	type FilePreviewItem,
	type FilePreviewKind,
	type FilePreviewState,
	fetchFilePreviewContent,
	PROJECT_FILE_RESTORED_EVENT,
} from "./file-preview-utils";
import { OfficePreview } from "./OfficePreview";
import { SpreadsheetPreview } from "./SpreadsheetPreview";

export type { FilePreviewItem } from "./file-preview-utils";

export function FilePreviewDrawer({
	file,
	open,
	onOpenChange,
}: {
	file: FilePreviewItem | null;
	open: boolean;
	onOpenChange: (open: boolean) => void;
}) {
	const [preview, setPreview] = useState<FilePreviewState>({ status: "idle" });
	const [htmlView, setHtmlView] = useState<"preview" | "source">("preview");
	const [drawerWidth, setDrawerWidth] = useState(FILE_PREVIEW_DRAWER_DEFAULT_WIDTH);
	const [historyOpen, setHistoryOpen] = useState(false);
	const [versions, setVersions] = useState<BackendProjectFileVersion[]>([]);
	const [versionsLoading, setVersionsLoading] = useState(false);
	const [versionsError, setVersionsError] = useState<string | null>(null);
	const [selectedVersionPublicId, setSelectedVersionPublicId] = useState("");
	const [restoreTarget, setRestoreTarget] = useState<BackendProjectFileVersion | null>(null);
	const [restoring, setRestoring] = useState(false);
	const drawerRef = useRef<HTMLDivElement>(null);
	const selectedVersion = useMemo(
		() => versions.find((version) => version.public_id === selectedVersionPublicId) ?? null,
		[versions, selectedVersionPublicId],
	);
	const previewFile = useMemo(() => {
		if (!file || !selectedVersion) return file;
		return {
			...file,
			name: selectedVersion.name || file.name,
			title: `${selectedVersion.name || file.name} ${selectedVersion.version_label || `第 ${selectedVersion.version_no} 版`}`,
			mimeType: selectedVersion.mime_type || file.mimeType,
			storageUri: selectedVersion.storage_uri || undefined,
			versionPublicId: selectedVersion.public_id,
			versionLabel: selectedVersion.version_label,
			versionNo: selectedVersion.version_no,
		} satisfies FilePreviewItem;
	}, [file, selectedVersion]);
	const previewKind = useMemo(() => detectFilePreviewKind(previewFile), [previewFile]);
	const canShowHistory = Boolean(file?.projectId && file.publicId);

	const closePreview = () => {
		onOpenChange(false);
	};

	useEffect(() => {
		setHtmlView("preview");
	}, [file?.publicId, file?.name, selectedVersionPublicId]);

	useEffect(() => {
		if (!open || !file) {
			setPreview({ status: "idle" });
			setHistoryOpen(false);
			setVersions([]);
			setVersionsError(null);
			setSelectedVersionPublicId("");
			return;
		}
		if (previewKind === "unsupported") {
			setPreview({ status: "ready" });
			return;
		}
		if (!previewFile) {
			setPreview({ status: "error", message: "文件缺少预览来源" });
			return;
		}

		const currentFile = previewFile;
		let cancelled = false;
		let objectUrl: string | undefined;
		const controller = new AbortController();

		async function loadPreview() {
			setPreview({ status: "loading" });
			try {
				const response = await fetchFilePreviewContent(currentFile, {
					signal: controller.signal,
				});
				const mimeType =
					response.headers.get("content-type") ??
					currentFile.mimeType ??
					"application/octet-stream";

				if (previewKind === "markdown" || previewKind === "text") {
					const text = await response.text();
					if (!cancelled) setPreview({ status: "ready", text });
					return;
				}

				if (previewKind === "html") {
					const text = await response.text();
					const htmlBlob = new Blob([text], { type: "text/html" });
					objectUrl = URL.createObjectURL(htmlBlob);
					if (!cancelled) setPreview({ status: "ready", text, objectUrl, mimeType });
					return;
				}

				if (
					previewKind === "docx" ||
					previewKind === "xlsx" ||
					previewKind === "pptx" ||
					previewKind === "spreadsheet"
				) {
					const buffer = await response.arrayBuffer();
					if (!cancelled) setPreview({ status: "ready", buffer });
					return;
				}

				const blob = await response.blob();
				objectUrl = URL.createObjectURL(blob);
				if (!cancelled) setPreview({ status: "ready", objectUrl, mimeType });
			} catch (err) {
				if (cancelled || controller.signal.aborted) return;
				const message = err instanceof Error ? err.message : "预览加载失败";
				setPreview({ status: "error", message });
			}
		}

		loadPreview();

		return () => {
			cancelled = true;
			controller.abort();
			if (objectUrl) URL.revokeObjectURL(objectUrl);
		};
	}, [open, file, previewFile, previewKind]);

	useEffect(() => {
		if (open && file) {
			setHistoryOpen(Boolean(file.openHistory));
		}
	}, [open, file]);

	useEffect(() => {
		if (!open || !file?.projectId || !file.publicId || !historyOpen) return;
		const projectId = file.projectId;
		const filePublicId = file.publicId;

		let cancelled = false;
		async function loadVersions() {
			setVersionsLoading(true);
			setVersionsError(null);
			try {
				const response = await projectFileApi.versions(projectId, filePublicId);
				if (cancelled) return;
				if (response.data.code !== 0) {
					throw new Error(response.data.message || "版本历史加载失败");
				}
				const items = response.data.data?.items ?? [];
				setVersions(items);
				if (!selectedVersionPublicId) {
					setSelectedVersionPublicId(response.data.data?.current_file_public_id || filePublicId);
				}
			} catch (err) {
				if (!cancelled) {
					setVersionsError(err instanceof Error ? err.message : "版本历史加载失败");
				}
			} finally {
				if (!cancelled) setVersionsLoading(false);
			}
		}

		loadVersions();
		return () => {
			cancelled = true;
		};
	}, [open, file?.projectId, file?.publicId, historyOpen, selectedVersionPublicId]);

	useEffect(() => {
		if (!open) return;

		const handlePointerDown = (event: PointerEvent) => {
			const target = event.target;
			if (!(target instanceof Element)) return;
			if (drawerRef.current?.contains(target)) return;
			if (target.closest("[data-file-preview-trigger]")) return;
			onOpenChange(false);
		};

		document.addEventListener("pointerdown", handlePointerDown);
		return () => document.removeEventListener("pointerdown", handlePointerDown);
	}, [open, onOpenChange]);

	const handleDownload = async () => {
		if (!file) return;
		try {
			const response = await downloadFilePreviewContent(previewFile ?? file);
			const blob = await response.blob();
			const objectUrl = URL.createObjectURL(blob);
			const link = document.createElement("a");
			link.href = objectUrl;
			link.download = previewFile?.name || file.name;
			document.body.appendChild(link);
			link.click();
			link.remove();
			window.setTimeout(() => URL.revokeObjectURL(objectUrl), 0);
		} catch (err) {
			console.error("Failed to download file preview", err);
		}
	};

	const handleRestoreVersion = async () => {
		if (!file?.projectId || !restoreTarget) return;
		setRestoring(true);
		try {
			const response = await projectFileApi.restoreVersion(file.projectId, restoreTarget.public_id);
			if (response.data.code !== 0) {
				throw new Error(response.data.message || "恢复版本失败");
			}
			toast.success("已恢复为新的最新版本");
			const node = response.data.data;
			if (file && node) {
				filePreviewActions.open({
					...file,
					publicId: node.public_id ?? file.publicId,
					storageUri: node.storage_uri ?? file.storageUri,
					versionNo: node.version_no,
					versionLabel: node.version_label,
					versionCount: node.version_count,
				});
			}
			setRestoreTarget(null);
			setSelectedVersionPublicId("");
			window.dispatchEvent(
				new CustomEvent(PROJECT_FILE_RESTORED_EVENT, {
					detail: { projectId: file.projectId, taskId: file.taskId },
				}),
			);
		} catch (err) {
			toast.error(err instanceof Error ? err.message : "恢复版本失败");
		} finally {
			setRestoring(false);
		}
	};

	const handleDrawerResizeStart = (event: React.PointerEvent<HTMLElement>) => {
		event.preventDefault();
		const startX = event.clientX;
		const startWidth = drawerWidth;

		const handlePointerMove = (moveEvent: PointerEvent) => {
			const candidateWidth = startWidth - (moveEvent.clientX - startX);
			const maxWidth = Math.min(FILE_PREVIEW_DRAWER_MAX_WIDTH, window.innerWidth - 160);
			const nextWidth = Math.min(
				Math.max(candidateWidth, FILE_PREVIEW_DRAWER_MIN_WIDTH),
				Math.max(FILE_PREVIEW_DRAWER_MIN_WIDTH, maxWidth),
			);
			setDrawerWidth(nextWidth);
		};

		const handlePointerUp = () => {
			window.removeEventListener("pointermove", handlePointerMove);
			window.removeEventListener("pointerup", handlePointerUp);
		};

		window.addEventListener("pointermove", handlePointerMove);
		window.addEventListener("pointerup", handlePointerUp);
	};

	if (!open || !file) {
		return null;
	}

	const displayTitle = previewFile?.title || file.title || file.name;

	return (
		<div
			ref={drawerRef}
			className="fixed inset-y-4 right-4 z-50 flex flex-col overflow-hidden rounded-2xl border border-[var(--leros-control-border)] bg-[var(--leros-surface)] p-0 shadow-2xl"
			style={{ width: `${drawerWidth}px`, maxWidth: `${drawerWidth}px` }}
		>
			<button
				type="button"
				aria-label="拖动调整预览宽度"
				title="拖动调整预览宽度"
				onPointerDown={handleDrawerResizeStart}
				className="absolute left-0 top-0 z-10 flex h-full w-4 -translate-x-1/2 cursor-col-resize items-center justify-center"
			>
				<div className="flex h-16 w-2 items-center justify-center rounded-full bg-[var(--leros-surface-soft)] text-[var(--leros-text-muted)] shadow-sm ring-1 ring-[var(--leros-control-border)]">
					<ChevronsLeftRightEllipsis className="size-3" />
				</div>
			</button>
			<div className="flex items-center justify-between border-b border-[var(--leros-control-border)] px-6 py-4">
				<div className="min-w-0">
					<div className="truncate text-lg font-medium text-[var(--leros-text-strong)]">
						{displayTitle}
					</div>
				</div>
				<div className="flex items-center gap-2">
					{canShowHistory ? (
						<button
							type="button"
							onClick={() => setHistoryOpen((value) => !value)}
							className="group relative rounded-lg p-2 text-[var(--leros-text-muted)] transition-colors hover:bg-[var(--leros-primary-softer)] hover:text-[var(--leros-primary)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--leros-primary)]/30"
							aria-label="历史版本"
							title="历史版本"
						>
							<History className="size-4" />
							<span className="pointer-events-none absolute right-0 top-full z-30 mt-2 whitespace-nowrap rounded-md bg-[var(--leros-text-strong)] px-2 py-1 text-[11px] font-medium text-white opacity-0 shadow-sm group-hover:opacity-100 group-focus-visible:opacity-100">
								历史版本
							</span>
						</button>
					) : null}
					<button
						type="button"
						onClick={() => void handleDownload()}
						className="group relative rounded-lg p-2 text-[var(--leros-text-muted)] transition-colors hover:bg-[var(--leros-primary-softer)] hover:text-[var(--leros-primary)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--leros-primary)]/30"
						aria-label="下载文件"
						title="下载文件"
					>
						<Download className="size-4" />
						<span className="pointer-events-none absolute right-0 top-full z-30 mt-2 whitespace-nowrap rounded-md bg-[var(--leros-text-strong)] px-2 py-1 text-[11px] font-medium text-white opacity-0 shadow-sm group-hover:opacity-100 group-focus-visible:opacity-100">
							下载文件
						</span>
					</button>
					<button
						type="button"
						onClick={closePreview}
						className="rounded-lg p-2 text-[var(--leros-text-muted)] transition-colors hover:bg-[var(--leros-primary-softer)]"
						title="关闭"
					>
						<X className="size-4" />
					</button>
				</div>
			</div>
			<div className="flex min-h-0 flex-1 overflow-hidden bg-[var(--leros-surface-soft)]">
				<div className="flex min-w-0 flex-1 flex-col overflow-hidden p-6">
					<FilePreviewContent
						fileName={previewFile?.name || file.name}
						displayTitle={displayTitle}
						previewKind={previewKind}
						preview={preview}
						htmlView={htmlView}
						onHtmlViewChange={setHtmlView}
					/>
				</div>
				{historyOpen && canShowHistory ? (
					<FileVersionPanel
						currentPublicId={file.publicId ?? ""}
						selectedPublicId={selectedVersionPublicId || (file.publicId ?? "")}
						versions={versions}
						loading={versionsLoading}
						error={versionsError}
						onSelect={(version) => setSelectedVersionPublicId(version.public_id)}
						onRestore={setRestoreTarget}
					/>
				) : null}
			</div>
			{restoreTarget ? (
				<div className="absolute inset-0 z-20 flex items-center justify-center bg-black/20 px-8">
					<div className="w-full max-w-md rounded-xl bg-white p-5 shadow-xl">
						<div className="flex items-start justify-between gap-4">
							<div>
								<h3 className="text-base font-semibold text-[var(--leros-text-strong)]">
									恢复此版本
								</h3>
								<p className="mt-2 text-sm leading-6 text-[var(--leros-text-muted)]">
									恢复后，将基于该历史版本的内容生成一个新版本，并设为当前版本。当前版本及其他历史版本均会保留。
								</p>
							</div>
							<button
								type="button"
								onClick={() => setRestoreTarget(null)}
								className="rounded-lg p-1.5 text-[var(--leros-text-muted)] hover:bg-[var(--leros-surface-soft)]"
								disabled={restoring}
							>
								<X className="size-4" />
							</button>
						</div>
						<div className="mt-5 flex justify-end gap-2">
							<button
								type="button"
								onClick={() => setRestoreTarget(null)}
								className="rounded-lg px-4 py-2 text-sm text-[var(--leros-text-muted)] hover:bg-[var(--leros-surface-soft)]"
								disabled={restoring}
							>
								取消
							</button>
							<button
								type="button"
								onClick={() => void handleRestoreVersion()}
								className="inline-flex items-center gap-2 rounded-lg bg-[var(--leros-text-strong)] px-4 py-2 text-sm font-medium text-white disabled:opacity-60"
								disabled={restoring}
							>
								{restoring ? <LoaderCircle className="size-4 animate-spin" /> : null}
								确认恢复
							</button>
						</div>
					</div>
				</div>
			) : null}
		</div>
	);
}

function FileVersionPanel({
	currentPublicId,
	selectedPublicId,
	versions,
	loading,
	error,
	onSelect,
	onRestore,
}: {
	currentPublicId: string;
	selectedPublicId: string;
	versions: BackendProjectFileVersion[];
	loading: boolean;
	error: string | null;
	onSelect: (version: BackendProjectFileVersion) => void;
	onRestore: (version: BackendProjectFileVersion) => void;
}) {
	return (
		<aside className="flex w-52 shrink-0 flex-col border-l border-[var(--leros-control-border)] bg-white">
			<div className="border-b border-[var(--leros-control-border)] px-3 py-2.5">
				<div className="text-sm font-semibold text-[var(--leros-text-strong)]">历史记录</div>
				<div className="mt-0.5 text-xs text-[var(--leros-text-muted)]">
					{versions.length > 0 ? `${versions.length} 个版本` : "查看文件版本"}
				</div>
			</div>
			<div className="min-h-0 flex-1 overflow-auto p-1.5">
				{loading ? (
					<div className="flex items-center justify-center py-10 text-xs text-[var(--leros-text-muted)]">
						<LoaderCircle className="mr-2 size-3.5 animate-spin" />
						加载中
					</div>
				) : error ? (
					<div className="px-3 py-8 text-center text-xs text-[var(--leros-danger)]">{error}</div>
				) : versions.length === 0 ? (
					<div className="px-3 py-8 text-center text-xs text-[var(--leros-text-muted)]">
						暂无历史版本
					</div>
				) : (
					<div className="space-y-1">
						{versions.map((version) => {
							const isCurrent = version.public_id === currentPublicId;
							const isSelected = version.public_id === selectedPublicId;
							return (
								<div key={version.public_id} className="relative">
									<button
										type="button"
										onClick={() => onSelect(version)}
										className={cn(
											"w-full cursor-pointer rounded-md px-2.5 py-1.5 pr-8 text-left transition-colors",
											isSelected
												? "bg-[var(--leros-primary-softer)] text-[var(--leros-primary)]"
												: "hover:bg-[var(--leros-surface-soft)]",
										)}
									>
										<span className="block truncate text-xs font-semibold">
											{version.version_label || `第 ${version.version_no} 版`}
										</span>
										<span className="mt-0.5 block truncate text-[10px] text-[var(--leros-text-muted)]">
											{formatVersionTime(version.created_at)}
											{isCurrent ? (
												<span className="ml-1.5 rounded-full bg-[var(--leros-primary)]/10 px-1 py-0 text-[9px] text-[var(--leros-primary)]">
													最新
												</span>
											) : null}
										</span>
									</button>
									{!isCurrent ? (
										<button
											type="button"
											onClick={() => onRestore(version)}
											className="absolute right-1.5 top-1.5 rounded p-1 text-[var(--leros-text-muted)] hover:bg-white hover:text-[var(--leros-primary)]"
											title="恢复"
										>
											<RotateCcw className="size-3" />
										</button>
									) : null}
								</div>
							);
						})}
					</div>
				)}
			</div>
		</aside>
	);
}

function formatVersionTime(timestamp?: number): string {
	if (!timestamp) return "-";
	return new Intl.DateTimeFormat("zh-CN", {
		month: "2-digit",
		day: "2-digit",
		hour: "2-digit",
		minute: "2-digit",
	}).format(new Date(timestamp * 1000));
}

function FilePreviewContent({
	fileName,
	displayTitle,
	previewKind,
	preview,
	htmlView,
	onHtmlViewChange,
}: {
	fileName: string;
	displayTitle: string;
	previewKind: FilePreviewKind;
	preview: FilePreviewState;
	htmlView: "preview" | "source";
	onHtmlViewChange: (view: "preview" | "source") => void;
}) {
	if (preview.status === "loading" || preview.status === "idle") {
		return (
			<div className="flex flex-1 items-center justify-center text-sm text-[var(--leros-text-muted)]">
				<LoaderCircle className="mr-2 size-4 animate-spin" />
				加载预览中
			</div>
		);
	}

	if (preview.status === "error") {
		return (
			<div className="flex flex-1 items-center justify-center px-8 text-center text-sm text-[var(--leros-text-muted)]">
				<div>
					<p>无法加载文件预览</p>
					<p className="mt-1 text-xs">{preview.message}</p>
				</div>
			</div>
		);
	}

	if (preview.status !== "ready") {
		return null;
	}

	if (
		(previewKind === "docx" || previewKind === "xlsx" || previewKind === "pptx") &&
		preview.buffer
	) {
		return (
			<div className="min-h-0 flex-1 overflow-hidden rounded-xl bg-white shadow-sm">
				<OfficePreview buffer={preview.buffer} fileName={fileName} format={previewKind} />
			</div>
		);
	}

	if (previewKind === "spreadsheet" && preview.buffer) {
		return (
			<div className="min-h-0 flex-1 overflow-hidden rounded-xl bg-white shadow-sm">
				<SpreadsheetPreview buffer={preview.buffer} fileName={fileName} />
			</div>
		);
	}

	if (previewKind === "markdown") {
		return (
			<div className="min-h-0 flex-1 overflow-auto rounded-xl bg-white px-8 py-7 shadow-sm">
				<MarkdownRenderer
					content={preview.text ?? ""}
					className="prose prose-slate prose-sm max-w-none prose-headings:text-[var(--leros-text-strong)] prose-p:leading-7 prose-pre:rounded-lg prose-pre:bg-slate-950"
				/>
			</div>
		);
	}

	if (previewKind === "html" && preview.text !== undefined) {
		return (
			<div className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-xl bg-white shadow-sm">
				<div className="flex shrink-0 items-center justify-between border-b border-[var(--leros-control-border)] bg-[var(--leros-surface-soft)] px-3 py-2">
					<div className="flex items-center gap-1">
						{(["preview", "source"] as const).map((view) => (
							<button
								key={view}
								type="button"
								onClick={() => onHtmlViewChange(view)}
								className={cn(
									"rounded-md px-3 py-1.5 text-xs transition-colors",
									htmlView === view
										? "bg-white font-medium text-[var(--leros-text-strong)] shadow-sm"
										: "text-[var(--leros-text-muted)] hover:text-[var(--leros-text-strong)]",
								)}
							>
								{view === "preview" ? "预览" : "源码"}
							</button>
						))}
					</div>
					{htmlView === "preview" && (
						<span
							title="当前仅支持基础静态预览，包含脚本或复杂交互的页面请下载后用浏览器打开。"
							className="inline-flex items-center gap-1 text-[11px] text-[var(--leros-text-muted)]"
						>
							<ShieldCheck className="size-3.5 text-emerald-600" />
							安全预览
						</span>
					)}
				</div>
				{htmlView === "preview" && preview.objectUrl ? (
					<iframe
						title={`${displayTitle} 预览`}
						src={preview.objectUrl}
						sandbox=""
						referrerPolicy="no-referrer"
						className="min-h-0 flex-1 border-0 bg-white"
					/>
				) : (
					<pre className="min-h-0 flex-1 overflow-auto bg-slate-950 p-4 text-xs leading-6 text-slate-100">
						{preview.text}
					</pre>
				)}
			</div>
		);
	}

	if (previewKind === "text") {
		return (
			<pre className="min-h-0 flex-1 overflow-auto rounded-xl bg-white p-4 text-sm leading-6 text-[var(--leros-text)] shadow-sm">
				{preview.text ?? ""}
			</pre>
		);
	}

	if (previewKind === "image" && preview.objectUrl) {
		return (
			<div className="flex flex-1 items-center justify-center overflow-auto rounded-xl bg-white p-4 shadow-sm">
				<img
					src={preview.objectUrl}
					alt={displayTitle}
					className="max-h-full max-w-full object-contain"
				/>
			</div>
		);
	}

	if (previewKind === "pdf" && preview.objectUrl) {
		return (
			<div className="min-h-0 flex-1 overflow-hidden rounded-xl bg-white shadow-sm">
				<iframe
					title={displayTitle}
					src={preview.objectUrl}
					className="h-full w-full border-0 bg-white"
				/>
			</div>
		);
	}

	return (
		<div className="flex flex-1 items-center justify-center rounded-xl bg-white px-8 text-center text-sm text-[var(--leros-text-muted)] shadow-sm">
			<div>
				<FileText className="mx-auto mb-3 size-8 text-[var(--leros-text-subtle)]" />
				<p>此文件类型暂不支持内嵌预览</p>
				<p className="mt-1 text-xs">请使用下载按钮在本地查看</p>
			</div>
		</div>
	);
}
