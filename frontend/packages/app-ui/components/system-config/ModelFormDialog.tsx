"use client";

import type {
	CreateModelParams,
	ModelItem,
	TestModelParams,
	UpdateModelParams,
} from "@leros/store";
import { useModelStore } from "@leros/store";
import { Button } from "@leros/ui/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@leros/ui/components/ui/dialog";
import { Input } from "@leros/ui/components/ui/input";
import { Label } from "@leros/ui/components/ui/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@leros/ui/components/ui/select";
import { Switch } from "@leros/ui/components/ui/switch";
import { CheckCircle2, Loader2, Wifi } from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";

const PROVIDER_OPTIONS = [{ value: "openai", label: "自定义 (OpenAI 兼容)" }];

const PURPOSE_OPTIONS = [
	{ value: "conversation", label: "对话" },
	{ value: "translation", label: "翻译" },
];

export type ModelFormDialogProps = {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	/** 传入 model 表示编辑，否则为创建 */
	model?: ModelItem | null;
};

export function ModelFormDialog({ open, onOpenChange, model }: ModelFormDialogProps) {
	const { createModel, updateModel, testModel } = useModelStore((s) => s);
	const [provider, setProvider] = useState("openai");
	const [purpose, setPurpose] = useState("conversation");
	const [name, setName] = useState("");
	const [modelName, setModelName] = useState("");
	const [baseUrl, setBaseUrl] = useState("");
	const [apiKey, setApiKey] = useState("");
	const [temperature, setTemperature] = useState("1");
	const [maxTokens, setMaxTokens] = useState("");
	const [description, setDescription] = useState("");
	const [vision, setVision] = useState(false);
	const [configTopP, setConfigTopP] = useState("");
	const [configFrequencyPenalty, setConfigFrequencyPenalty] = useState("");
	const [configPresencePenalty, setConfigPresencePenalty] = useState("");
	const [configLimitContext, setConfigLimitContext] = useState("");
	const [configLimitOutput, setConfigLimitOutput] = useState("");
	const [testing, setTesting] = useState(false);
	const [submitting, setSubmitting] = useState(false);
	const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null);

	const isEdit = Boolean(model);

	useEffect(() => {
		if (!open) return;
		if (model) {
			setProvider(model.provider ?? "openai");
			setPurpose(model.purpose ?? "conversation");
			setName(model.name ?? "");
			setModelName(model.model ?? "");
			setBaseUrl(model.baseUrl ?? "");
			setApiKey("");
			setTemperature(model.temperature != null ? String(model.temperature) : "");
			setMaxTokens(model.maxTokens > 0 ? String(model.maxTokens) : "");
			setDescription(model.description ?? "");
			const cfg = model.config;
			setVision(Boolean(cfg?.vision));
			setConfigTopP(typeof cfg?.top_p === "number" ? String(cfg.top_p) : "");
			setConfigFrequencyPenalty(
				typeof cfg?.frequency_penalty === "number" ? String(cfg.frequency_penalty) : "",
			);
			setConfigPresencePenalty(
				typeof cfg?.presence_penalty === "number" ? String(cfg.presence_penalty) : "",
			);
			const limit = (cfg?.limit ?? {}) as Record<string, unknown>;
			setConfigLimitContext(typeof limit.context === "number" ? String(limit.context) : "");
			setConfigLimitOutput(typeof limit.output === "number" ? String(limit.output) : "");
		} else {
			setProvider("openai");
			setPurpose("conversation");
			setName("");
			setModelName("");
			setBaseUrl("");
			setApiKey("");
			setTemperature("");
			setMaxTokens("");
			setDescription("");
			setVision(false);
			setConfigTopP("");
			setConfigFrequencyPenalty("");
			setConfigPresencePenalty("");
			setConfigLimitContext("");
			setConfigLimitOutput("");
		}
		setTestResult(null);
	}, [open, model]);

	const formValid = Boolean(
		name.trim() && modelName.trim() && baseUrl.trim() && (isEdit || apiKey.trim()),
	);

	const handleTest = async () => {
		if (testing || submitting) return;
		setTesting(true);
		setTestResult(null);
		try {
			const params: TestModelParams = {
				provider,
				model: modelName.trim(),
				base_url: baseUrl.trim(),
			};
			// 编辑且未改 API Key 时，按 id 让后端读取已保存的密钥测试；
			// 否则用当前表单值（含新建场景）
			if (isEdit && model && !apiKey.trim()) {
				params.id = model.id;
			} else {
				params.api_key = apiKey.trim();
			}
			const res = await testModel(params);
			const data = res.data.data;
			if (data?.success) {
				setTestResult({ success: true, message: data.message || "连接成功" });
				toast.success(data.message || "连接成功");
			} else {
				const message = data?.message || "连接测试未通过";
				setTestResult({ success: false, message });
				toast.error(message);
			}
		} catch (err) {
			const message = err instanceof Error ? err.message : "连接测试失败";
			setTestResult({ success: false, message });
			toast.error(message);
		} finally {
			setTesting(false);
		}
	};

	const buildConfig = (): Record<string, unknown> => {
		const cfg: Record<string, unknown> = {};
		if (vision) cfg.vision = true;
		if (configTopP !== "") {
			const v = Number(configTopP);
			if (!Number.isNaN(v) && v >= 0 && v <= 1) cfg.top_p = v;
		}
		if (configFrequencyPenalty !== "") {
			const v = Number(configFrequencyPenalty);
			if (!Number.isNaN(v) && v >= 0 && v <= 2) cfg.frequency_penalty = v;
		}
		if (configPresencePenalty !== "") {
			const v = Number(configPresencePenalty);
			if (!Number.isNaN(v) && v >= 0 && v <= 2) cfg.presence_penalty = v;
		}
		const limit: Record<string, number> = {};
		if (configLimitContext !== "") {
			const v = Number(configLimitContext);
			if (!Number.isNaN(v) && v > 0) limit.context = Math.round(v);
		}
		if (configLimitOutput !== "") {
			const v = Number(configLimitOutput);
			if (!Number.isNaN(v) && v > 0) limit.output = Math.round(v);
		}
		if (Object.keys(limit).length > 0) cfg.limit = limit;
		return cfg;
	};

	const handleSave = async () => {
		if (submitting) return;
		if (!name.trim()) {
			toast.error("请填写名称");
			return;
		}
		if (!purpose.trim()) {
			toast.error("请选择用途");
			return;
		}
		if (!modelName.trim() || !baseUrl.trim() || (!isEdit && !apiKey.trim())) {
			toast.error("请填写模型名、地址和 API Key");
			return;
		}
		setSubmitting(true);
		try {
			if (isEdit && model) {
				const params: UpdateModelParams = {
					id: model.id,
					name: name.trim(),
					provider,
					purpose,
					model: modelName.trim(),
					base_url: baseUrl.trim(),
				};
				if (apiKey.trim()) params.api_key = apiKey.trim();
				if (temperature !== "") {
					const t = Number(temperature);
					if (!Number.isNaN(t) && t >= 0 && t <= 2) params.temperature = t;
				}
				if (maxTokens !== "") {
					const m = Number(maxTokens);
					if (!Number.isNaN(m) && m > 0) params.max_tokens = Math.round(m);
				}
				if (description.trim()) params.description = description.trim();
				const cfg = buildConfig();
				if (Object.keys(cfg).length > 0) params.config = cfg;
				await updateModel(params);
				toast.success("模型已更新");
			} else {
				const params: CreateModelParams = {
					provider,
					purpose,
					model: modelName.trim(),
					base_url: baseUrl.trim(),
					api_key: apiKey.trim(),
				};
				if (description.trim()) params.description = description.trim();
				if (temperature !== "") {
					const t = Number(temperature);
					if (!Number.isNaN(t) && t >= 0 && t <= 2) params.temperature = t;
				}
				if (maxTokens !== "") {
					const m = Number(maxTokens);
					if (!Number.isNaN(m) && m > 0) params.max_tokens = Math.round(m);
				}
				const cfg = buildConfig();
				if (Object.keys(cfg).length > 0) params.config = cfg;
				await createModel(params);
				toast.success("模型已创建");
			}
			onOpenChange(false);
		} catch (err) {
			toast.error(err instanceof Error ? err.message : "保存失败，请稍后重试");
		} finally {
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			open={open}
			onOpenChange={(nextOpen) => {
				if (!nextOpen && !submitting && !testing) onOpenChange(false);
			}}
		>
			<DialogContent className="max-h-[min(92dvh,820px)] max-w-[min(94vw,640px)] gap-6 overflow-y-auto sm:rounded-2xl">
				<DialogHeader className="mb-6 border-b border-slate-200 pb-4">
					<DialogTitle>{isEdit ? "编辑模型" : "新建模型"}</DialogTitle>
					<DialogDescription>配置模型服务提供方与连接信息</DialogDescription>
				</DialogHeader>
				<div className="grid gap-4">
					<div className="grid gap-2">
						<Label>
							名称 <span className="text-red-500">*</span>
						</Label>
						<Input
							value={name}
							onChange={(e) => setName(e.target.value)}
							placeholder="例如：主力对话模型"
						/>
					</div>
					<div className="grid gap-2">
						<Label>
							供应商 <span className="text-red-500">*</span>
						</Label>
						<Select value={provider} onValueChange={(v) => setProvider(v ?? "openai")}>
							<SelectTrigger className="w-full">
								<SelectValue placeholder="选择供应商">
									{PROVIDER_OPTIONS.find((opt) => opt.value === provider)?.label ?? provider}
								</SelectValue>
							</SelectTrigger>
							<SelectContent>
								{PROVIDER_OPTIONS.map((option) => (
									<SelectItem key={option.value} value={option.value}>
										{option.label}
									</SelectItem>
								))}
							</SelectContent>
						</Select>
					</div>
					<div className="grid gap-2">
						<Label>
							用途 <span className="text-red-500">*</span>
						</Label>
						<Select value={purpose} onValueChange={(v) => setPurpose(v ?? "conversation")}>
							<SelectTrigger className="w-full">
								<SelectValue placeholder="选择用途">
									{PURPOSE_OPTIONS.find((opt) => opt.value === purpose)?.label ?? purpose}
								</SelectValue>
							</SelectTrigger>
							<SelectContent>
								{PURPOSE_OPTIONS.map((option) => (
									<SelectItem key={option.value} value={option.value}>
										{option.label}
									</SelectItem>
								))}
							</SelectContent>
						</Select>
					</div>
					<div className="grid gap-2">
						<Label>
							Model <span className="text-red-500">*</span>
						</Label>
						<Input
							value={modelName}
							onChange={(e) => setModelName(e.target.value)}
							placeholder="例如：gpt-4o"
						/>
					</div>
					<div className="grid gap-2">
						<Label>
							Base URL <span className="text-red-500">*</span>
						</Label>
						<Input value={baseUrl} onChange={(e) => setBaseUrl(e.target.value)} placeholder="" />
					</div>
					<div className="grid gap-2">
						<Label>
							API Key <span className="text-red-500">*</span>
						</Label>
						<Input
							type="password"
							value={apiKey}
							onChange={(e) => setApiKey(e.target.value)}
							placeholder={isEdit ? "留空表示不修改" : ""}
						/>
						{isEdit && !apiKey.trim() ? (
							<span className="text-xs text-slate-400">留空时测试连接将使用已保存的密钥</span>
						) : null}
					</div>
					<div className="grid grid-cols-2 gap-4">
						<div className="grid gap-2">
							<Label>温度 (Temperature, 0-2)</Label>
							<Input
								type="number"
								min="0"
								max="2"
								step="0.1"
								value={temperature}
								onChange={(e) => setTemperature(e.target.value)}
								placeholder="1"
							/>
						</div>
						<div className="grid gap-2">
							<Label>最大词元 (Max Tokens)</Label>
							<Input
								type="number"
								min="1"
								value={maxTokens}
								onChange={(e) => setMaxTokens(e.target.value)}
								placeholder="例如：4096"
							/>
						</div>
					</div>
					<div className="grid gap-3 rounded-lg border border-slate-200 bg-slate-50 p-4">
						<div className="text-sm font-medium text-slate-700">扩展配置（高级参数）</div>
						<div className="flex items-center justify-between rounded-md border border-slate-200 bg-white px-3 py-2">
							<div className="flex flex-col">
								<Label>支持图片输入 (Vision)</Label>
								<span className="text-xs text-slate-400">模型是否为多模态，支持图片输入</span>
							</div>
							<Switch checked={vision} onCheckedChange={(v) => setVision(Boolean(v))} />
						</div>
						<div className="grid grid-cols-3 gap-3">
							<div className="grid gap-2">
								<Label>随机采样 (Top P, 0-1)</Label>
								<Input
									type="number"
									min="0"
									max="1"
									step="0.1"
									value={configTopP}
									onChange={(e) => setConfigTopP(e.target.value)}
									placeholder="0.9"
								/>
								<span className="text-xs text-slate-400">按概率分布采样，值越大输出越多样</span>
							</div>
							<div className="grid gap-2">
								<Label>重复控制 (Frequency Penalty, 0-2)</Label>
								<Input
									type="number"
									min="0"
									max="2"
									step="0.1"
									value={configFrequencyPenalty}
									onChange={(e) => setConfigFrequencyPenalty(e.target.value)}
									placeholder="0"
								/>
								<span className="text-xs text-slate-400">值越大，输出越不重复</span>
							</div>
							<div className="grid gap-2">
								<Label>话题探索 (Presence Penalty, 0-2)</Label>
								<Input
									type="number"
									min="0"
									max="2"
									step="0.1"
									value={configPresencePenalty}
									onChange={(e) => setConfigPresencePenalty(e.target.value)}
									placeholder="0"
								/>
								<span className="text-xs text-slate-400">值越大，越倾向谈论新内容</span>
							</div>
						</div>
						<div className="grid grid-cols-2 gap-3">
							<div className="grid gap-2">
								<Label>上下文窗口 (Context Window)</Label>
								<Input
									type="number"
									min="1"
									value={configLimitContext}
									onChange={(e) => setConfigLimitContext(e.target.value)}
									placeholder="例如：128000"
								/>
								<span className="text-xs text-slate-400">模型能接收的最大输入上下文长度</span>
							</div>
							<div className="grid gap-2">
								<Label>最大输出词元 (Max Output Tokens)</Label>
								<Input
									type="number"
									min="1"
									value={configLimitOutput}
									onChange={(e) => setConfigLimitOutput(e.target.value)}
									placeholder="例如：8192"
								/>
								<span className="text-xs text-slate-400">单次生成结果的最大长度</span>
							</div>
						</div>
					</div>
					<div className="grid gap-2">
						<Label>描述</Label>
						<textarea
							value={description}
							onChange={(e) => setDescription(e.target.value)}
							placeholder="模型的用途说明（可选）"
							rows={2}
							className="w-full resize-none rounded-md border border-slate-200 bg-white px-3 py-2 text-sm text-slate-800 focus:border-[#4f46e5] focus:outline-none"
						/>
					</div>
					{testResult ? (
						<p
							className={`flex items-center gap-2 rounded-lg px-3 py-2 text-sm ${
								testResult.success ? "bg-emerald-50 text-emerald-700" : "bg-red-50 text-red-700"
							}`}
							role="alert"
						>
							{testResult.success ? <CheckCircle2 className="size-4" /> : null}
							{testResult.message}
						</p>
					) : null}
				</div>
				<DialogFooter className="border-t border-slate-200 pt-4">
					<Button
						type="button"
						variant="outline"
						onClick={() => void handleTest()}
						disabled={
							testing ||
							submitting ||
							!modelName.trim() ||
							!baseUrl.trim() ||
							(!isEdit && !apiKey.trim())
						}
					>
						{testing ? <Loader2 className="animate-spin" /> : <Wifi />}
						{testing ? "测试中" : "测试连接"}
					</Button>
					<Button type="button" onClick={handleSave} disabled={submitting || testing || !formValid}>
						{submitting ? <Loader2 className="animate-spin" /> : null}
						{isEdit ? "保存" : "创建"}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
