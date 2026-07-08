"use client";

import {
	type DigitalAssistantItem,
	type HumanProjectMemberOption,
	type ProjectMember,
	projectMemberApi,
	useDAStore,
} from "@leros/store";
import { Button } from "@leros/ui/components/ui/button";
import {
	Command,
	CommandEmpty,
	CommandGroup,
	CommandInput,
	CommandItem,
	CommandList,
} from "@leros/ui/components/ui/command";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@leros/ui/components/ui/dialog";
import { cn } from "@leros/ui/lib/utils";
import { Bot, Check, LoaderCircle, UserRound, X } from "lucide-react";
import { type ReactNode, useEffect, useMemo, useRef, useState } from "react";
import { useAuth } from "../auth";
import { ProtectedImage } from "../avatar/ProtectedImage";
import { renderHighlightedText } from "../common/searchText";

type MemberTab = "assistant" | "human";

type ProjectMemberPickerDialogProps = {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	selectedMembers: ProjectMember[];
	onConfirm: (members: ProjectMember[]) => void;
};

function isSummonableAssistant(assistant: DigitalAssistantItem): boolean {
	if (assistant.status !== "active") return false;
	const deploymentStatus = assistant.deploymentStatus?.trim();
	return !deploymentStatus || deploymentStatus === "ready";
}

function assistantToProjectMember(assistant: DigitalAssistantItem): ProjectMember {
	return {
		id: `assistant-${assistant.publicId}`,
		memberId: assistant.id,
		publicId: assistant.publicId,
		type: "assistant",
		role: "member",
		name: assistant.name,
		description:
			assistant.description ||
			(assistant.expertise.length > 0 ? assistant.expertise.join("、") : "AI 队友"),
		avatarUrl: assistant.avatar,
	};
}

function humanToProjectMember(member: HumanProjectMemberOption): ProjectMember {
	// 中文注释：真人成员副标题只展示手机号或邮箱，避免把无业务价值的 github_login 暴露在成员选择列表里。
	const descriptionParts = [member.phone, member.email].filter(Boolean);

	return {
		id: `user-${member.public_id}`,
		memberId: member.id ?? 0,
		publicId: member.public_id,
		type: "user",
		role: "member",
		name: member.name,
		description: descriptionParts.join(" / "),
		avatarUrl: member.avatar_url,
	};
}

function memberKey(member: Pick<ProjectMember, "type" | "memberId" | "publicId">) {
	return `${member.type}-${member.publicId ?? member.memberId}`;
}

function memberMatchesQuery(member: ProjectMember, query: string) {
	if (!query) return true;
	return [member.name, member.description, member.role].join(" ").toLowerCase().includes(query);
}

function isSameProjectMember(
	a: Pick<ProjectMember, "type" | "memberId" | "publicId">,
	b: Pick<ProjectMember, "type" | "memberId" | "publicId">,
) {
	if (a.type !== b.type) return false;
	if (a.publicId && b.publicId) return a.publicId === b.publicId;
	return a.memberId > 0 && b.memberId > 0 && a.memberId === b.memberId;
}

function isAlreadySelectedMember(selectedMembers: ProjectMember[], candidate: ProjectMember) {
	return selectedMembers.some((selected) => {
		if (isSameProjectMember(selected, candidate)) return true;
		// 中文注释：兼容旧项目详情缺少 public_id 的数据，避免已加入成员继续出现在候选列表。
		return (
			!selected.publicId &&
			selected.type === candidate.type &&
			selected.name.trim() !== "" &&
			selected.name === candidate.name
		);
	});
}

function resolveMemberPublicIdentity(
	member: ProjectMember,
	options: ProjectMember[],
): ProjectMember {
	if (member.publicId) return member;
	const matched = options.find(
		(option) =>
			option.type === member.type && option.memberId > 0 && option.memberId === member.memberId,
	);
	if (!matched?.publicId) return member;

	return {
		...member,
		id: matched.id,
		memberId: matched.memberId || member.memberId,
		// 中文注释：项目成员详情可能只带内部 memberId，提交更新前要回填候选列表里的 public_id。
		publicId: matched.publicId,
		description: member.description || matched.description,
		avatarUrl: member.avatarUrl || matched.avatarUrl,
	};
}

export function ProjectMemberPickerDialog({
	open,
	onOpenChange,
	selectedMembers,
	onConfirm,
}: ProjectMemberPickerDialogProps) {
	const { user } = useAuth();
	const { assistants, assistantsLoaded, fetchAssistants } = useDAStore((s) => s);
	const [activeTab, setActiveTab] = useState<MemberTab>("assistant");
	const [draftMembers, setDraftMembers] = useState<ProjectMember[]>([]);
	const [assistantSearch, setAssistantSearch] = useState("");
	const [humanSearch, setHumanSearch] = useState("");
	const [humanOptions, setHumanOptions] = useState<ProjectMember[]>([]);
	const [humansLoading, setHumansLoading] = useState(false);
	const [humansError, setHumansError] = useState<string | null>(null);
	const wasOpenRef = useRef(false);

	useEffect(() => {
		const wasOpen = wasOpenRef.current;
		wasOpenRef.current = open;
		if (!open || wasOpen) return;

		// 中文注释：弹窗草稿只在打开瞬间初始化，避免 AI/成员列表异步刷新把用户刚删除的成员重置回来。
		setDraftMembers(selectedMembers);
		setActiveTab("assistant");
		setAssistantSearch("");
		setHumanSearch("");
		if (!assistantsLoaded) {
			void fetchAssistants();
		}
	}, [assistantsLoaded, fetchAssistants, open, selectedMembers]);

	useEffect(() => {
		if (!open) return;

		const timer = window.setTimeout(() => {
			setHumansLoading(true);
			setHumansError(null);
			projectMemberApi
				.listHumanMembers({ keyword: humanSearch.trim(), limit: 50 })
				.then((items) => {
					setHumanOptions(items.map(humanToProjectMember));
				})
				.catch((error: unknown) => {
					const message = error instanceof Error ? error.message : "成员加载失败";
					setHumansError(message);
					setHumanOptions([]);
				})
				.finally(() => {
					setHumansLoading(false);
				});
		}, 200);

		return () => window.clearTimeout(timer);
	}, [humanSearch, open]);

	const assistantOptions = useMemo(
		() => assistants.filter(isSummonableAssistant).map(assistantToProjectMember),
		[assistants],
	);
	const identityOptions = useMemo(
		() => [...assistantOptions, ...humanOptions],
		[assistantOptions, humanOptions],
	);
	useEffect(() => {
		if (!open || identityOptions.length === 0) return;
		setDraftMembers((current) =>
			current.map((member) => resolveMemberPublicIdentity(member, identityOptions)),
		);
	}, [identityOptions, open]);
	const selectedKeys = useMemo(
		() => new Set(draftMembers.map((member) => memberKey(member))),
		[draftMembers],
	);
	const visibleDraftMembers = useMemo(
		() =>
			draftMembers.filter(
				// 中文注释：默认 AI 员工是系统兜底成员，不在添加成员弹窗的已选择列表里展示。
				(member) => !(member.type === "assistant" && member.isDefault),
			),
		[draftMembers],
	);
	const filteredAssistants = useMemo(() => {
		const query = assistantSearch.trim().toLowerCase();
		return assistantOptions.filter(
			(member) =>
				!selectedKeys.has(memberKey(member)) &&
				!isAlreadySelectedMember(draftMembers, member) &&
				memberMatchesQuery(member, query),
		);
	}, [assistantOptions, assistantSearch, draftMembers, selectedKeys]);
	const filteredHumans = useMemo(() => {
		const query = humanSearch.trim().toLowerCase();
		return humanOptions.filter(
			(member) =>
				member.publicId !== user?.publicId &&
				!selectedKeys.has(memberKey(member)) &&
				!isAlreadySelectedMember(draftMembers, member) &&
				memberMatchesQuery(member, query),
		);
	}, [draftMembers, humanOptions, humanSearch, selectedKeys, user?.publicId]);

	const addMember = (member: ProjectMember) => {
		setDraftMembers((current) => {
			if (
				current.some(
					(item) =>
						memberKey(item) === memberKey(member) || isAlreadySelectedMember([item], member),
				)
			) {
				return current;
			}
			return [...current, member];
		});
	};

	const removeMember = (member: ProjectMember) => {
		if (isProtectedMember(member)) return;
		setDraftMembers((current) => current.filter((item) => memberKey(item) !== memberKey(member)));
	};

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent
				className="flex max-h-[calc(100vh-48px)] flex-col overflow-hidden sm:max-w-[720px]"
				showCloseButton={false}
			>
				<DialogHeader className="shrink-0">
					<div className="flex items-center justify-between gap-4">
						<DialogTitle>添加项目成员</DialogTitle>
						<Button
							type="button"
							variant="ghost"
							size="icon-sm"
							onClick={() => onOpenChange(false)}
							aria-label="关闭"
						>
							<X className="size-4" />
						</Button>
					</div>
					<DialogDescription>选择 AI 队友或真人成员加入项目。</DialogDescription>
				</DialogHeader>

				<div className="mt-4 grid h-[420px] max-h-[calc(100vh-220px)] min-h-0 gap-4 overflow-hidden sm:grid-cols-[minmax(0,1fr)_220px]">
					<div className="flex min-h-0 flex-col">
						<div className="grid h-9 shrink-0 grid-cols-2 rounded-lg bg-[var(--leros-surface-soft)] p-1">
							<MemberTabButton
								active={activeTab === "assistant"}
								onClick={() => setActiveTab("assistant")}
							>
								AI 队友
							</MemberTabButton>
							<MemberTabButton active={activeTab === "human"} onClick={() => setActiveTab("human")}>
								真人成员
							</MemberTabButton>
						</div>
						<div className="mt-3 min-h-0 flex-1">
							{activeTab === "assistant" ? (
								<MemberCommandList
									search={assistantSearch}
									onSearchChange={setAssistantSearch}
									placeholder="搜索 AI 队友"
									emptyText="没有可添加的 AI 队友"
									members={filteredAssistants}
									onSelect={addMember}
									loading={!assistantsLoaded}
								/>
							) : (
								<MemberCommandList
									search={humanSearch}
									onSearchChange={setHumanSearch}
									placeholder="搜索真人成员"
									emptyText="没有可添加的真人成员"
									members={filteredHumans}
									onSelect={addMember}
									loading={humansLoading}
									error={humansError}
								/>
							)}
						</div>
					</div>

					<div className="flex min-h-0 flex-col rounded-xl border border-[var(--leros-control-border)] bg-white p-3">
						<div className="mb-2 text-xs font-medium text-[var(--leros-text-muted)]">
							已选择 {visibleDraftMembers.length} 位
						</div>
						{visibleDraftMembers.length === 0 ? (
							<div className="flex flex-1 items-center justify-center rounded-lg border border-dashed border-[var(--leros-control-border)] px-3 py-6 text-center text-xs text-[var(--leros-text-subtle)]">
								暂未选择成员
							</div>
						) : (
							<div className="min-h-0 flex-1 space-y-2 overflow-y-auto pr-1 no-scrollbar">
								{visibleDraftMembers.map((member) => (
									<ProjectMemberChip
										key={memberKey(member)}
										member={member}
										readonly={isProtectedMember(member)}
										onRemove={() => removeMember(member)}
									/>
								))}
							</div>
						)}
					</div>
				</div>

				<DialogFooter className="mt-5 shrink-0 border-t border-[var(--leros-control-border)] pt-4">
					<Button variant="outline" onClick={() => onOpenChange(false)}>
						取消
					</Button>
					<Button
						onClick={() => {
							onConfirm(
								draftMembers.map((member) => resolveMemberPublicIdentity(member, identityOptions)),
							);
							onOpenChange(false);
						}}
					>
						确认添加
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}

function MemberTabButton({
	active,
	onClick,
	children,
}: {
	active: boolean;
	onClick: () => void;
	children: ReactNode;
}) {
	return (
		<button
			type="button"
			className={cn(
				"rounded-md px-3 text-sm font-medium transition-colors",
				active
					? "bg-white text-[var(--leros-text-strong)] shadow-sm"
					: "text-[var(--leros-text-muted)] hover:text-[var(--leros-text-strong)]",
			)}
			onClick={onClick}
		>
			{children}
		</button>
	);
}

function MemberCommandList({
	search,
	onSearchChange,
	placeholder,
	emptyText,
	members,
	onSelect,
	loading,
	error,
}: {
	search: string;
	onSearchChange: (value: string) => void;
	placeholder: string;
	emptyText: string;
	members: ProjectMember[];
	onSelect: (member: ProjectMember) => void;
	loading?: boolean;
	error?: string | null;
}) {
	return (
		<Command
			shouldFilter={false}
			className="flex h-full min-h-0 flex-col rounded-none bg-transparent p-0"
		>
			<CommandInput value={search} onValueChange={onSearchChange} placeholder={placeholder} />
			<CommandList className="max-h-none min-h-0 flex-1">
				<CommandEmpty className="py-6 text-slate-400">{emptyText}</CommandEmpty>
				<CommandGroup className="p-1">
					{loading && (
						<div className="flex items-center gap-2 px-3 py-2 text-xs text-slate-400">
							<LoaderCircle className="size-3.5 animate-spin" />
							加载中...
						</div>
					)}
					{!loading && error && <div className="px-3 py-2 text-xs text-red-400">{error}</div>}
					{!loading &&
						!error &&
						members.map((member) => (
							<CommandItem
								key={memberKey(member)}
								value={memberKey(member)}
								onSelect={() => onSelect(member)}
								className="rounded-lg px-2.5 py-2 data-selected:bg-[var(--leros-surface-soft)]"
							>
								<MemberAvatar member={member} />
								<div className="min-w-0 flex-1">
									<div className="truncate font-medium text-slate-700">
										{renderHighlightedText(member.name, search)}
									</div>
									<div className="truncate text-xs text-slate-400">
										{renderHighlightedText(member.description ?? "", search)}
									</div>
								</div>
								<Check className="size-4 opacity-0" />
							</CommandItem>
						))}
				</CommandGroup>
			</CommandList>
		</Command>
	);
}

function isProtectedMember(member: ProjectMember) {
	return member.role === "owner" || member.isDefault === true;
}

export function ProjectMemberChip({
	member,
	onRemove,
	readonly = false,
	className,
}: {
	member: ProjectMember;
	onRemove?: () => void;
	readonly?: boolean;
	className?: string;
}) {
	return (
		<div
			className={cn(
				"group inline-flex min-w-0 items-center gap-2 rounded-lg border border-[var(--leros-control-border)] bg-[var(--leros-surface)] py-1.5 pl-1.5 pr-2",
				className,
			)}
		>
			<MemberAvatar member={member} />
			<div className="min-w-0 flex-1">
				<div className="truncate text-xs font-medium text-[var(--leros-text)]">{member.name}</div>
				<div className="text-[10px] text-[var(--leros-text-subtle)]">
					{member.role === "owner"
						? "创建者"
						: member.isDefault
							? "默认 AI 队友"
							: member.type === "assistant"
								? "AI 队友"
								: "真人成员"}
				</div>
			</div>
			{!readonly && onRemove && (
				<button
					type="button"
					className="rounded-full p-0.5 text-[var(--leros-text-subtle)] opacity-0 transition-opacity hover:bg-[var(--leros-control-border)] hover:text-[var(--leros-text)] group-hover:opacity-100"
					aria-label={`移除成员 ${member.name}`}
					onClick={onRemove}
				>
					<X className="size-3" />
				</button>
			)}
		</div>
	);
}

function MemberAvatar({ member }: { member: ProjectMember }) {
	const fallback = (
		<div
			className={cn(
				"flex size-7 shrink-0 items-center justify-center rounded-lg",
				member.type === "assistant" ? "bg-blue-50 text-blue-600" : "bg-emerald-50 text-emerald-600",
			)}
		>
			{member.type === "assistant" ? (
				<Bot className="size-3.5" />
			) : (
				<UserRound className="size-3.5" />
			)}
		</div>
	);

	if (member.avatarUrl) {
		return (
			<ProtectedImage
				src={member.avatarUrl}
				alt={member.name}
				className="size-7 shrink-0 rounded-lg object-cover"
				fallback={fallback}
			/>
		);
	}

	return fallback;
}
