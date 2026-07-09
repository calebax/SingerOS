"use client";

import type { Action, ResourceRef } from "@leros/store";
import { useCan } from "@leros/store";
import type { ReactNode } from "react";

type CanGateProps = {
	action: Action;
	resource: ResourceRef | null | undefined;
	children: ReactNode;
	fallback?: ReactNode;
	showWhileLoading?: boolean;
};

export function CanGate({
	action,
	resource,
	children,
	fallback = null,
	showWhileLoading = false,
}: CanGateProps) {
	const { allowed, loading } = useCan(action, resource);

	if (loading) {
		return showWhileLoading ? <>{children}</> : null;
	}
	if (!allowed) {
		return <>{fallback}</>;
	}
	return <>{children}</>;
}
