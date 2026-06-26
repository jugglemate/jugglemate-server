import { useMemo } from "react";
import { useTranslation } from "react-i18next";
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
  const { t } = useTranslation();

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
          message.success({ content: t("crud.createSuccess"), key: keys.create });
        },
        onError: () => {
          message.error({ content: t("crud.createFailed"), key: keys.create });
        },
      },
      updateLifecycle: {
        onMutate: () => {
          message.loading({ content: t("crud.updating"), key: keys.update, duration: 0 });
        },
        onSuccess: () => {
          message.success({ content: t("crud.updateSuccess"), key: keys.update });
        },
        onError: () => {
          message.error({ content: t("crud.updateFailed"), key: keys.update });
        },
      },
      deleteLifecycle: {
        onMutate: () => {
          message.loading({ content: t("crud.deleting"), key: keys.delete, duration: 0 });
        },
        onSuccess: () => {
          message.success({ content: t("crud.deleteSuccess"), key: keys.delete });
        },
        onError: () => {
          message.error({ content: t("crud.deleteFailed"), key: keys.delete });
        },
      },
    }),
    [keys.create, keys.delete, keys.update, message, t],
  );
}
