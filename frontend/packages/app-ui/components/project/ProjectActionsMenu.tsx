"use client";

import { Action, type Project, useProjectCapabilities } from "@leros/store";
import { DropdownMenuItem } from "@leros/ui/components/ui/dropdown-menu";
import { LogOut, Pencil, Trash2 } from "lucide-react";
import { CanGate } from "../permission/CanGate";

type ProjectActionsMenuProps = {
	project: Project;
	onRename: (project: Project) => void;
	onDelete: (project: Project) => void;
	onLeave: (project: Project) => void;
};

/** 项目更多操作菜单项（重命名 / 删除 / 离开），按权限显隐。 */
export function ProjectActionsMenu({
	project,
	onRename,
	onDelete,
	onLeave,
}: ProjectActionsMenuProps) {
	useProjectCapabilities(project.id);
	const resource = { type: "project" as const, publicId: project.id };

	return (
		<>
			<CanGate action={Action.ProjectUpdate} resource={resource}>
				<DropdownMenuItem onClick={() => onRename(project)}>
					<Pencil className="size-3.5" />
					<span>重命名</span>
				</DropdownMenuItem>
			</CanGate>
			<CanGate action={Action.ProjectDelete} resource={resource}>
				<DropdownMenuItem variant="destructive" onClick={() => onDelete(project)}>
					<Trash2 className="size-3.5" />
					<span>删除</span>
				</DropdownMenuItem>
			</CanGate>
			<CanGate action={Action.ProjectMemberLeave} resource={resource}>
				<DropdownMenuItem onClick={() => onLeave(project)}>
					<LogOut className="size-3.5" />
					<span>离开项目</span>
				</DropdownMenuItem>
			</CanGate>
		</>
	);
}
