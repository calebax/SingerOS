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
import { getAssistantDisplayStatus, isAssistantAvailable } from "./assistantStatus";

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

	return (
		<div
			data-slot="assistant-card"
			className={cn(
				"group relative flex min-h-44 gap-4 rounded-2xl border p-5 transition-colors w-full text-left",
				"border-slate-200 bg-white hover:border-blue-200 hover:bg-blue-50/30",
			)}
		>
			<button
				type="button"
				className="flex min-w-0 flex-1 cursor-pointer items-start gap-4 text-left outline-none"
				onClick={() => onSelect(assistant)}
			>
				<AssistantAvatar name={assistant.name} src={assistant.avatar} size="lg" />

				<div className="flex flex-1 flex-col gap-1 min-w-0">
					<div className="flex flex-wrap items-center gap-2 pr-20">
						<span className="text-base font-semibold text-slate-900 truncate">
							{assistant.name}
						</span>
						{assistant.roleName ? (
							<span className="text-sm text-slate-500">{assistant.roleName}</span>
						) : null}
						<Badge
							variant="outline"
							className={cn("text-xs shrink-0", statusInfo.className)}
							title={statusInfo.title}
						>
							{statusInfo.label}
						</Badge>
					</div>
					<span className="mt-2 text-sm leading-6 text-slate-500 line-clamp-2">
						{assistant.description || "暂无描述"}
					</span>
					<div className="mt-auto flex items-center gap-2 pt-5">
						<span className="text-xs text-slate-400">
							创建于 {new Date(assistant.createdAt).toLocaleDateString("zh-CN")}
						</span>
					</div>
				</div>
			</button>

			<div className="absolute right-4 top-4 flex items-center gap-1.5">
				<Button
					size="sm"
					onClick={() => onSummon(assistant)}
					disabled={!available}
					title={available ? `召唤 ${assistant.name}` : statusInfo.label}
				>
					召唤
				</Button>
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
							onClick={() => {
								onEdit(assistant);
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
