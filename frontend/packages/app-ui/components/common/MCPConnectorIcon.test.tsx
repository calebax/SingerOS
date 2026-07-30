import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { MCPConnectorIcon } from "./MCPConnectorIcon";

describe("MCPConnectorIcon", () => {
	afterEach(cleanup);

	it("uses the CoreKG platform logo for CoreKG connector identities", () => {
		render(<MCPConnectorIcon code="corekg-0123456789abcdef" name="CoreKG" />);

		expect(screen.getByRole("img", { name: "CoreKG Logo" })).toHaveAttribute("src");
	});

	it("does not use the CoreKG logo for custom connectors", () => {
		render(<MCPConnectorIcon code="mcp-0123456789abcdef" name="Custom MCP" />);

		expect(screen.queryByRole("img")).not.toBeInTheDocument();
	});
});
