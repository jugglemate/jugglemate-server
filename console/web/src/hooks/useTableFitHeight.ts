import { useLayoutEffect, useRef, useState, type RefObject } from "react";

export type UseTableFitHeightArgs = {
  pageShellRef: RefObject<HTMLDivElement | null>;
  toolbarRef: RefObject<HTMLDivElement | null>;
  middleRef: RefObject<HTMLDivElement | null>;
  tableFrameRef: RefObject<HTMLDivElement | null>;
  marginLG: number;
  rowCount: number;
  isLoading: boolean;
  showPagination: boolean;
};

export function useTableFitHeight(args: UseTableFitHeightArgs) {
  const {
    pageShellRef,
    toolbarRef,
    middleRef,
    tableFrameRef,
    marginLG,
    rowCount,
    isLoading,
    showPagination,
  } = args;

  const tableAvailableRef = useRef(0);
  const [tableAreaMaxHeight, setTableAreaMaxHeight] = useState<number | undefined>(() =>
    typeof window !== "undefined" ? Math.max(240, Math.floor(window.innerHeight - 280)) : undefined,
  );
  const [tableScrollY, setTableScrollY] = useState<number | undefined>(() => {
    if (typeof window === "undefined") return undefined;
    const maxH = Math.max(240, Math.floor(window.innerHeight - 280));
    return Math.max(120, maxH - 40);
  });

  useLayoutEffect(() => {
    const shell = pageShellRef.current;
    const toolbarEl = toolbarRef.current;
    const mid = middleRef.current;
    if (!shell || !toolbarEl || !mid) return;

    const headReserve = 40;
    const rowEstimate = 44;
    const paginationBlock = 56;

    let rafRetry = 0;
    const maxRafRetries = 8;

    const measure = () => {
      const toolbarRect = toolbarEl.getBoundingClientRect();
      const mainEl = shell.closest<HTMLElement>(".main-layout-main");
      const mainRect = mainEl?.getBoundingClientRect();
      const viewportBottom =
        typeof window !== "undefined" ? window.innerHeight : Number.POSITIVE_INFINITY;
      const clipBottom = mainRect
        ? mainRect.bottom
        : Math.min(shell.getBoundingClientRect().bottom, viewportBottom);

      const maxMiddleFromShell = Math.max(
        0,
        Math.floor(clipBottom - toolbarRect.bottom - marginLG),
      );

      const midHRaw = mid.clientHeight;
      const midH = midHRaw > 0 ? Math.min(midHRaw, maxMiddleFromShell) : maxMiddleFromShell;

      if (maxMiddleFromShell === 0 && rafRetry < maxRafRetries) {
        rafRetry += 1;
        requestAnimationFrame(() => requestAnimationFrame(measure));
        return;
      }

      const frameInner = Math.max(0, Math.floor(midH));
      tableAvailableRef.current = frameInner;

      setTableAreaMaxHeight(frameInner > 0 ? frameInner : undefined);

      const pagReserve = showPagination ? paginationBlock : 0;
      const bodyMax = Math.max(120, frameInner - headReserve - pagReserve);

      if (isLoading && rowCount === 0) {
        setTableScrollY(bodyMax);
        return;
      }

      if (rowCount === 0 && !isLoading) {
        setTableScrollY(undefined);
        return;
      }

      const naturalBodyH = headReserve + rowCount * rowEstimate;
      const naturalTotalH = naturalBodyH + pagReserve;
      const slack = 12;

      if (naturalTotalH + slack > frameInner) {
        setTableScrollY(bodyMax);
        return;
      }

      setTableScrollY(undefined);
      requestAnimationFrame(() => {
        const frame = tableFrameRef.current;
        const avail = tableAvailableRef.current;
        const minFrame = headReserve + pagReserve;
        if (!frame || avail <= minFrame) return;
        if (frame.scrollHeight > frame.clientHeight + 1) {
          setTableScrollY(Math.max(120, avail - headReserve - pagReserve));
        }
      });
    };

    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(shell);
    ro.observe(mid);
    return () => ro.disconnect();
  }, [
    showPagination,
    rowCount,
    isLoading,
    marginLG,
    pageShellRef,
    toolbarRef,
    middleRef,
    tableFrameRef,
  ]);

  return {
    tableAreaMaxHeight,
    tableScrollY,
    lockScrollHeight: tableScrollY != null,
  };
}
