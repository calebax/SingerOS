"use client";

import { cn } from "@leros/ui/lib/utils";
import { DiceBearAvatar } from "../avatar/DiceBearAvatar";
import { ProtectedImage } from "../avatar/ProtectedImage";

const sizeClassMap = {
	sm: "size-7 text-xs",
	default: "size-12 text-lg",
	lg: "size-16 text-2xl",
};

export function AssistantAvatar({
	name,
	src,
	size = "default",
	className,
}: {
	name: string;
	src?: string | null;
	size?: keyof typeof sizeClassMap;
	className?: string;
}) {
	const sizeClass = sizeClassMap[size];
	const pixelSize = size === "lg" ? 128 : size === "sm" ? 56 : 96;
	const fallback = (
		<DiceBearAvatar
			seed={`digital-assistant:${name}`}
			alt={name}
			className={sizeClass}
			size={pixelSize}
		/>
	);

	return (
		<div
			className={cn(
				"flex shrink-0 items-center justify-center overflow-hidden rounded-full bg-gradient-to-br from-blue-500 to-indigo-600 font-semibold text-white",
				sizeClass,
				className,
			)}
		>
			{src ? (
				<ProtectedImage
					src={src}
					alt={name}
					className={cn("rounded-full object-cover", sizeClass)}
					fallback={fallback}
				/>
			) : (
				fallback
			)}
		</div>
	);
}
