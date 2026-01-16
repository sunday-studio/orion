import "vitest-dom/extend-expect";
import {expect, afterEach} from "vitest";
import * as matchers from "vitest-dom/matchers";
import {cleanup} from "@testing-library/react";

expect.extend(matchers);

// Auto-clean between tests
afterEach(() => {
  cleanup();
});
