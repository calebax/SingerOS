import type { AppNavigation } from "./LeftRail";

export function navigateToWorkbench(
	navigation: AppNavigation | undefined,
	switchView: (view: "workbench") => void,
) {
	if (navigation) {
		navigation.goToRoute("workbench");
		return;
	}
	switchView("workbench");
}
