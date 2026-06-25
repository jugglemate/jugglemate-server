import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { UseMutationResult } from "@tanstack/react-query";

type MutationLifecycle<TValues> = {
  onMutate?: (values: TValues) => void;
  onSuccess?: (values: TValues) => void;
  onError?: (values: TValues) => void;
};

export type ResourceListData<TItem extends { id: string }> = {
  list: readonly TItem[];
  total: number;
};

type OptimisticFlags = {
  update?: boolean;
  delete?: boolean;
};

type UseResourceCRUDOptions<
  TListData extends ResourceListData<{ id: string }>,
  TCreateValues,
  TUpdateValues extends { id: string },
> = {
  queryKey: readonly unknown[];
  invalidateKey?: readonly unknown[];
  queryFn: () => Promise<unknown>;
  select: (raw: unknown) => TListData;
  createFn: (values: TCreateValues) => Promise<unknown>;
  updateFn: (values: TUpdateValues) => Promise<unknown>;
  deleteFn: (id: string) => Promise<unknown>;
  createLifecycle?: MutationLifecycle<TCreateValues>;
  updateLifecycle?: MutationLifecycle<TUpdateValues>;
  deleteLifecycle?: MutationLifecycle<string>;
  optimistic?: OptimisticFlags;
};

export type ResourceCRUDResult<TListData, TCreateValues, TUpdateValues> = {
  data: TListData | undefined;
  isLoading: boolean;
  createMutation: UseMutationResult<unknown, Error, TCreateValues>;
  updateMutation: UseMutationResult<unknown, Error, TUpdateValues>;
  deleteMutation: UseMutationResult<unknown, Error, string>;
};

/** @deprecated Prefer `ResourceCRUDResult` — alias kept for plan wording compatibility */
export type CrudResult<TListData, TCreateValues, TUpdateValues> = ResourceCRUDResult<
  TListData,
  TCreateValues,
  TUpdateValues
>;

type RollbackCtx<TListData> = { snapshot: TListData | undefined };

export function applyOptimisticListUpdate<
  TItem extends { id: string },
  TUpdate extends { id: string },
>(data: ResourceListData<TItem>, values: TUpdate): { list: TItem[]; total: number } {
  const { id, ...patch } = values as TUpdate & Record<string, unknown>;
  return {
    list: data.list.map((row) =>
      row.id === id ? ({ ...(row as object), ...patch } as TItem) : row,
    ),
    total: data.total,
  };
}

export function applyOptimisticListDelete<TItem extends { id: string }>(
  data: ResourceListData<TItem>,
  id: string,
): { list: TItem[]; total: number } {
  return {
    list: data.list.filter((row) => row.id !== id),
    total: Math.max(0, data.total - 1),
  };
}

export function useResourceCRUD<
  TListData extends ResourceListData<{ id: string }>,
  TCreateValues,
  TUpdateValues extends { id: string },
>(
  options: UseResourceCRUDOptions<TListData, TCreateValues, TUpdateValues>,
): ResourceCRUDResult<TListData, TCreateValues, TUpdateValues> {
  const queryClient = useQueryClient();
  const invalidateKey = options.invalidateKey ?? [options.queryKey[0]];
  const queryKey = options.queryKey;
  const opt = options.optimistic;

  const query = useQuery({
    queryKey,
    queryFn: options.queryFn,
    select: options.select,
  });

  const createMutation = useMutation({
    mutationFn: options.createFn,
    onMutate: (values) => {
      options.createLifecycle?.onMutate?.(values);
    },
    onSuccess: (_data, values) => {
      void queryClient.invalidateQueries({ queryKey: invalidateKey });
      options.createLifecycle?.onSuccess?.(values);
    },
    onError: (_error, values) => {
      options.createLifecycle?.onError?.(values);
    },
  });

  const updateMutation = useMutation({
    mutationFn: options.updateFn,
    onMutate: async (values): Promise<RollbackCtx<TListData>> => {
      await queryClient.cancelQueries({ queryKey });
      const previous = queryClient.getQueryData<TListData>(queryKey);
      const applied = Boolean(opt?.update && previous);
      if (applied) {
        queryClient.setQueryData<TListData>(
          queryKey,
          applyOptimisticListUpdate(
            previous as ResourceListData<{ id: string }>,
            values as { id: string } & Record<string, unknown>,
          ) as unknown as TListData,
        );
      }
      options.updateLifecycle?.onMutate?.(values);
      return { snapshot: applied ? previous : undefined };
    },
    onSuccess: (_data, values) => {
      void queryClient.invalidateQueries({ queryKey: invalidateKey });
      options.updateLifecycle?.onSuccess?.(values);
    },
    onError: (_error, values, context) => {
      const snap = context?.snapshot;
      if (snap !== undefined) {
        queryClient.setQueryData(queryKey, snap);
      }
      options.updateLifecycle?.onError?.(values);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: options.deleteFn,
    onMutate: async (id): Promise<RollbackCtx<TListData>> => {
      await queryClient.cancelQueries({ queryKey });
      const previous = queryClient.getQueryData<TListData>(queryKey);
      const applied = Boolean(opt?.delete && previous);
      if (applied) {
        queryClient.setQueryData<TListData>(
          queryKey,
          applyOptimisticListDelete(
            previous as ResourceListData<{ id: string }>,
            id,
          ) as unknown as TListData,
        );
      }
      options.deleteLifecycle?.onMutate?.(id);
      return { snapshot: applied ? previous : undefined };
    },
    onSuccess: (_data, id) => {
      void queryClient.invalidateQueries({ queryKey: invalidateKey });
      options.deleteLifecycle?.onSuccess?.(id);
    },
    onError: (_error, id, context) => {
      const snap = context?.snapshot;
      if (snap !== undefined) {
        queryClient.setQueryData(queryKey, snap);
      }
      options.deleteLifecycle?.onError?.(id);
    },
  });

  return {
    data: query.data,
    isLoading: query.isLoading,
    createMutation,
    updateMutation,
    deleteMutation,
  };
}
