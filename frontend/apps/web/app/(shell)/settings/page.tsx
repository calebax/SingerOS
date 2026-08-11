"use client";

import { ModelManagementView, PrivateSettingsPage } from "@leros/app-ui";
import { isPrivateDeployment } from "@leros/store";

export default function SettingsPage() {
	return (
		<>
			<ModelManagementView />
			{isPrivateDeployment ? <PrivateSettingsPage /> : null}
		</>
	);
}
