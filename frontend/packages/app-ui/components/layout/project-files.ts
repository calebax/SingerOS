export type BackendProjectFileNodeLike = {
	name?: string;
	path?: string;
	type?: string;
	children?: BackendProjectFileNodeLike[];
	size?: number;
	mime_type?: string;
	mod_time?: number;
	created_at?: number;
	public_id?: string;
	storage_uri?: string;
	initial_file_public_id?: string;
	version_no?: number;
	version_label?: string;
	version_count?: number;
};

export type ProjectFileNode = {
	name: string;
	path: string;
	type: "file" | "directory";
	children: ProjectFileNode[];
	size: number;
	mimeType: string;
	modTime: number;
	createdAt: number;
	publicId: string;
	storageUri: string;
	initialFilePublicId: string;
	versionNo: number;
	versionLabel: string;
	versionCount: number;
};

// 统一清洗后端文件树结构，避免页面层到处处理空值和字段名差异。
export function normalizeProjectFileTree(
	nodes: BackendProjectFileNodeLike[] | null | undefined,
): ProjectFileNode[] {
	if (!Array.isArray(nodes)) return [];

	return nodes.map((node) => ({
		name: String(node.name ?? ""),
		path: normalizeFilePath(node.path),
		type: node.type === "directory" ? "directory" : "file",
		size: typeof node.size === "number" ? node.size : 0,
		mimeType: typeof node.mime_type === "string" ? node.mime_type : "",
		modTime: typeof node.mod_time === "number" ? node.mod_time : 0,
		createdAt: typeof node.created_at === "number" ? node.created_at * 1000 : 0,
		publicId: typeof node.public_id === "string" ? node.public_id : "",
		storageUri: typeof node.storage_uri === "string" ? node.storage_uri : "",
		initialFilePublicId:
			typeof node.initial_file_public_id === "string" ? node.initial_file_public_id : "",
		versionNo: typeof node.version_no === "number" ? node.version_no : 0,
		versionLabel:
			typeof node.version_label === "string" && node.version_label
				? node.version_label
				: typeof node.version_no === "number" && node.version_no > 0
					? `第 ${node.version_no} 版`
					: "",
		versionCount: typeof node.version_count === "number" ? node.version_count : 0,
		children: normalizeProjectFileTree(node.children),
	}));
}

// 文件页默认要预览第一个文件，所以这里直接给出可选文件的稳定顺序。
export function collectSelectableFiles(nodes: ProjectFileNode[]): ProjectFileNode[] {
	const result: ProjectFileNode[] = [];

	for (const node of nodes) {
		if (node.type === "file") {
			result.push(node);
			continue;
		}
		result.push(...collectSelectableFiles(node.children));
	}

	return result;
}

export type FileSource = "task" | "upload";

export function getFileSource(path: string): FileSource {
	const normalized = normalizeFilePath(path);
	return normalized.startsWith("artifacts") ? "task" : "upload";
}

// 中文注释：优先展示文件扩展名，保证未知 MIME 时仍能识别文件格式。
export function getProjectFileTypeLabel(file: Pick<ProjectFileNode, "name" | "mimeType">): string {
	const extension = file.name.match(/\.([^.]+)$/)?.[1];
	if (extension) return extension.toUpperCase();

	const mimeSubtype = file.mimeType.split("/")[1]?.split(";")[0];
	return mimeSubtype ? mimeSubtype.toUpperCase() : "未知";
}

export function sortProjectFilesByUploadedTimeDesc(files: ProjectFileNode[]): ProjectFileNode[] {
	return [...files].sort((left, right) => right.createdAt - left.createdAt);
}

function normalizeFilePath(path: string | undefined): string {
	if (!path) return "";
	return path.replace(/^\/+/, "");
}
