"use client";

import {
	type BackendAITeammateTemplate,
	type DigitalAssistantItem,
	digitalAssistantApi,
	projectFileApi,
	useDAStore,
} from "@leros/store";
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
	Sheet,
	SheetContent,
	SheetFooter,
	SheetHeader,
	SheetTitle,
} from "@leros/ui/components/ui/sheet";
import { cn } from "@leros/ui/lib/utils";
import {
	ChartNoAxesCombined,
	FileSearch,
	ImagePlus,
	Lightbulb,
	Loader2,
	PenLine,
	WandSparkles,
} from "lucide-react";
import { type ChangeEvent, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { blobToDataURL, cacheProtectedImageDataURL } from "../avatar/ProtectedImage";
import { AssistantAvatar } from "./AssistantAvatar";

export type AssistantCreateDialogProps = {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	onCreated?: (assistant: DigitalAssistantItem) => void;
};

const PRESET_TEMPLATE_CODES = [
	"bid-strategist",
	"contract-review-expert",
	"data-analysis-expert",
	"document-generation-expert",
	"ai-ppt-expert",
	"recruiting-expert",
	"stock-investment-expert",
] as const;

export function AssistantCreateDialog({
	open,
	onOpenChange,
	onCreated,
}: AssistantCreateDialogProps) {
	const { createAssistant, createAssistantFromTemplate, updateAssistantStatus } = useDAStore(
		(s) => s,
	);
	const [templates, setTemplates] = useState<BackendAITeammateTemplate[]>([]);
	const [templatesLoading, setTemplatesLoading] = useState(false);
	const [selectedTemplate, setSelectedTemplate] = useState<BackendAITeammateTemplate | null>(null);
	const [detailTemplate, setDetailTemplate] = useState<BackendAITeammateTemplate | null>(null);
	const [customMode, setCustomMode] = useState(false);
	const [name, setName] = useState("");
	const [roleName, setRoleName] = useState("");
	const [introduction, setIntroduction] = useState("");
	const [avatar, setAvatar] = useState("");
	const [uploadingAvatar, setUploadingAvatar] = useState(false);
	const [previewAvatar, setPreviewAvatar] = useState<string | undefined>();
	const [submitting, setSubmitting] = useState(false);
	const selectionTouchedRef = useRef(false);

	useEffect(() => {
		// 中文注释：预设角色从后端模板读取，保证组织后续调整模板时创建入口无需重新发版。
		if (!open) return;
		let cancelled = false;
		selectionTouchedRef.current = false;
		setTemplatesLoading(true);
		void digitalAssistantApi
			.listTemplates({ status: "active", list_all: true, limit: 100 })
			.then((response) => {
				if (cancelled) return;
				const items = response.data.data?.items ?? [];
				const templatesByCode = new Map(items.map((item) => [item.code, item]));
				// 中文注释：创建入口只展示产品约定的七个内置角色，并保持固定顺序，不混入历史市场模板。
				const presetTemplates = PRESET_TEMPLATE_CODES.flatMap((code) => {
					const template = templatesByCode.get(code);
					return template ? [template] : [];
				});
				setTemplates(presetTemplates);
				// 中文注释：模板异步返回时，不覆盖用户已经切换的自定义模式或手动选择。
				if (presetTemplates[0] && !selectionTouchedRef.current) {
					selectTemplate(presetTemplates[0], false);
				}
			})
			.catch((error) => {
				if (cancelled) return;
				console.error("fetch ai teammate templates error:", error);
				toast.error("AI 队友模板加载失败");
			})
			.finally(() => {
				if (!cancelled) setTemplatesLoading(false);
			});
		return () => {
			cancelled = true;
		};
	}, [open]);

	const selectTemplate = (template: BackendAITeammateTemplate, markTouched = true) => {
		if (markTouched) selectionTouchedRef.current = true;
		setCustomMode(false);
		setSelectedTemplate(template);
		setRoleName(template.name);
		setIntroduction(template.description ?? "");
		setAvatar(template.avatar ?? "");
		setPreviewAvatar(undefined);
	};

	const switchToCustom = () => {
		selectionTouchedRef.current = true;
		setCustomMode(true);
		setSelectedTemplate(null);
		setName("");
		setRoleName("");
		setIntroduction("");
		setAvatar("");
		setPreviewAvatar(undefined);
		setDetailTemplate(null);
	};

	const formValid = Boolean(
		name.trim() && roleName.trim() && introduction.trim() && (customMode || selectedTemplate),
	);

	const handleSubmit = async () => {
		if (!formValid || submitting) {
			toast.error("请填写自定义名称、角色名称和简介");
			return;
		}
		setSubmitting(true);
		try {
			let assistant: DigitalAssistantItem | null;
			if (selectedTemplate) {
				assistant = await createAssistantFromTemplate({
					template_id: selectedTemplate.id,
					name: name.trim(),
					role_name: roleName.trim(),
				});
			} else {
				assistant = await createAssistant({
					name: name.trim(),
					role_name: roleName.trim(),
					avatar: avatar.trim() || undefined,
					// 中文注释：当前自定义创建只保留一个“简介”输入，同时用于卡片简介和角色设定。
					description: introduction.trim(),
					system_prompt: `你是${roleName.trim()}，名称是${name.trim()}。${introduction.trim()}`,
				});
				if (assistant) {
					const activated = await updateAssistantStatus(assistant.id, "active");
					if (!activated) {
						toast.error("AI 队友已创建，但启用失败；可稍后在列表中重试");
						handleClose();
						return;
					}
					assistant = { ...assistant, status: "active", deploymentStatus: "pending" };
				}
			}
			if (!assistant) {
				toast.error("创建队友失败");
				return;
			}
			toast.success("AI 队友创建中，请等待部署完成后再使用");
			onCreated?.(assistant);
			handleClose();
		} finally {
			setSubmitting(false);
		}
	};

	const handleClose = () => {
		setTemplates([]);
		setSelectedTemplate(null);
		setDetailTemplate(null);
		setCustomMode(false);
		setName("");
		setRoleName("");
		setIntroduction("");
		setAvatar("");
		setPreviewAvatar(undefined);
		onOpenChange(false);
	};

	const handleAvatarChange = async (event: ChangeEvent<HTMLInputElement>) => {
		const file = event.target.files?.[0];
		event.target.value = "";
		if (!file) return;
		if (!isImageFile(file)) {
			toast.error("请选择图片文件");
			return;
		}
		const previewURL = URL.createObjectURL(file);
		setPreviewAvatar(previewURL);
		setUploadingAvatar(true);
		try {
			const response = await projectFileApi.uploadLoose({ file, purpose: "avatar" });
			const publicId = response.data?.public_id;
			if (!publicId) throw new Error("头像上传失败");
			// 中文注释：AI 队友头像字段保存文件 public_id，展示时统一通过 preview 接口读取。
			setAvatar(publicId);
			void blobToDataURL(file).then((dataURL) => cacheProtectedImageDataURL(publicId, dataURL));
			setPreviewAvatar(undefined);
			toast.success("头像已上传");
		} catch (error) {
			console.error("upload ai teammate avatar error:", error);
			toast.error(error instanceof Error ? error.message : "头像上传失败");
			setPreviewAvatar(undefined);
		} finally {
			URL.revokeObjectURL(previewURL);
			setUploadingAvatar(false);
		}
	};

	return (
		<>
			<Dialog
				open={open}
				onOpenChange={(nextOpen) => {
					if (!nextOpen && !submitting && !uploadingAvatar) handleClose();
				}}
			>
				<DialogContent className="flex max-h-[min(92dvh,820px)] max-w-[min(94vw,960px)] flex-col overflow-hidden p-0 sm:rounded-2xl">
					<DialogHeader className="border-b border-slate-200 px-6 py-5">
						<DialogTitle>创建 AI 队友</DialogTitle>
						<DialogDescription>选择一个预设角色，或创建自定义 AI 队友</DialogDescription>
					</DialogHeader>
					<div className="min-h-0 flex-1 overflow-y-auto px-6 py-5">
						<div className="flex items-center justify-between">
							<h3 className="text-sm font-semibold text-slate-900">选择角色</h3>
							<Button type="button" variant="ghost" size="sm" onClick={switchToCustom}>
								自定义 AI 队友
							</Button>
						</div>
						{templatesLoading ? (
							<div className="flex h-40 items-center justify-center text-sm text-slate-500">
								<Loader2 className="mr-2 size-4 animate-spin" />
								加载角色模板…
							</div>
						) : templates.length > 0 ? (
							<div className="mt-3 grid gap-3 md:grid-cols-2">
								{templates.map((template) => (
									<div
										key={template.id}
										className={cn(
											"flex items-center gap-3 rounded-xl border p-3 transition-colors",
											selectedTemplate?.id === template.id && !customMode
												? "border-slate-900 bg-slate-50"
												: "border-slate-200 hover:border-slate-400",
										)}
									>
										<button
											type="button"
											className="flex min-w-0 flex-1 items-center gap-3 text-left"
											onClick={() => selectTemplate(template)}
										>
											<AssistantAvatar name={template.name} src={template.avatar} />
											<span className="min-w-0 flex-1">
												<span className="block text-sm font-medium text-slate-900">
													{template.name}
												</span>
												<span className="mt-1 block truncate text-xs text-slate-500">
													{template.description}
												</span>
											</span>
										</button>
										<Button
											type="button"
											variant="ghost"
											size="sm"
											onClick={() => setDetailTemplate(template)}
										>
											查看详情
										</Button>
									</div>
								))}
							</div>
						) : (
							<div className="flex h-40 items-center justify-center text-sm text-slate-500">
								暂无可用的预设角色，可先创建自定义 AI 队友
							</div>
						)}

						<div className="mt-6 border-t border-slate-200 pt-5">
							<h3 className="text-sm font-semibold text-slate-900">
								{customMode ? "自定义 AI 队友" : "完善队友信息"}
							</h3>
							<div className="mt-4 grid gap-4 md:grid-cols-[auto_1fr_1fr]">
								<div className="flex items-center gap-3 md:row-span-2 md:flex-col md:items-start">
									<AssistantAvatar
										name={name || roleName || "AI"}
										src={previewAvatar || avatar}
										size="lg"
									/>
									{customMode ? (
										<label className="inline-flex h-8 cursor-pointer items-center rounded-md border border-slate-200 px-2 text-xs font-medium text-slate-700 hover:bg-slate-50">
											<ImagePlus className="mr-1.5 size-3.5" />
											{uploadingAvatar ? "上传中" : "上传头像"}
											<input
												type="file"
												accept="image/*"
												className="sr-only"
												onChange={handleAvatarChange}
												disabled={uploadingAvatar}
											/>
										</label>
									) : null}
								</div>
								<Field
									label="自定义名称"
									value={name}
									onChange={setName}
									placeholder="例如：小投、法务小周"
								/>
								<Field
									label="角色名称"
									value={roleName}
									onChange={setRoleName}
									placeholder="例如：投标经理"
									readOnly={!customMode}
								/>
								<label className="space-y-1.5 md:col-span-2">
									<span className="text-xs font-medium text-slate-700">
										简介 <span className="text-red-500">*</span>
									</span>
									<textarea
										value={introduction}
										onChange={(event) => setIntroduction(event.target.value)}
										rows={3}
										readOnly={!customMode}
										className="w-full resize-none rounded-md border border-slate-200 bg-white px-3 py-2 text-sm text-slate-800 read-only:bg-slate-50 focus:border-blue-300 focus:outline-none"
									/>
								</label>
							</div>
						</div>
					</div>
					<DialogFooter className="border-t border-slate-200 px-6 py-4">
						<Button
							variant="outline"
							onClick={handleClose}
							disabled={submitting || uploadingAvatar}
						>
							取消
						</Button>
						<Button onClick={handleSubmit} disabled={!formValid || uploadingAvatar || submitting}>
							{submitting ? "创建中…" : "创建并启用"}
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			<TemplateDetailSheet
				template={detailTemplate}
				onOpenChange={(nextOpen) => !nextOpen && setDetailTemplate(null)}
				onSelect={(template) => {
					selectTemplate(template);
					setDetailTemplate(null);
				}}
			/>
		</>
	);
}

function Field({
	label,
	value,
	placeholder,
	readOnly,
	onChange,
}: {
	label: string;
	value: string;
	placeholder: string;
	readOnly?: boolean;
	onChange: (value: string) => void;
}) {
	return (
		<label className="space-y-1.5">
			<span className="text-xs font-medium text-slate-700">
				{label} <span className="text-red-500">*</span>
			</span>
			<input
				type="text"
				value={value}
				onChange={(event) => onChange(event.target.value)}
				placeholder={placeholder}
				readOnly={readOnly}
				className="w-full rounded-md border border-slate-200 bg-white px-3 py-2 text-sm text-slate-800 read-only:bg-slate-50 focus:border-blue-300 focus:outline-none"
			/>
		</label>
	);
}

function TemplateDetailSheet({
	template,
	onOpenChange,
	onSelect,
}: {
	template: BackendAITeammateTemplate | null;
	onOpenChange: (open: boolean) => void;
	onSelect: (template: BackendAITeammateTemplate) => void;
}) {
	return (
		<Sheet open={!!template} onOpenChange={onOpenChange}>
			<SheetContent className="gap-0 sm:max-w-[680px]">
				{template ? (
					<>
						<SheetHeader className="border-b border-slate-200 px-6 py-5 pr-14">
							<SheetTitle className="text-base font-semibold text-slate-900">员工详情</SheetTitle>
						</SheetHeader>
						<div className="border-b border-slate-100 px-6 py-6">
							<div className="flex items-start gap-4">
								<AssistantAvatar name={template.name} src={template.avatar} size="lg" />
								<div className="min-w-0">
									<h2 className="text-xl font-semibold text-slate-900">{template.name}</h2>
									<p className="mt-2 text-sm leading-6 text-slate-500">
										{template.description || "暂无角色介绍"}
									</p>
								</div>
							</div>
						</div>
						<div className="min-h-0 flex-1 overflow-y-auto p-6">
							<h3 className="text-base font-semibold text-slate-900">核心能力</h3>
							<div className="mt-4 space-y-3">
								{(template.expertise ?? []).map((item, index) => {
									const ExpertiseIcon =
										EXPERTISE_ICONS[index % EXPERTISE_ICONS.length] ?? FileSearch;
									return (
										<div
											key={item}
											className="flex items-center gap-3 rounded-xl border border-slate-200 bg-white px-4 py-4"
										>
											<ExpertiseIcon className="size-5 shrink-0 text-blue-500" aria-hidden="true" />
											<span className="text-sm font-medium text-slate-800">{item}</span>
										</div>
									);
								})}
							</div>
						</div>
						<SheetFooter className="flex-row justify-start border-t border-slate-200 p-4">
							<Button className="min-w-28" onClick={() => onSelect(template)}>
								就他了！
							</Button>
						</SheetFooter>
					</>
				) : null}
			</SheetContent>
		</Sheet>
	);
}

// 中文注释：擅长领域目前只提供文本，按顺序轮换图标以增强能力卡片的可扫描性。
const EXPERTISE_ICONS = [FileSearch, ChartNoAxesCombined, PenLine, WandSparkles, Lightbulb];

function isImageFile(file: File): boolean {
	if (file.type.startsWith("image/")) return true;
	return /\.(avif|bmp|gif|jpe?g|png|svg|webp)$/i.test(file.name);
}
