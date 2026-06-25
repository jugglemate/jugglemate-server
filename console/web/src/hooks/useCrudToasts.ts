import { useMemo } from "react";
import type { MessageInstance } from "antd/es/message/interface";

type MutationLifecycle<TValues> = {
  onMutate?: (values: TValues) => void;
  onSuccess?: (values: TValues) => void;
  onError?: (values: TValues) => void;
};

export type CrudToastLifecycles<TCreate, TUpdate> = {
  createLifecycle?: MutationLifecycle<TCreate>;
  updateLifecycle?: MutationLifecycle<TUpdate>;
  deleteLifecycle?: MutationLifecycle<string>;
};

export function useCrudToasts<TCreate, TUpdate>(options: {
  message: MessageInstance;
  resourceKey: string;
}): CrudToastLifecycles<TCreate, TUpdate> {
  const { message, resourceKey } = options;

  const keys = useMemo(
    () => ({
      create: `${resourceKey}-mutation-create`,
      update: `${resourceKey}-mutation-update`,
      delete: `${resourceKey}-mutation-delete`,
    }),
    [resourceKey],
  );

  return useMemo(
    () => ({
      createLifecycle: {
        onSuccess: () => {
          message.success({ content: "Created successfully", key: keys.create });
        },
        onError: () => {
          message.error({ content: "Create failed", key: keys.create });
        },
      },
      updateLifecycle: {
        onMutate: () => {
          message.loading({ content: "Updating…", key: keys.update, duration: 0 });
        },
        onSuccess: () => {
          message.success({ content: "Updated successfully", key: keys.update });
        },
        onError: () => {
          message.error({ content: "Update failed", key: keys.update });
        },
      },
      deleteLifecycle: {
        onMutate: () => {
          message.loading({ content: "Deleting…", key: keys.delete, duration: 0 });
        },
        onSuccess: () => {
          message.success({ content: "Deleted successfully", key: keys.delete });
        },
        onError: () => {
          message.error({ content: "Delete failed", key: keys.delete });
        },
      },
    }),
    [keys.create, keys.delete, keys.update, message],
  );
}
