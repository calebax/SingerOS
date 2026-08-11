"use client";

import { PrivateSettingsPage } from "@leros/app-ui";
import { isPrivateDeployment } from "@leros/store";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

export default function SettingsPage() {
	const router = useRouter();

	useEffect(() => {
		if (!isPrivateDeployment) {
			router.replace("/workbench");
		}
	}, [router]);

	if (!isPrivateDeployment) {
		return null;
	}

	return <PrivateSettingsPage />;
}
