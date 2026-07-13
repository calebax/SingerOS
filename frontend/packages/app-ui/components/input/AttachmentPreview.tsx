"use client";

import type { Attachment } from "@leros/store/types/chat";
import { X } from "lucide-react";
import { ProjectFileTypeIcon } from "../layout/project-file-type-icon";

export function AttachmentPreview({
	attachments,
	onPreview,
	onRemove,
}: {
	attachments: Attachment[];
	onPreview: (attachment: Attachment) => void;
	onRemove: (id: string) => void;
}) {
	return (
		<div data-slot="attachment-preview" className="mb-3 flex flex-wrap gap-2">
			{attachments.map((attachment) => (
				<div
					key={attachment.id}
					className="group flex items-center gap-1 rounded-lg bg-white/90 p-1 text-sm shadow-sm ring-1 ring-slate-200/70 transition-colors hover:bg-blue-50/60 hover:ring-blue-200"
				>
					<button
						type="button"
						onClick={() => onPreview(attachment)}
						className="flex min-w-0 cursor-pointer items-center gap-2 rounded-md px-2 py-1 text-left"
						title="点击预览"
					>
						{attachment.type === "image" && attachment.url ? (
							<img
								src={attachment.url}
								alt={attachment.name}
								className="size-8 rounded object-cover"
							/>
						) : (
							<div className="flex size-8 shrink-0 items-center justify-center rounded bg-slate-50">
								<ProjectFileTypeIcon fileName={attachment.name} className="size-6 object-contain" />
							</div>
						)}
						<span className="max-w-[160px] truncate text-slate-600">{attachment.name}</span>
					</button>
					<button
						type="button"
						onClick={() => onRemove(attachment.id)}
						className="text-slate-400 transition-colors hover:text-slate-600"
						aria-label={`移除 ${attachment.name}`}
					>
						<X className="size-3.5" />
					</button>
				</div>
			))}
		</div>
	);
}
