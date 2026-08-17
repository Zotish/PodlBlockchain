import { describe, expect, it } from "vitest";
import { validatePasswordStrength } from "./WalletLogin";
import { getTrustedWalletConnectOrigin } from "./WalletDashboard";

describe("wallet security boundaries", () => {
  it("accepts a strong local vault password", () => {
    expect(validatePasswordStrength("PoDL-Vault-2026!")).toBe("");
  });

  it("rejects incomplete local vault passwords", () => {
    expect(validatePasswordStrength("short")).toMatch(/10 characters/i);
    expect(validatePasswordStrength("lowercase-only-password")).toMatch(/uppercase/i);
  });

  it("allows the local DEX during local development", () => {
    expect(
      getTrustedWalletConnectOrigin(
        "http://localhost:3000/swap",
        "http://localhost:5173"
      )
    ).toBe("http://localhost:3000");
  });

  it("rejects an unconfigured requesting origin", () => {
    expect(
      getTrustedWalletConnectOrigin(
        "https://malicious.example/connect",
        "https://explorer.example"
      )
    ).toBe("");
  });
});
