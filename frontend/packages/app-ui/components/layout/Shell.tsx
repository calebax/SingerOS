"use client";

import { useAuthStore, useChatStore, useLayoutStore, usePermissionStore } from "@leros/store";
import { type ReactNode, useEffect } from "react";
import { AuthProvider } from "../auth";
import { AssistantListView } from "../digitalAssistant/AssistantListView";
import { PermissionDeniedListener } from "../permission/PermissionDeniedListener";
import { CenterCanvas } from "./CenterCanvas";
import { FilePreviewHost } from "./FilePreviewHost";
import { type AppNavigation, LeftRail } from "./LeftRail";
import { ProjectPage } from "./ProjectPage";
import { TaskDetailPage } from "./TaskDetailPage";
import { WorkbenchPanel } from "./WorkbenchPanel";

export function Shell({
	logoSrc,
	navigation,
	children,
}: {
	logoSrc?: string;
	navigation?: AppNavigation;
	children?: ReactNode;
}) {
	const currentView = useLayoutStore((s) => s.currentView);
	const { startGlobalEvents, stopGlobalEvents } = useChatStore((s) => s);
	const orgId = useAuthStore((s) => s.authUser?.currentOrg?.id);
	const invalidateAll = usePermissionStore((s) => s.invalidateAll);

	useEffect(() => {
		invalidateAll();
	}, [invalidateAll, orgId]);

	useEffect(() => {
		void startGlobalEvents();
		return () => {
			stopGlobalEvents();
		};
	}, [startGlobalEvents, stopGlobalEvents]);

	return (
		<AuthProvider logoSrc={logoSrc}>
			<PermissionDeniedListener />
			<div className="leros-app-shell">
				<LeftRail logoSrc={logoSrc} navigation={navigation} />
				{children ?? (
					<>
						{currentView === "chat" && <CenterCanvas />}
						{currentView === "workbench" && <WorkbenchPanel />}
						{currentView === "tasks" && <EmptyPage />}
						{currentView === "project" && <ProjectPage />}
						{currentView === "taskDetail" && <TaskDetailPage />}
						{currentView === "digitalAssistant" && <AssistantListView />}
					</>
				)}
			</div>
			<FilePreviewHost />
		</AuthProvider>
	);
}

function EmptyPage() {
	return <div data-slot="empty-page" className="min-h-0 flex-1 bg-[#f7f8fd]" />;
}
