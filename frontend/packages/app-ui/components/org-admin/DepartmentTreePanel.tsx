"use client";

import { type Department, type OrgMember, orgAdminApi, useAuthStore } from "@leros/store";
import { Badge } from "@leros/ui/components/ui/badge";
import { Button } from "@leros/ui/components/ui/button";
import { Checkbox } from "@leros/ui/components/ui/checkbox";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@leros/ui/components/ui/dialog";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuTrigger,
} from "@leros/ui/components/ui/dropdown-menu";
import { Input } from "@leros/ui/components/ui/input";
import { ScrollArea } from "@leros/ui/components/ui/scroll-area";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@leros/ui/components/ui/table";
import { cn } from "@leros/ui/lib/utils";
import {
	ChevronDown,
	ChevronRight,
	Loader2,
	MoreHorizontal,
	Pencil,
	Plus,
	Search,
	X,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import {
	buildDepartmentTree,
	countDepartments,
	type DepartmentTreeNode,
	filterDepartmentTree,
} from "./departmentTree";

type DepartmentDialogMode =
	| { type: "create"; parentId: number; parentName: string }
	| { type: "rename"; department: Department }
	| { type: "delete"; department: Department }
	| null;

type MemberDialogState = { open: false } | { open: true; defaultDepartmentId: number | null };

type EditMemberDialogState = { open: false } | { open: true; member: OrgMember };

function formatMemberCreatedAt(timestamp: string | undefined) {
	if (!timestamp) return "-";
	try {
		return new Date(timestamp).toLocaleString("zh-CN", {
			year: "numeric",
			month: "2-digit",
			day: "2-digit",
			hour: "2-digit",
			minute: "2-digit",
			second: "2-digit",
		});
	} catch {
		return "-";
	}
}

function DepartmentTreeItem({
	node,
	level,
	selectedId,
	onSelect,
	onCreate,
	onRename,
	onDelete,
}: {
	node: DepartmentTreeNode;
	level: number;
	selectedId: number | null;
	onSelect: (id: number) => void;
	onCreate: (parentId: number, parentName: string) => void;
	onRename: (department: Department) => void;
	onDelete: (department: Department) => void;
}) {
	const [expanded, setExpanded] = useState(true);
	const hasChildren = node.children.length > 0;

	return (
		<div>
			<div
				className={cn(
					"group flex items-center gap-1 rounded-lg pr-1",
					selectedId === node.id && "bg-[var(--leros-primary-softer)]",
				)}
				style={{ paddingLeft: `${level * 16 + 8}px` }}
			>
				<button
					type="button"
					className="flex size-6 shrink-0 items-center justify-center rounded-md text-[var(--leros-text-subtle)] hover:bg-slate-100"
					onClick={() => setExpanded((value) => !value)}
					aria-label={expanded ? "收起" : "展开"}
				>
					{hasChildren ? (
						expanded ? (
							<ChevronDown className="size-3.5" />
						) : (
							<ChevronRight className="size-3.5" />
						)
					) : (
						<span className="size-3.5" />
					)}
				</button>
				<button
					type="button"
					className="min-w-0 flex-1 truncate py-2 text-left text-sm text-[var(--leros-text)]"
					onClick={() => onSelect(node.id)}
				>
					{node.name}
				</button>
				<DropdownMenu>
					<DropdownMenuTrigger
						className="rounded-md p-1 opacity-0 transition-opacity hover:bg-slate-100 group-hover:opacity-100"
						aria-label={`管理 ${node.name}`}
					>
						<MoreHorizontal className="size-4 text-[var(--leros-text-subtle)]" />
					</DropdownMenuTrigger>
					<DropdownMenuContent align="end" side="right" sideOffset={4}>
						<DropdownMenuItem onClick={() => onCreate(node.id, node.name)}>
							新建子部门
						</DropdownMenuItem>
						<DropdownMenuItem onClick={() => onRename(node)}>重命名</DropdownMenuItem>
						{node.parent_id !== 0 && (
							<DropdownMenuItem variant="destructive" onClick={() => onDelete(node)}>
								删除
							</DropdownMenuItem>
						)}
					</DropdownMenuContent>
				</DropdownMenu>
			</div>
			{expanded &&
				node.children.map((child) => (
					<DepartmentTreeItem
						key={child.id}
						node={child}
						level={level + 1}
						selectedId={selectedId}
						onSelect={onSelect}
						onCreate={onCreate}
						onRename={onRename}
						onDelete={onDelete}
					/>
				))}
		</div>
	);
}

export function DepartmentTreePanel({ compact = false }: { compact?: boolean }) {
	const user = useAuthStore((s) => s.authUser);
	const orgId = user?.currentOrg?.id;
	const orgName = user?.currentOrg?.name ?? "当前组织";

	const [loading, setLoading] = useState(true);
	const [departments, setDepartments] = useState<Department[]>([]);
	const [members, setMembers] = useState<OrgMember[]>([]);
	const [membersLoading, setMembersLoading] = useState(false);
	const [search, setSearch] = useState("");
	const [selectedId, setSelectedId] = useState<number | null>(null);
	const [dialogMode, setDialogMode] = useState<DepartmentDialogMode>(null);
	const [dialogValue, setDialogValue] = useState("");
	const [submitting, setSubmitting] = useState(false);
	const [memberDialog, setMemberDialog] = useState<MemberDialogState>({ open: false });
	const [editMemberDialog, setEditMemberDialog] = useState<EditMemberDialogState>({ open: false });

	const loadDepartments = useCallback(async () => {
		if (!orgId) {
			setLoading(false);
			return;
		}
		setLoading(true);
		try {
			const resp = await orgAdminApi.listDepartments({ org_id: orgId, list_all: true });
			setDepartments(resp.data.data.items ?? []);
		} catch (err) {
			const message = err instanceof Error ? err.message : "部门加载失败";
			toast.error(message);
		} finally {
			setLoading(false);
		}
	}, [orgId]);

	const loadMembers = useCallback(async () => {
		if (!orgId) return;
		setMembersLoading(true);
		try {
			const resp = await orgAdminApi.listOrgMembers({
				org_id: orgId,
				department_id: selectedId ?? undefined,
				list_all: true,
			});
			setMembers(resp.data.data.items ?? []);
		} catch (err) {
			const message = err instanceof Error ? err.message : "成员加载失败";
			toast.error(message);
		} finally {
			setMembersLoading(false);
		}
	}, [orgId, selectedId]);

	useEffect(() => {
		void loadDepartments();
	}, [loadDepartments]);

	useEffect(() => {
		void loadMembers();
	}, [loadMembers]);

	const tree = useMemo(() => buildDepartmentTree(departments), [departments]);
	const filteredTree = useMemo(() => filterDepartmentTree(tree, search), [tree, search]);
	const departmentCount = useMemo(() => countDepartments(tree), [tree]);
	const selectedDepartment = departments.find((item) => item.id === selectedId) ?? null;
	const isDefaultUser = user?.uin === user?.currentOrg?.createdByUin;

	const openCreateDialog = (parentId: number, parentName: string) => {
		setDialogMode({ type: "create", parentId, parentName });
		setDialogValue("");
	};

	const openRenameDialog = (department: Department) => {
		setDialogMode({ type: "rename", department });
		setDialogValue(department.name);
	};

	const openDeleteDialog = (department: Department) => {
		setDialogMode({ type: "delete", department });
		setDialogValue("");
	};

	const handleCreateMember = async (name: string, phone: string, departmentIds: number[]) => {
		if (!orgId) return;
		setSubmitting(true);
		try {
			await orgAdminApi.createOrgMember({
				name: name.trim(),
				phone: phone.trim(),
				department_ids: departmentIds,
			});
			toast.success("成员已创建");
			setMemberDialog({ open: false });
			await loadMembers();
		} catch (err) {
			const message = err instanceof Error ? err.message : "创建成员失败";
			toast.error(message);
		} finally {
			setSubmitting(false);
		}
	};

	const handleUpdateMember = async (id: number, name: string, departmentIds: number[]) => {
		if (!orgId) return;
		setSubmitting(true);
		try {
			await orgAdminApi.updateOrgMember({
				id,
				name: name.trim(),
				department_ids: departmentIds,
			});
			toast.success("成员已更新");
			setEditMemberDialog({ open: false });
			await loadMembers();
		} catch (err) {
			const message = err instanceof Error ? err.message : "更新成员失败";
			toast.error(message);
		} finally {
			setSubmitting(false);
		}
	};

	const handleDialogConfirm = async () => {
		if (!orgId || !dialogMode) return;
		setSubmitting(true);
		try {
			if (dialogMode.type === "create") {
				const name = dialogValue.trim();
				if (!name) {
					toast.error("部门名称不能为空");
					return;
				}
				await orgAdminApi.createDepartment({
					org_id: orgId,
					name,
					parent_id: dialogMode.parentId,
				});
				toast.success("部门已创建");
			}
			if (dialogMode.type === "rename") {
				const name = dialogValue.trim();
				if (!name) {
					toast.error("部门名称不能为空");
					return;
				}
				await orgAdminApi.updateDepartment({ id: dialogMode.department.id, name });
				toast.success("部门已更新");
			}
			if (dialogMode.type === "delete") {
				await orgAdminApi.deleteDepartment({ id: dialogMode.department.id });
				toast.success("部门已删除");
				if (selectedId === dialogMode.department.id) {
					setSelectedId(null);
				}
			}
			setDialogMode(null);
			await loadDepartments();
		} catch (err) {
			const message = err instanceof Error ? err.message : "操作失败";
			toast.error(message);
		} finally {
			setSubmitting(false);
		}
	};

	if (!user?.currentOrg) {
		return (
			<div className="rounded-2xl border border-[var(--leros-control-border)] bg-[var(--leros-surface,#fff)] p-8 text-sm text-[var(--leros-text-subtle)]">
				请先登录并选择组织后再管理部门。
			</div>
		);
	}

	return (
		<div
			className={cn("flex h-full min-h-0 flex-col", compact ? "min-h-[480px]" : "min-h-[560px]")}
		>
			{!compact && (
				<div className="mb-4 flex shrink-0 items-center justify-between gap-3">
					<h1 className="text-xl font-semibold text-[var(--leros-text-strong)]">通讯录</h1>
				</div>
			)}

			<div className="flex min-h-0 flex-1 overflow-hidden rounded-2xl border border-[var(--leros-control-border)] bg-[var(--leros-surface,#fff)]">
				<aside
					className={cn(
						"flex shrink-0 flex-col border-r border-[var(--leros-control-border)]",
						compact ? "w-[240px]" : "w-[280px]",
					)}
				>
					<div className="border-b border-[var(--leros-control-border)] p-3">
						<div className="relative">
							<Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-[var(--leros-text-subtle)]" />
							<Input
								value={search}
								onChange={(event) => setSearch(event.target.value)}
								placeholder="搜索部门"
								className="pl-9"
							/>
						</div>
					</div>
					<div className="min-h-0 flex-1 overflow-y-auto p-2">
						{loading ? (
							<div className="flex items-center justify-center py-10 text-sm text-[var(--leros-text-subtle)]">
								<Loader2 className="mr-2 size-4 animate-spin" />
								加载中...
							</div>
						) : (
							filteredTree.map((node) => (
								<DepartmentTreeItem
									key={node.id}
									node={node}
									level={0}
									selectedId={selectedId}
									onSelect={setSelectedId}
									onCreate={openCreateDialog}
									onRename={openRenameDialog}
									onDelete={openDeleteDialog}
								/>
							))
						)}
					</div>
				</aside>

				<section className="flex min-h-0 min-w-0 flex-1 flex-col p-4 sm:p-6">
					<div className="mb-4 flex shrink-0 items-center justify-between gap-3 sm:mb-6">
						<div>
							<h2 className="text-lg font-semibold text-[var(--leros-text-strong)]">
								{selectedDepartment?.name ?? "通讯录"}
							</h2>
							<p className="mt-1 text-sm text-[var(--leros-text-subtle)]">
								当前组织共有 {departmentCount} 个部门
							</p>
						</div>
						{isDefaultUser && (
							<Button
								type="button"
								onClick={() => setMemberDialog({ open: true, defaultDepartmentId: selectedId })}
							>
								<Plus className="mr-1 size-4" />
								创建成员
							</Button>
						)}
					</div>

					{membersLoading ? (
						<div className="flex flex-1 items-center justify-center text-sm text-[var(--leros-text-subtle)]">
							<Loader2 className="mr-2 size-4 animate-spin" />
							加载成员中...
						</div>
					) : members.length === 0 ? (
						<div className="flex flex-1 flex-col items-center justify-center rounded-xl border border-dashed border-[var(--leros-control-border)] bg-[var(--leros-surface-soft,#f6f8fc)] px-4 py-10 text-center sm:px-6">
							<p className="text-sm text-[var(--leros-text-muted)]">暂无成员</p>
							{isDefaultUser ? (
								<>
									<p className="mt-2 max-w-sm text-xs leading-relaxed text-[var(--leros-text-subtle)]">
										点击下方按钮创建成员到当前组织
										{selectedDepartment ? `或「${selectedDepartment.name}」部门` : ""}
									</p>
									<Button
										type="button"
										className="mt-6"
										onClick={() => setMemberDialog({ open: true, defaultDepartmentId: selectedId })}
									>
										<Plus className="mr-1 size-4" />
										创建成员
									</Button>
								</>
							) : (
								<p className="mt-2 max-w-sm text-xs leading-relaxed text-[var(--leros-text-subtle)]">
									暂无成员，请联系组织管理员添加
								</p>
							)}
						</div>
					) : (
						<div className="min-h-0 flex-1 overflow-y-auto rounded-xl border border-[var(--leros-control-border)]">
							<Table>
								<TableHeader>
									<TableRow>
										<TableHead>用户名</TableHead>
										<TableHead>所属部门</TableHead>
										<TableHead>手机号</TableHead>
										<TableHead>创建时间</TableHead>
										<TableHead className="text-right">操作</TableHead>
									</TableRow>
								</TableHeader>
								<TableBody>
									{members.map((member) => (
										<TableRow key={member.id}>
											<TableCell className="font-medium">{member.user_name ?? "未命名"}</TableCell>
											<TableCell>
												<div className="flex flex-wrap gap-1">
													{(member.departments ?? []).map((dept) => (
														<Badge
															key={dept.id}
															variant={dept.is_primary ? "default" : "secondary"}
														>
															{dept.name}
														</Badge>
													))}
												</div>
											</TableCell>
											<TableCell>{member.user_phone ?? "-"}</TableCell>
											<TableCell>{formatMemberCreatedAt(member.created_at)}</TableCell>
											<TableCell className="text-right">
												{isDefaultUser && (
													<Button
														type="button"
														variant="ghost"
														size="sm"
														onClick={() => setEditMemberDialog({ open: true, member })}
													>
														<Pencil className="mr-1 size-3.5" />
														编辑
													</Button>
												)}
											</TableCell>
										</TableRow>
									))}
								</TableBody>
							</Table>
						</div>
					)}
				</section>
			</div>

			<Dialog open={dialogMode !== null} onOpenChange={(open) => !open && setDialogMode(null)}>
				<DialogContent className="sm:max-w-md">
					<DialogHeader>
						<DialogTitle>
							{dialogMode?.type === "create"
								? "新建部门"
								: dialogMode?.type === "rename"
									? "重命名部门"
									: "删除部门"}
						</DialogTitle>
						<DialogDescription>
							{dialogMode?.type === "create"
								? `将在「${dialogMode.parentName}」下创建子部门`
								: dialogMode?.type === "rename"
									? "请输入新的部门名称"
									: `确定删除「${dialogMode?.department.name}」吗？若存在子部门将无法删除。`}
						</DialogDescription>
					</DialogHeader>

					{dialogMode?.type !== "delete" ? (
						<Input
							value={dialogValue}
							onChange={(event) => setDialogValue(event.target.value)}
							placeholder="部门名称"
							autoFocus
						/>
					) : null}

					<DialogFooter>
						<Button
							type="button"
							variant="outline"
							onClick={() => setDialogMode(null)}
							disabled={submitting}
						>
							取消
						</Button>
						<Button
							type="button"
							variant={dialogMode?.type === "delete" ? "destructive" : "default"}
							onClick={() => void handleDialogConfirm()}
							disabled={submitting}
						>
							{submitting ? <Loader2 className="size-4 animate-spin" /> : null}
							{dialogMode?.type === "delete" ? "删除" : "确定"}
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			{memberDialog.open && (
				<CreateMemberDialog
					departments={departments}
					defaultDepartmentId={memberDialog.defaultDepartmentId}
					orgName={orgName}
					submitting={submitting}
					onClose={() => setMemberDialog({ open: false })}
					onSubmit={handleCreateMember}
				/>
			)}
			{editMemberDialog.open && (
				<EditMemberDialog
					departments={departments}
					member={editMemberDialog.member}
					submitting={submitting}
					onClose={() => setEditMemberDialog({ open: false })}
					onSubmit={handleUpdateMember}
				/>
			)}
		</div>
	);
}

type CreateMemberDialogProps = {
	departments: Department[];
	defaultDepartmentId: number | null;
	orgName: string;
	submitting: boolean;
	onClose: () => void;
	onSubmit: (name: string, phone: string, departmentIds: number[]) => void;
};

function CreateMemberDialog({
	departments,
	defaultDepartmentId,
	orgName,
	submitting,
	onClose,
	onSubmit,
}: CreateMemberDialogProps) {
	const [name, setName] = useState("");
	const [phone, setPhone] = useState("");
	const [selectedIds, setSelectedIds] = useState<number[]>(
		defaultDepartmentId ? [defaultDepartmentId] : [],
	);
	const [pickerOpen, setPickerOpen] = useState(false);

	const toggleDepartment = (id: number) => {
		setSelectedIds((prev) => {
			if (prev.includes(id)) {
				return prev.filter((item) => item !== id);
			}
			return [...prev, id];
		});
	};

	const departmentById = useMemo(() => {
		const map = new Map<number, Department>();
		for (const department of departments) {
			map.set(department.id, department);
		}
		return map;
	}, [departments]);

	const selectedDepartments = useMemo(() => {
		return selectedIds
			.map((id) => departmentById.get(id))
			.filter((item): item is Department => item !== undefined);
	}, [selectedIds, departmentById]);

	const handleSubmit = () => {
		const trimmedName = name.trim();
		const trimmedPhone = phone.trim();
		if (!trimmedName) {
			toast.error("用户名不能为空");
			return;
		}
		if (!trimmedPhone) {
			toast.error("手机号不能为空");
			return;
		}
		if (selectedIds.length === 0) {
			toast.error("请选择所属部门");
			return;
		}
		onSubmit(trimmedName, trimmedPhone, selectedIds);
	};

	return (
		<Dialog open onOpenChange={(open) => !open && onClose()}>
			<DialogContent className="flex h-[min(640px,85vh)] max-h-[720px] w-[min(520px,95vw)] max-w-none flex-col gap-0 p-0 sm:max-w-none">
				<DialogHeader className="shrink-0 px-6 py-4">
					<DialogTitle>创建成员</DialogTitle>
					<DialogDescription>
						创建成员到「{orgName}」，第一个选择的部门将作为主部门
					</DialogDescription>
				</DialogHeader>

				<div className="flex min-h-0 flex-1 flex-col gap-4 px-6 py-2">
					<div className="space-y-2">
						<label
							htmlFor="create-member-name"
							className="text-sm font-medium text-[var(--leros-text-strong)]"
						>
							用户名
						</label>
						<Input
							id="create-member-name"
							value={name}
							onChange={(event) => setName(event.target.value)}
							placeholder="请输入用户名"
							autoFocus
						/>
					</div>

					<div className="space-y-2">
						<label
							htmlFor="create-member-phone"
							className="text-sm font-medium text-[var(--leros-text-strong)]"
						>
							手机号
						</label>
						<Input
							id="create-member-phone"
							value={phone}
							onChange={(event) => setPhone(event.target.value)}
							placeholder="请输入手机号"
						/>
					</div>

					<div className="flex min-h-0 flex-1 flex-col gap-2">
						<div className="flex items-center justify-between">
							<span className="text-sm font-medium text-[var(--leros-text-strong)]">所属部门</span>
							<span className="text-xs text-[var(--leros-text-subtle)]">
								已选 {selectedIds.length} 个
							</span>
						</div>
						<div className="flex min-h-0 flex-1 flex-col gap-2 rounded-xl border border-[var(--leros-control-border)] p-3">
							{selectedDepartments.length === 0 ? (
								<p className="text-sm text-[var(--leros-text-subtle)]">暂未选择部门</p>
							) : (
								<ScrollArea className="flex-1">
									<div className="flex flex-wrap gap-2">
										{selectedDepartments.map((department, index) => (
											<Badge
												key={department.id}
												variant={index === 0 ? "default" : "secondary"}
												className="gap-1"
											>
												{index === 0 && <span className="text-[10px] opacity-80">主</span>}
												{department.name}
												<button
													type="button"
													className="ml-0.5 rounded-full p-0.5 hover:bg-black/10"
													onClick={() => toggleDepartment(department.id)}
												>
													<X className="size-3" />
												</button>
											</Badge>
										))}
									</div>
								</ScrollArea>
							)}
							<div className="flex gap-2 pt-2">
								<Button
									type="button"
									variant="outline"
									size="sm"
									onClick={() => setPickerOpen(true)}
								>
									选择部门
								</Button>
								{selectedIds.length > 0 && (
									<Button
										type="button"
										variant="ghost"
										size="sm"
										onClick={() => setSelectedIds([])}
									>
										清空
									</Button>
								)}
							</div>
						</div>
					</div>
				</div>

				<DialogFooter className="shrink-0 px-6 py-4">
					<Button type="button" variant="outline" onClick={onClose} disabled={submitting}>
						取消
					</Button>
					<Button type="button" onClick={handleSubmit} disabled={submitting}>
						{submitting ? <Loader2 className="mr-2 size-4 animate-spin" /> : null}
						确认添加
					</Button>
				</DialogFooter>
			</DialogContent>

			<DepartmentPickerDialog
				departments={departments}
				selectedIds={selectedIds}
				open={pickerOpen}
				onClose={() => setPickerOpen(false)}
				onConfirm={(ids) => {
					setSelectedIds(ids);
					setPickerOpen(false);
				}}
			/>
		</Dialog>
	);
}

type EditMemberDialogProps = {
	departments: Department[];
	member: OrgMember;
	submitting: boolean;
	onClose: () => void;
	onSubmit: (id: number, name: string, departmentIds: number[]) => void;
};

function EditMemberDialog({
	departments,
	member,
	submitting,
	onClose,
	onSubmit,
}: EditMemberDialogProps) {
	const [name, setName] = useState(member.user_name ?? "");
	const [selectedIds, setSelectedIds] = useState<number[]>(
		(member.departments ?? []).map((dept) => dept.department_id),
	);
	const [pickerOpen, setPickerOpen] = useState(false);

	const toggleDepartment = (id: number) => {
		setSelectedIds((prev) => {
			if (prev.includes(id)) {
				return prev.filter((item) => item !== id);
			}
			return [...prev, id];
		});
	};

	const departmentById = useMemo(() => {
		const map = new Map<number, Department>();
		for (const department of departments) {
			map.set(department.id, department);
		}
		return map;
	}, [departments]);

	const selectedDepartments = useMemo(() => {
		return selectedIds
			.map((id) => departmentById.get(id))
			.filter((item): item is Department => item !== undefined);
	}, [selectedIds, departmentById]);

	const handleSubmit = () => {
		const trimmedName = name.trim();
		if (!trimmedName) {
			toast.error("用户名不能为空");
			return;
		}
		if (selectedIds.length === 0) {
			toast.error("请选择所属部门");
			return;
		}
		onSubmit(member.id, trimmedName, selectedIds);
	};

	return (
		<Dialog open onOpenChange={(open) => !open && onClose()}>
			<DialogContent className="flex h-[min(640px,85vh)] max-h-[720px] w-[min(520px,95vw)] max-w-none flex-col gap-0 p-0 sm:max-w-none">
				<DialogHeader className="shrink-0 px-6 py-4">
					<DialogTitle>编辑成员</DialogTitle>
					<DialogDescription>修改成员的用户名和所属部门，手机号不可修改</DialogDescription>
				</DialogHeader>

				<div className="flex min-h-0 flex-1 flex-col gap-4 px-6 py-2">
					<div className="space-y-2">
						<label
							htmlFor="edit-member-name"
							className="text-sm font-medium text-[var(--leros-text-strong)]"
						>
							用户名
						</label>
						<Input
							id="edit-member-name"
							value={name}
							onChange={(event) => setName(event.target.value)}
							placeholder="请输入用户名"
							autoFocus
						/>
					</div>

					<div className="space-y-2">
						<label
							htmlFor="edit-member-phone"
							className="text-sm font-medium text-[var(--leros-text-strong)]"
						>
							手机号
						</label>
						<Input id="edit-member-phone" value={member.user_phone ?? "-"} disabled />
					</div>

					<div className="flex min-h-0 flex-1 flex-col gap-2">
						<div className="flex items-center justify-between">
							<span className="text-sm font-medium text-[var(--leros-text-strong)]">所属部门</span>
							<span className="text-xs text-[var(--leros-text-subtle)]">
								已选 {selectedIds.length} 个
							</span>
						</div>
						<div className="flex min-h-0 flex-1 flex-col gap-2 rounded-xl border border-[var(--leros-control-border)] p-3">
							{selectedDepartments.length === 0 ? (
								<p className="text-sm text-[var(--leros-text-subtle)]">暂未选择部门</p>
							) : (
								<ScrollArea className="flex-1">
									<div className="flex flex-wrap gap-2">
										{selectedDepartments.map((department, index) => (
											<Badge
												key={department.id}
												variant={index === 0 ? "default" : "secondary"}
												className="gap-1"
											>
												{index === 0 && <span className="text-[10px] opacity-80">主</span>}
												{department.name}
												<button
													type="button"
													className="ml-0.5 rounded-full p-0.5 hover:bg-black/10"
													onClick={() => toggleDepartment(department.id)}
												>
													<X className="size-3" />
												</button>
											</Badge>
										))}
									</div>
								</ScrollArea>
							)}
							<div className="flex gap-2 pt-2">
								<Button
									type="button"
									variant="outline"
									size="sm"
									onClick={() => setPickerOpen(true)}
								>
									选择部门
								</Button>
								{selectedIds.length > 0 && (
									<Button
										type="button"
										variant="ghost"
										size="sm"
										onClick={() => setSelectedIds([])}
									>
										清空
									</Button>
								)}
							</div>
						</div>
					</div>
				</div>

				<DialogFooter className="shrink-0 px-6 py-4">
					<Button type="button" variant="outline" onClick={onClose} disabled={submitting}>
						取消
					</Button>
					<Button type="button" onClick={handleSubmit} disabled={submitting}>
						{submitting ? <Loader2 className="mr-2 size-4 animate-spin" /> : null}
						保存
					</Button>
				</DialogFooter>
			</DialogContent>

			<DepartmentPickerDialog
				departments={departments}
				selectedIds={selectedIds}
				open={pickerOpen}
				onClose={() => setPickerOpen(false)}
				onConfirm={(ids) => {
					setSelectedIds(ids);
					setPickerOpen(false);
				}}
			/>
		</Dialog>
	);
}

type DepartmentPickerDialogProps = {
	departments: Department[];
	selectedIds: number[];
	open: boolean;
	onClose: () => void;
	onConfirm: (ids: number[]) => void;
};

function DepartmentPickerDialog({
	departments,
	selectedIds,
	open,
	onClose,
	onConfirm,
}: DepartmentPickerDialogProps) {
	const [draftIds, setDraftIds] = useState<number[]>(selectedIds);

	useEffect(() => {
		if (open) {
			setDraftIds(selectedIds);
		}
	}, [open, selectedIds]);

	const toggle = (id: number) => {
		setDraftIds((prev) => {
			if (prev.includes(id)) {
				return prev.filter((item) => item !== id);
			}
			return [...prev, id];
		});
	};

	const renderNode = (node: DepartmentTreeNode, level: number) => {
		const checked = draftIds.includes(node.id);
		return (
			<div key={node.id}>
				<div
					className="flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 hover:bg-slate-100"
					style={{ paddingLeft: `${level * 16 + 8}px` }}
				>
					<Checkbox checked={checked} onCheckedChange={() => toggle(node.id)} />
					<span className="min-w-0 flex-1 truncate text-sm text-[var(--leros-text)]">
						{node.name}
					</span>
				</div>
				{node.children.map((child) => renderNode(child, level + 1))}
			</div>
		);
	};

	const tree = useMemo(() => buildDepartmentTree(departments), [departments]);

	return (
		<Dialog open={open} onOpenChange={(isOpen) => !isOpen && onClose()}>
			<DialogContent className="flex h-[min(560px,85vh)] w-[min(400px,95vw)] max-w-none flex-col gap-0 p-0 sm:max-w-none">
				<DialogHeader className="shrink-0 px-6 py-4">
					<DialogTitle>选择部门</DialogTitle>
					<DialogDescription>勾选成员所属的部门，第一个选择的部门将作为主部门</DialogDescription>
				</DialogHeader>

				<ScrollArea className="min-h-0 flex-1 px-6 py-2">
					<div className="space-y-1">{tree.map((node) => renderNode(node, 0))}</div>
				</ScrollArea>

				<DialogFooter className="shrink-0 px-6 py-4">
					<Button type="button" variant="outline" onClick={onClose}>
						取消
					</Button>
					<Button type="button" onClick={() => onConfirm(draftIds)}>
						确认 ({draftIds.length})
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
