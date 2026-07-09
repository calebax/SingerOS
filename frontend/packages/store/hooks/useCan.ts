import { useEffect } from "react";
import { useAppStore } from "../appStore";
import type { Action, BatchCheckItem, ResourceRef } from "../permission/types";

export function useCan(action: Action, resource: ResourceRef | null | undefined) {
	const allowed = useAppStore((state) => state.can(action, resource));
	const ensureCapabilities = useAppStore((state) => state.ensureCapabilities);

	useEffect(() => {
		if (!resource?.publicId) return;
		void ensureCapabilities([{ action, resource }]);
	}, [action, ensureCapabilities, resource?.publicId, resource?.type]);

	return {
		allowed: allowed === true,
		loading: allowed === "unknown",
		denied: allowed === false,
	};
}

export function useEnsureCapabilities(items: BatchCheckItem[], enabled = true) {
	const ensureCapabilities = useAppStore((state) => state.ensureCapabilities);

	useEffect(() => {
		if (!enabled || items.length === 0) return;
		void ensureCapabilities(items);
	}, [enabled, ensureCapabilities, items]);
}

export function useProjectCapabilities(projectPublicId: string | null | undefined) {
	const ensureCapabilities = useAppStore((state) => state.ensureCapabilities);

	useEffect(() => {
		if (!projectPublicId) return;
		void ensureCapabilities(
			(
				[
					"project:update",
					"project:delete",
					"project:member.create",
					"project:member.update",
					"project:member.delete",
					"project:member.leave",
					"task:create",
				] as const
			).map((action) => ({
				action,
				resource: { type: "project" as const, publicId: projectPublicId },
			})),
		);
	}, [ensureCapabilities, projectPublicId]);
}

export function useTaskCapabilities(taskPublicId: string | null | undefined) {
	const ensureCapabilities = useAppStore((state) => state.ensureCapabilities);

	useEffect(() => {
		if (!taskPublicId) return;
		void ensureCapabilities(
			(["task:view", "task:update", "task:delete"] as const).map((action) => ({
				action,
				resource: { type: "task" as const, publicId: taskPublicId },
			})),
		);
	}, [ensureCapabilities, taskPublicId]);
}

/** 项目「更多操作」菜单：预取权限并汇总是否有任一可执行项。 */
export function useProjectMenuCapabilities(projectPublicId: string | null | undefined) {
	useProjectCapabilities(projectPublicId);
	const resource = projectPublicId ? { type: "project" as const, publicId: projectPublicId } : null;

	const rename = useCan("project:update", resource);
	const del = useCan("project:delete", resource);
	const leave = useCan("project:member.leave", resource);

	const loading = rename.loading || del.loading || leave.loading;
	const hasAny = rename.allowed || del.allowed || leave.allowed;

	return {
		loading,
		hasAny,
		canRename: rename.allowed,
		canDelete: del.allowed,
		canLeave: leave.allowed,
	};
}
