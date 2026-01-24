import {useSearchParams} from "react-router";
import {useCallback} from "react";

export function useURLState(key: string, defaultValue: string = "") {
  const [searchParams, setSearchParams] = useSearchParams();

  const setValue = useCallback(
    (value: string) => {
      setSearchParams(
        (params) => {
          if (value) {
            params.set(key, value);
          } else {
            params.delete(key);
          }
          return params;
        },
        {replace: true}
      );
    },
    [key, setSearchParams]
  );

  const deleteValue = useCallback(() => {
    setSearchParams(
      (params) => {
        params.delete(key);
        return params;
      },
      {replace: true}
    );
  }, [key, setSearchParams]);

  return {
    value: searchParams.get(key) || defaultValue,
    setValue,
    deleteValue,
    setSearchParams,
  };
}
