import { useCallback, useEffect, useState } from "react";

export function useUrlSearchState<TSearch extends { keyword: string; offset: number }>(args: {
  search: TSearch;
  setSearch: (next: TSearch) => void;
}) {
  const { search, setSearch } = args;
  const [keywordInput, setKeywordInput] = useState(search.keyword);

  useEffect(() => {
    setKeywordInput(search.keyword);
  }, [search.keyword]);

  const commitKeyword = useCallback(() => {
    const trimmed = keywordInput.trim();
    setSearch({ ...search, keyword: trimmed, offset: 0 });
  }, [keywordInput, search, setSearch]);

  const applyKeyword = useCallback(
    (kw: string) => {
      const trimmed = kw.trim();
      setKeywordInput(trimmed);
      setSearch({ ...search, keyword: trimmed, offset: 0 });
    },
    [search, setSearch],
  );

  return { keywordInput, setKeywordInput, commitKeyword, applyKeyword };
}
