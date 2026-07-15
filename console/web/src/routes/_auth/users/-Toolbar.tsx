import { Button, Flex, Input, Select, Typography, theme } from "antd";
import { UserRound, UserRoundPlus } from "lucide-react";
import { forwardRef } from "react";
import { useTranslation } from "react-i18next";
import { useUserRoleOptions } from "@/hooks/useUserRoleOptions";

const { Title, Text } = Typography;

/** 搜索框宽度 */
const SEARCH_WIDTH = 260;
/** 角色筛选宽度 */
const ROLE_WIDTH = 140;

export type ToolbarProps = {
  keywordInput: string;
  onKeywordChange: (value: string) => void;
  onSearch: (keyword: string) => void;
  onClearSearch: () => void;
  roleValue: string | undefined;
  onRoleChange: (role: string) => void;
  onCreateClick: () => void;
};

/**
 * Team 页页头：左侧标题 + 副标题，右侧搜索、角色筛选与"添加成员"按钮。
 * @param keywordInput 搜索关键字输入值
 * @param onKeywordChange 关键字输入变化回调
 * @param onSearch 提交搜索回调
 * @param onClearSearch 清空搜索回调
 * @param roleValue 当前角色筛选值
 * @param onRoleChange 角色筛选变化回调
 * @param onCreateClick 点击"添加成员"回调
 */
export const Toolbar = forwardRef<HTMLDivElement, ToolbarProps>(function Toolbar(
  {
    keywordInput,
    onKeywordChange,
    onSearch,
    onClearSearch,
    roleValue,
    onRoleChange,
    onCreateClick,
  },
  ref,
) {
  const { t } = useTranslation();
  const roleOptions = useUserRoleOptions();
  const { token } = theme.useToken();

  return (
    <Flex ref={ref} wrap justify="space-between" align="flex-end" gap={token.margin}>
      <Flex vertical gap={token.marginXXS} style={{ minWidth: 0 }}>
        <Title level={3} style={{ margin: 0 }}>
          {t("users.title")}
        </Title>
        <Text type="secondary">{t("users.subtitle")}</Text>
      </Flex>
      <Flex wrap gap={token.marginSM} align="center">
        <Input.Search
          allowClear
          placeholder={t("users.searchPlaceholder")}
          style={{ width: SEARCH_WIDTH }}
          value={keywordInput}
          onChange={(e) => onKeywordChange(e.target.value)}
          onSearch={(v) => onSearch(v)}
          onClear={onClearSearch}
        />
        <Select
          allowClear
          placeholder={t("users.rolePlaceholder")}
          style={{ width: ROLE_WIDTH }}
          prefix={<UserRound size={token.fontSize} />}
          value={roleValue}
          onChange={(v) => onRoleChange(v ?? "")}
          options={[...roleOptions]}
        />
        <Button
          type="primary"
          icon={<UserRoundPlus size={token.fontSize} />}
          onClick={onCreateClick}
        >
          {t("users.addMember")}
        </Button>
      </Flex>
    </Flex>
  );
});
