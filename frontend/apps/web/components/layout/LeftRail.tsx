"use client";

import type { NavItem, ViewMode } from "@leros/store";
import { useLayoutStore } from "@leros/store";
import { Button } from "@leros/ui/components/ui/button";
import { ScrollArea } from "@leros/ui/components/ui/scroll-area";
import { cn } from "@leros/ui/lib/utils";
import {
	BookOpen,
	Bot,
	Calendar,
	ChevronDown,
	ChevronLeft,
	ChevronRight,
	Code2,
	GitBranch,
	Hammer,
	MessageSquare,
	Network,
	Paintbrush,
	Settings,
	Star,
	Terminal,
	Users,
} from "lucide-react";

const iconMap: Record<string, React.ReactNode> = {
	IconRobot: <Bot className="size-4" />,
	IconCommand: <Terminal className="size-4" />,
	IconUsers: <Users className="size-4" />,
	IconBook: <BookOpen className="size-4" />,
	IconStar: <Star className="size-4" />,
	IconGitBranch: <GitBranch className="size-4" />,
	IconCode: <Code2 className="size-4" />,
	IconHammer: <Hammer className="size-4" />,
	IconPaint: <Paintbrush className="size-4" />,
	IconNetwork: <Network className="size-4" />,
	IconReport: <Calendar className="size-4" />,
	IconCalendar: <Calendar className="size-4" />,
	IconSettings: <Settings className="size-4" />,
	IconSettings2: <Settings className="size-4" />,
	IconMessage: <MessageSquare className="size-4" />,
};

const navIdToView: Record<string, ViewMode> = {
	"ai-assistant": "chat",
	workbench: "workbench",
	"ai-employee": "digitalAssistant",
	knowledge: "knowledge",
	skills: "skills",
	settings: "settings",
};

export function LeftRail() {
	const {
		leftRailCollapsed,
		navGroups,
		collapsedNavGroups,
		currentView,
		toggleLeftRail,
		toggleNavGroup,
		switchView,
	} = useLayoutStore((s) => s);

	const handleNavClick = (item: NavItem) => {
		const view = navIdToView[item.id] ?? "chat";
		switchView(view);
	};

	const isItemActive = (item: NavItem) => {
		const view = navIdToView[item.id] ?? "chat";
		return currentView === view;
	};

	return (
		<div
			className={cn(
				"flex h-full flex-col border-r border-slate-200/50 bg-white/95 transition-all duration-300",
				leftRailCollapsed ? "w-[56px]" : "w-[244px]",
			)}
		>
			<ScrollArea className="flex-1">
				<div className="p-2">
					{navGroups.map((group) => {
						const isCollapsed = collapsedNavGroups.has(group.id);

						if (leftRailCollapsed) {
							return (
								<div key={group.id} className="mb-2 space-y-1">
									{group.items.map((item: NavItem) => (
										<CollapsedNavItemButton
											key={item.id}
											item={item}
											active={isItemActive(item)}
											onClick={() => handleNavClick(item)}
										/>
									))}
								</div>
							);
						}

						return (
							<div key={group.id} className="mb-4 last:mb-0">
								{group.label && (
									<button
										type="button"
										onClick={() => toggleNavGroup(group.id)}
										className="mb-1 flex w-full items-center gap-1 rounded-lg px-2 py-1.5 text-[11px] font-medium uppercase tracking-wide text-slate-400 transition-colors hover:bg-slate-50 hover:text-slate-500"
									>
										{isCollapsed ? (
											<ChevronRight className="size-3.5" />
										) : (
											<ChevronDown className="size-3.5" />
										)}
										<span className="tracking-wide uppercase">{group.label}</span>
									</button>
								)}

								{!isCollapsed && (
									<div className="space-y-1">
										{group.items.map((item: NavItem) => (
											<NavItemButton
												key={item.id}
												item={item}
												active={isItemActive(item)}
												onClick={() => handleNavClick(item)}
											/>
										))}
									</div>
								)}
							</div>
						);
					})}
				</div>
			</ScrollArea>

			<div className="border-t border-slate-200/50 p-2">
				<Button
					variant="ghost"
					size="sm"
					className={cn(
						"w-full justify-start rounded-xl text-slate-500 hover:bg-slate-50 hover:text-slate-700",
						leftRailCollapsed && "justify-center",
					)}
					onClick={toggleLeftRail}
				>
					{leftRailCollapsed ? (
						<ChevronRight className="size-4" />
					) : (
						<>
							<ChevronLeft className="size-4 mr-1.5" />
							收起侧栏
						</>
					)}
				</Button>
			</div>
		</div>
	);
}

function NavItemButton({
	item,
	active,
	onClick,
}: {
	item: NavItem;
	active: boolean;
	onClick: () => void;
}) {
	const icon = iconMap[item.icon];
	return (
		<button
			type="button"
			onClick={onClick}
			className={cn(
				"group relative flex items-center gap-2.5 rounded-xl px-2.5 py-2.5 text-sm cursor-pointer transition-all w-full text-left",
				active
					? "bg-slate-100 text-slate-900"
					: "text-slate-600 hover:bg-slate-50 hover:text-slate-800",
			)}
		>
			<span
				className={cn(
					"flex size-7 items-center justify-center rounded-lg transition-colors",
					active ? "bg-white text-blue-600 shadow-sm" : "text-slate-400 group-hover:text-slate-600",
				)}
			>
				{icon}
			</span>
			<span className="truncate font-medium">{item.label}</span>
			{item.badge && (
				<span className="ml-auto rounded-full bg-red-100 text-red-600 px-1.5 py-0.5 text-xs">
					{item.badge}
				</span>
			)}
		</button>
	);
}

function CollapsedNavItemButton({
	item,
	active,
	onClick,
}: {
	item: NavItem;
	active: boolean;
	onClick: () => void;
}) {
	const icon = iconMap[item.icon];
	return (
		<button
			type="button"
			onClick={onClick}
			className={cn(
				"flex items-center justify-center rounded-xl p-2.5 transition-all w-full cursor-pointer",
				active
					? "bg-slate-100 text-blue-600"
					: "text-slate-500 hover:bg-slate-50 hover:text-slate-700",
			)}
			title={item.label}
		>
			{icon}
		</button>
	);
}
