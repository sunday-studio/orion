import assert from "node:assert/strict";
import { test } from "node:test";
import {
  SIDEBAR_STORAGE_KEY,
  parseSidebarMode,
  readSidebarMode,
  writeSidebarMode,
} from "./sidebar-state.ts";

const memory = (initial: Record<string, string> = {}) => {
  const data = new Map(Object.entries(initial));
  return {
    getItem: (key: string) => data.get(key) ?? null,
    setItem: (key: string, value: string) => void data.set(key, value),
    data,
  };
};

test("stored modes are read back", () => {
  for (const mode of ["open", "collapsed"] as const) {
    assert.equal(parseSidebarMode(mode, 1440), mode);
  }
});

test("nothing stored: open on desktop, collapsed on a narrow screen", () => {
  assert.equal(parseSidebarMode(null, 1440), "open");
  assert.equal(parseSidebarMode(null, 767), "collapsed");
  assert.equal(parseSidebarMode(null, 768), "open");
});

test("the old fully hidden state becomes the icon rail", () => {
  assert.equal(parseSidebarMode("closed", 1440), "collapsed");
});

test("a garbage value is ignored", () => {
  assert.equal(parseSidebarMode("hidden", 1440), "open");
  assert.equal(parseSidebarMode("", 400), "collapsed");
});

test("the mode survives a write and a read", () => {
  const storage = memory();
  writeSidebarMode("collapsed", storage);
  assert.equal(storage.data.get(SIDEBAR_STORAGE_KEY), "collapsed");
  assert.equal(readSidebarMode(1440, storage), "collapsed");
  writeSidebarMode("open", storage);
  assert.equal(readSidebarMode(1440, storage), "open");
});

test("storage that throws or is missing falls back to the default", () => {
  const broken = {
    getItem: () => {
      throw new Error("blocked");
    },
    setItem: () => {
      throw new Error("blocked");
    },
  };
  assert.equal(readSidebarMode(1440, broken), "open");
  assert.doesNotThrow(() => writeSidebarMode("collapsed", broken));
  assert.equal(readSidebarMode(1440, null), "open");
  assert.doesNotThrow(() => writeSidebarMode("collapsed", null));
});
