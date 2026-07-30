import { cn } from "@leros/ui/lib/utils";
import { Server } from "lucide-react";

const COREKG_ICON_SRC = new URL("../../assets/icons/corekg.svg", import.meta.url).href;

function isCoreKGConnector(code: string) {
	const normalizedCode = code.trim().toLocaleLowerCase();
	return normalizedCode === "corekg" || normalizedCode.startsWith("corekg-");
}

export function MCPConnectorIcon({
	code,
	name,
	className,
}: {
	code: string;
	name?: string;
	className?: string;
}) {
	if (isCoreKGConnector(code)) {
		return (
			<img
				src={COREKG_ICON_SRC}
				alt={`${name || "CoreKG"} Logo`}
				className={cn("size-7 shrink-0 rounded-lg", className)}
			/>
		);
	}

	return (
		<div
			className={cn(
				"flex size-7 shrink-0 items-center justify-center rounded-lg bg-blue-50 text-blue-600",
				className,
			)}
		>
			<Server className="size-3.5" />
		</div>
	);
}
