import { parseAsInteger, useQueryStates } from "nuqs";
import { useCallback, useMemo } from "react";

type Options = {
  /** URL key for the size, for example "pageSize" or "monitorsSize". */
  key?: string;
  /** URL key of the page this table uses, reset to 1 whenever the size changes. */
  pageKey?: string;
};

/**
 * URL-backed (nuqs) page size. An unknown value in the URL falls back to `defaultLimit`;
 * changing the size returns the table to its first page.
 */
export const usePageSize = (
  defaultLimit: number,
  { key = "pageSize", pageKey = "page" }: Options = {},
) => {
  const parsers = useMemo(
    () => ({ [key]: parseAsInteger.withDefault(defaultLimit), [pageKey]: parseAsInteger }),
    [defaultLimit, key, pageKey],
  );
  const [state, setState] = useQueryStates(parsers);
  const requested = Number(state[key]);
  const limit =
    Number.isFinite(requested) && requested >= 1 && requested <= 200 ? requested : defaultLimit;
  const setLimit = useCallback(
    (next: number) => {
      void setState({ [key]: next === defaultLimit ? null : next, [pageKey]: null });
    },
    [defaultLimit, key, pageKey, setState],
  );
  return { limit, setLimit };
};
