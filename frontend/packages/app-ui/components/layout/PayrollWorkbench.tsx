"use client";

import { projectFileApi, useLayoutStore } from "@leros/store";
import type { Attachment } from "@leros/store/types/chat";
import { Button } from "@leros/ui/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@leros/ui/components/ui/dialog";
import { cn } from "@leros/ui/lib/utils";
import { Calculator, FileSpreadsheet, Upload, X } from "lucide-react";
import { useMemo, useRef, useState } from "react";
import { toast } from "sonner";
import { useAuth } from "../auth";
import type { AppNavigation } from "./LeftRail";
import { ProjectTaskPickerField } from "./ProjectTaskPicker";

type SelectedFile = {
	id: string;
	file: File;
};

const PAYROLL_STARTER_PROMPT = `请分析我上传的考勤和工资资料，梳理人员、考勤、工资基准及待核对项。
请先说明资料完整性和无法确认的规则，不要自行推断病假、旷工、跨项目补贴或入离职的工资金额。`;

export function PayrollWorkbench({ navigation }: { navigation?: AppNavigation }) {
	const { projects, fetchProjects, fetchTasks, sendWorkbenchMessage } = useLayoutStore((s) => s);
	const { isAuthenticated, requireAuth } = useAuth();
	const [dialogOpen, setDialogOpen] = useState(false);
	const [projectId, setProjectId] = useState("");
	const [taskId, setTaskId] = useState("");
	const [files, setFiles] = useState<SelectedFile[]>([]);
	const [submitting, setSubmitting] = useState(false);
	const inputRef = useRef<HTMLInputElement>(null);

	const projectOptions = useMemo(
		() => projects.map((project) => ({ id: project.id, name: project.name, tasks: project.tasks })),
		[projects],
	);

	const openDialog = () => {
		requireAuth(() => {
			void fetchProjects();
			setDialogOpen(true);
		});
	};

	const resetDialog = () => {
		setProjectId("");
		setTaskId("");
		setFiles([]);
	};

	const addFiles = (selected: FileList | null) => {
		const nextFiles = Array.from(selected ?? []);
		if (!nextFiles.length) return;
		setFiles((current) => {
			const existing = new Set(
				current.map(({ file }) => `${file.name}:${file.size}:${file.lastModified}`),
			);
			const additions = nextFiles
				.filter((file) => {
					const key = `${file.name}:${file.size}:${file.lastModified}`;
					if (existing.has(key)) return false;
					existing.add(key);
					return true;
				})
				.map((file) => ({ id: `payroll-${crypto.randomUUID()}`, file }));
			return [...current, ...additions];
		});
	};

	const uploadFiles = async (): Promise<Attachment[]> => {
		const attachments: Attachment[] = [];
		for (const selected of files) {
			const response = projectId
				? await projectFileApi.upload({
						projectId,
						projectPublicId: projectId,
						file: selected.file,
					})
				: await projectFileApi.uploadLoose({
						file: selected.file,
						purpose: "attachment",
						withLocalPath: true,
					});
			const payload = response.data;
			const fileUploadId = payload.public_id?.trim();
			if (!fileUploadId) throw new Error(`文件「${selected.file.name}」上传失败`);
			attachments.push({
				id: selected.id,
				type: selected.file.type.startsWith("image/") ? "image" : "file",
				name: payload.original_name || payload.filename || selected.file.name,
				size: payload.file_size ?? payload.size ?? selected.file.size,
				fileUploadId,
				mimeType: payload.mime_type || selected.file.type,
				storageUri: payload.storage_uri,
				uploadStatus: "completed",
			});
		}
		return attachments;
	};

	const startAnalysis = async () => {
		if (!files.length || submitting) return;
		setSubmitting(true);
		try {
			const attachments = await uploadFiles();
			const result = await sendWorkbenchMessage(
				PAYROLL_STARTER_PROMPT,
				projectId || undefined,
				"default",
				attachments,
				undefined,
				undefined,
				undefined,
				undefined,
				undefined,
				taskId || null,
			);
			if (!result?.project_id || !result.task_id || !result.session_id) {
				throw new Error("创建工资核算任务失败");
			}
			setDialogOpen(false);
			resetDialog();
			navigation?.goToTaskDetail(result.project_id, result.task_id, result.session_id);
		} catch (error) {
			console.error("PayrollWorkbench start analysis error:", error);
			toast.error(error instanceof Error ? error.message : "启动工资核算分析失败");
		} finally {
			setSubmitting(false);
		}
	};

	return (
		<div className="flex h-full min-h-0 flex-1 flex-col bg-[var(--leros-app-bg)]">
			<header className="shrink-0 border-b border-[var(--leros-control-border)] px-6 py-5">
				<h1 className="text-xl font-semibold text-[var(--leros-text-strong)]">工作台</h1>
				<p className="mt-2 text-sm text-[var(--leros-text-muted)]">
					选择固定业务功能，快速开始处理工作资料。
				</p>
			</header>
			<main className="flex min-h-0 flex-1 flex-col px-6 py-6">
				<div className="w-full">
					<div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
						<button
							type="button"
							onClick={openDialog}
							className={cn(
								"group flex min-h-[132px] cursor-pointer flex-col rounded-lg border border-slate-200 bg-white p-4",
								"text-left transition-colors hover:border-[var(--leros-primary-soft)]",
								"hover:bg-[var(--leros-primary-softer)]/35",
							)}
						>
							<div className="flex size-10 items-center justify-center rounded-lg bg-emerald-50 text-emerald-600">
								<Calculator className="size-5" />
							</div>
							<h3 className="mt-4 text-sm font-semibold text-[var(--leros-text-strong)]">
								考勤工资核算
							</h3>
							<p className="mt-1 text-xs leading-5 text-[var(--leros-text-muted)]">
								上传人员、历史工资和当月考勤资料，发起工资核算分析任务。
							</p>
						</button>
					</div>
				</div>
			</main>

			<Dialog
				open={dialogOpen}
				onOpenChange={(open) => {
					setDialogOpen(open);
					if (!open) resetDialog();
				}}
			>
				<DialogContent className="max-w-[560px]">
					<DialogHeader>
						<div className="flex size-10 items-center justify-center rounded-xl bg-emerald-50 text-emerald-600">
							<Calculator className="size-5" />
						</div>
						<DialogTitle className="mt-3">考勤工资核算</DialogTitle>
						<DialogDescription>
							上传人员底表、历史工资表和当月考勤资料，开始一个资料分析任务。
						</DialogDescription>
					</DialogHeader>

					<ProjectTaskPickerField
						projects={projectOptions}
						projectId={projectId}
						taskId={taskId}
						allowNewProject
						allowSelectTask
						onLoadProjectTasks={fetchTasks}
						onSelect={(nextProjectId, nextTaskId) => {
							setProjectId(nextProjectId);
							setTaskId(nextTaskId);
							if (nextProjectId) fetchTasks(nextProjectId);
						}}
					/>

					<div>
						<div className="mb-2 flex items-center justify-between">
							<div className="text-sm font-semibold text-slate-800">核算资料</div>
							<span className="text-xs text-slate-400">至少上传 1 份文件</span>
						</div>
						<div className="rounded-xl border border-dashed border-slate-200 bg-slate-50/60 p-3">
							{files.length ? (
								<div className="space-y-2">
									{files.map((selected) => (
										<div
											key={selected.id}
											className="flex items-center gap-2 rounded-lg bg-white px-3 py-2 text-sm shadow-sm"
										>
											<FileSpreadsheet className="size-4 shrink-0 text-emerald-600" />
											<span className="min-w-0 flex-1 truncate text-slate-700">
												{selected.file.name}
											</span>
											<button
												type="button"
												onClick={() =>
													setFiles((current) => current.filter((file) => file.id !== selected.id))
												}
												className="text-slate-400 hover:text-slate-700"
												aria-label={`移除 ${selected.file.name}`}
											>
												<X className="size-4" />
											</button>
										</div>
									))}
								</div>
							) : (
								<p className="py-4 text-center text-xs text-slate-400">
									建议上传人员底表、历史工资表和考勤表
								</p>
							)}
							<Button
								type="button"
								size="sm"
								variant="outline"
								className="mt-3"
								onClick={() => inputRef.current?.click()}
							>
								<Upload className="size-3.5" />
								选择文件
							</Button>
						</div>
					</div>

					<DialogFooter>
						<Button type="button" variant="ghost" onClick={() => setDialogOpen(false)}>
							取消
						</Button>
						<Button
							type="button"
							onClick={() => void startAnalysis()}
							disabled={!isAuthenticated || submitting || files.length === 0}
							className="bg-[var(--leros-primary)] text-white hover:bg-[var(--leros-primary)]/90"
						>
							{submitting ? "启动中..." : "开始分析"}
						</Button>
					</DialogFooter>
					<input
						ref={inputRef}
						type="file"
						className="hidden"
						multiple
						onChange={(event) => {
							addFiles(event.target.files);
							event.target.value = "";
						}}
					/>
				</DialogContent>
			</Dialog>
		</div>
	);
}
