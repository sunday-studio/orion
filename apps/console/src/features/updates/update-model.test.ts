import assert from "node:assert/strict";
import { describe, it } from "node:test";
import {
  compareVersions,
  coreUpdateCommand,
  isBehind,
  nodeCanUpdate,
  pendingUpdate,
  releaseNotesUrl,
  stateBadge,
} from "./update-model.ts";

describe("versions", () => {
  it("orders releases and treats a pre-release as older than its release", () => {
    assert.equal(compareVersions("0.9.2", "0.9.3"), -1);
    assert.equal(compareVersions("v0.9.3", "0.9.3"), 0);
    assert.equal(compareVersions("0.10.0", "0.9.9"), 1);
    assert.equal(compareVersions("0.9.3-rc1", "0.9.3"), -1);
    assert.equal(compareVersions("1.0", "1.0.0"), 0);
  });

  it("never says a dev build is behind", () => {
    assert.equal(compareVersions("dev", "0.9.3"), undefined);
    assert.equal(isBehind("dev", "0.9.3"), false);
    assert.equal(isBehind(undefined, "0.9.3"), false);
  });

  it("offers a Node the release Core runs, never a newer one", () => {
    const settings = { current_version: "0.9.3", latest_version: "0.9.4" };
    assert.equal(nodeCanUpdate("0.9.1", settings), true);
    assert.equal(nodeCanUpdate("0.9.3", settings), false);
    assert.equal(nodeCanUpdate("0.9.4", settings), false);
    assert.equal(nodeCanUpdate("0.9.1", {}), false);
    assert.equal(nodeCanUpdate("dev", settings), false);
    assert.equal(nodeCanUpdate("0.9.1", { current_version: "dev" }), false);
  });

  it("links to the notes of a release", () => {
    assert.equal(
      releaseNotesUrl("0.9.3"),
      "https://github.com/sunday-studio/orion/releases/tag/0.9.3",
    );
    assert.equal(
      releaseNotesUrl("v0.9.3"),
      "https://github.com/sunday-studio/orion/releases/tag/0.9.3",
    );
  });
});

describe("states", () => {
  it("only offers an update that is neither skipped nor installed", () => {
    assert.equal(pendingUpdate({ state: "available" })?.state, "available");
    assert.equal(pendingUpdate({ state: "security" })?.state, "security");
    assert.equal(pendingUpdate({ state: "skipped" }), undefined);
    assert.equal(pendingUpdate({ state: "up_to_date" }), undefined);
  });

  it("uses the same words for each state", () => {
    assert.deepEqual(stateBadge({ state: "available", latest_version: "0.9.3" }), {
      tone: "info",
      text: "0.9.3 available",
    });
    assert.equal(stateBadge({ state: "security" }).text, "Security update");
    assert.equal(stateBadge({ state: "error" }).text, "Could not check");
    assert.equal(stateBadge({ state: "off" }).text, "Checks are off");
    assert.equal(stateBadge({ state: "up_to_date" }).text, "Up to date");
  });

  it("builds the Compose command for a version", () => {
    assert.equal(
      coreUpdateCommand("0.9.3"),
      "ORION_CORE_IMAGE=ghcr.io/sunday-studio/orion-core:0.9.3 docker compose up -d",
    );
  });
});
