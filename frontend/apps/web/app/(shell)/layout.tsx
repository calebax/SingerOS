import type { ReactNode } from "react";
import { LerosShell } from "@/components/LerosShell";

export default function ShellLayout({ children }: { children: ReactNode }) {
	return <LerosShell>{children}</LerosShell>;
}
