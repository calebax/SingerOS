"use client";

import type { DigitalAssistantItem } from "@leros/store";
import { Badge } from "@leros/ui/components/ui/badge";
import { Button } from "@leros/ui/components/ui/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuTrigger,
} from "@leros/ui/components/ui/dropdown-menu";
import { cn } from "@leros/ui/lib/utils";
import { MoreHorizontal, Pencil, Trash2 } from "lucide-react";
import { AssistantAvatar } from "./AssistantAvatar";
import {
	getAssistantDisplayStatus,
	getAssistantEditability,
	isAssistantAvailable,
} from "./assistantStatus";

export type AssistantCardProps = {
	assistant: DigitalAssistantItem;
	onSelect: (assistant: DigitalAssistantItem) => void;
	onSummon: (assistant: DigitalAssistantItem) => void;
	onEdit: (assistant: DigitalAssistantItem) => void;
	onDelete: (assistant: DigitalAssistantItem) => void;
};

export function AssistantCard({
	assistant,
	onSelect,
	onSummon,
	onEdit,
	onDelete,
}: AssistantCardProps) {
	const statusInfo = getAssistantDisplayStatus(assistant);
	const available = isAssistantAvailable(assistant);
	const editability = getAssistantEditability(assistant);

	return (
		<div
			data-slot="assistant-card"
			className={cn(
				"group relative flex min-h-40 w-full flex-col rounded-2xl border p-4 text-left transition-colors",
				"border-slate-200 bg-white hover:border-[#4f46e5] hover:bg-[#4f46e5]/10",
			)}
		>
			<button
				type="button"
				className="flex min-w-0 flex-1 cursor-pointer flex-col text-left outline-none"
				onClick={() => onSelect(assistant)}
			>
				<div className="flex w-full min-w-0 items-start gap-3 pr-14">
					<AssistantAvatar name={assistant.name} src={assistant.avatar} size="lg" />
					<div className="min-w-0 flex-1">
						<div className="flex min-w-0 items-baseline gap-2">
							<span className="truncate text-base font-semibold text-slate-900">
								{assistant.name}
							</span>
							{assistant.roleName ? (
								<span className="truncate text-sm text-slate-500">{assistant.roleName}</span>
							) : null}
						</div>
						<Badge
							variant="outline"
							className={cn("mt-2 shrink-0 text-xs", statusInfo.className)}
							title={statusInfo.title}
						>
							{statusInfo.label}
						</Badge>
					</div>
				</div>
				{/* 中文注释：简介固定占用两行高度，保证内容多少不影响底部信息位置。 */}
				<span className="mt-3 h-10 w-full text-sm leading-5 text-slate-500 line-clamp-2">
					{assistant.description || "暂无描述"}
				</span>
				<span className="mt-auto w-full pt-3 text-xs text-slate-400">
					创建于 {new Date(assistant.createdAt).toLocaleDateString("zh-CN")}
				</span>
			</button>

			<Button
				size="sm"
				className="absolute right-4 top-4"
				onClick={() => onSummon(assistant)}
				disabled={!available}
				title={available ? `召唤 ${assistant.name}` : statusInfo.label}
			>
				召唤
			</Button>
			<div className="absolute bottom-3 right-4">
				<DropdownMenu>
					<DropdownMenuTrigger
						render={
							<Button
								variant="ghost"
								size="icon-xs"
								className="text-slate-400 hover:text-slate-600 shrink-0"
							>
								<MoreHorizontal className="size-3.5" />
							</Button>
						}
					/>
					<DropdownMenuContent align="end" sideOffset={4}>
						<DropdownMenuItem
							disabled={!editability.canEdit}
							title={editability.reason}
							onClick={() => {
								if (editability.canEdit) onEdit(assistant);
							}}
						>
							<Pencil className="size-3.5 mr-2" />
							编辑
						</DropdownMenuItem>
						<DropdownMenuItem
							variant="destructive"
							onClick={() => {
								onDelete(assistant);
							}}
						>
							<Trash2 className="size-3.5 mr-2" />
							删除
						</DropdownMenuItem>
					</DropdownMenuContent>
				</DropdownMenu>
			</div>
		</div>
	);
}
