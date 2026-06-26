import type { CSSProperties } from "react";
import { useMemo } from "react";
import { createFileRoute } from "@tanstack/react-router";
import { Card, Col, Row, Typography, Avatar, theme, Flex, Skeleton, Timeline, Tag } from "antd";
import { useQuery } from "@tanstack/react-query";
import { DollarSign, Users, CreditCard, Activity } from "lucide-react";
import { useTranslation } from "react-i18next";
import "./index.css";

const { Title, Text } = Typography;

export const Route = createFileRoute("/_auth/dashboard/")({
  component: DashboardPage,
});

async function fetchDashboardShell() {
  await new Promise((r) => setTimeout(r, 1000));
  return true;
}

type AntToken = ReturnType<typeof theme.useToken>["token"];

/** Matches loaded stat card body height: title row, value, description. */
function StatCardSkeleton({ token }: { token: AntToken }) {
  const titleLine = Math.round(token.fontSizeSM * token.lineHeight);
  /** Lucide default icon box 24px to align with the first row of real cards. */
  const iconBox = 24;
  const titleRowHeight = Math.max(titleLine, iconBox);
  const valueLine = Math.round(24 * 1);
  const descLine = Math.round(token.fontSizeSM * token.lineHeight);

  return (
    <Card styles={{ body: { padding: token.paddingLG } }}>
      <div className="dash-skel-stat">
        <Flex
          justify="space-between"
          align="center"
          style={{ marginBottom: token.marginXS, minHeight: titleRowHeight }}
        >
          <Skeleton.Input active size="small" style={{ width: "55%", height: titleLine }} />
          <Skeleton.Avatar active size="small" style={{ width: iconBox, height: iconBox }} />
        </Flex>
        <Flex vertical gap={token.marginXS}>
          <Skeleton.Input active style={{ width: "25%", height: valueLine }} />
          <Skeleton.Input
            active
            size="small"
            style={{ width: "75%", maxWidth: 200, height: descLine }}
          />
        </Flex>
      </div>
    </Card>
  );
}

function DashboardSkeleton() {
  const { token } = theme.useToken();
  const body = { padding: token.paddingLG } as const;
  const cardTitleSkel = (w: number) => (
    <Skeleton.Input
      active
      style={{
        width: w,
        height: Math.round(Number(token.fontSizeHeading5) * Number(token.lineHeightHeading5)),
      }}
    />
  );

  return (
    <Flex vertical gap={token.marginLG}>
      <Row gutter={[16, 16]}>
        {[0, 1, 2, 3].map((i) => (
          <Col xs={24} sm={12} lg={6} key={i}>
            <StatCardSkeleton token={token} />
          </Col>
        ))}
      </Row>
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={14}>
          <Card title={cardTitleSkel(88)} styles={{ body: body }} style={{ height: "100%" }}>
            <Skeleton.Node active style={{ width: "100%", height: 300 }}>
              <div style={{ width: "100%", height: "100%" }} />
            </Skeleton.Node>
          </Card>
        </Col>
        <Col xs={24} lg={10}>
          <Card title={cardTitleSkel(112)} styles={{ body: body }} style={{ height: "100%" }}>
            <Flex vertical className="dash-skel-recent">
              {[0, 1, 2, 3, 4].map((i) => (
                <Flex
                  key={i}
                  align="center"
                  justify="space-between"
                  gap={token.marginSM}
                  style={{
                    padding: `${token.paddingXS}px ${token.paddingSM}px`,
                    borderRadius: token.borderRadius,
                  }}
                >
                  <Flex align="center" gap={token.marginSM} style={{ minWidth: 0, flex: 1 }}>
                    <Skeleton.Avatar active size={40} shape="circle" />
                    <Flex vertical style={{ minWidth: 0, flex: 1 }} gap={2}>
                      <Skeleton.Input active size="small" style={{ width: "50%", height: 12 }} />
                      <Skeleton.Input active size="small" style={{ width: "85%", height: 12 }} />
                    </Flex>
                  </Flex>
                </Flex>
              ))}
            </Flex>
          </Card>
        </Col>
      </Row>
    </Flex>
  );
}

function DashboardPage() {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const { isPending } = useQuery({
    queryKey: ["dashboard"],
    queryFn: fetchDashboardShell,
    staleTime: 60_000,
  });

  /* Hover: emphasize border; keep background matching the card so light theme doesn't gray the whole block */
  const cardHoverStyle = {
    ["--dash-card-hover-bg" as string]: token.colorBgContainer,
    ["--dash-card-hover-border" as string]: token.colorPrimaryBorderHover,
    ["--dash-card-hover-shadow" as string]: "none",
  } as CSSProperties;

  const stats = useMemo(
    () => [
      {
        title: t("dashboard.totalRevenue"),
        value: "$45,231.89",
        description: t("dashboard.revenueDesc"),
        icon: <DollarSign style={{ color: token.colorTextSecondary }} />,
      },
      {
        title: t("dashboard.subscriptions"),
        value: "+2350",
        description: t("dashboard.subscriptionsDesc"),
        icon: <Users style={{ color: token.colorTextSecondary }} />,
      },
      {
        title: t("dashboard.sales"),
        value: "+12,234",
        description: t("dashboard.salesDesc"),
        icon: <CreditCard style={{ color: token.colorTextSecondary }} />,
      },
      {
        title: t("dashboard.activeNow"),
        value: "+573",
        description: t("dashboard.activeDesc"),
        icon: <Activity style={{ color: token.colorTextSecondary }} />,
      },
    ],
    [t, token.colorTextSecondary],
  );

  const recentSales = useMemo(
    () => [
      {
        name: "Olivia Martin",
        email: "olivia.martin@email.com",
        amount: "+$1,999.00",
        initials: "OM",
      },
      {
        name: "Jackson Lee",
        email: "jackson.lee@email.com",
        amount: "+$39.00",
        initials: "JL",
      },
      {
        name: "Isabella Nguyen",
        email: "isabella.nguyen@email.com",
        amount: "+$299.00",
        initials: "IN",
      },
      {
        name: "William Kim",
        email: "will@email.com",
        amount: "+$99.00",
        initials: "WK",
      },
      {
        name: "Sofia Davis",
        email: "sofia.davis@email.com",
        amount: "+$39.00",
        initials: "SD",
      },
    ],
    [],
  );

  const timelineItems = useMemo(
    () => [
      {
        color: "green" as const,
        content: (
          <Flex vertical gap={4}>
            <Text strong>{t("dashboard.timelineDeploy")}</Text>
            <Text type="secondary">{t("dashboard.timelineDeployDesc")}</Text>
          </Flex>
        ),
      },
      {
        color: "blue" as const,
        content: (
          <Flex vertical gap={4}>
            <Text strong>{t("dashboard.timelineMenu")}</Text>
            <Text type="secondary">{t("dashboard.timelineMenuDesc")}</Text>
          </Flex>
        ),
      },
      {
        color: "gold" as const,
        content: (
          <Flex vertical gap={4}>
            <Text strong>{t("dashboard.timelineSecurity")}</Text>
            <Text type="secondary">{t("dashboard.timelineSecurityDesc")}</Text>
          </Flex>
        ),
      },
      {
        color: "red" as const,
        content: (
          <Flex vertical gap={4}>
            <Text strong>{t("dashboard.timelineIncident")}</Text>
            <Text type="secondary">{t("dashboard.timelineIncidentDesc")}</Text>
          </Flex>
        ),
      },
    ],
    [t],
  );

  if (isPending) {
    return <DashboardSkeleton />;
  }

  return (
    <Flex vertical gap={token.marginLG}>
      <Row gutter={[16, 16]}>
        {stats.map((stat) => (
          <Col xs={24} sm={12} lg={6} key={stat.title}>
            <Card
              className="dash-card-interactive"
              style={cardHoverStyle}
              styles={{ body: { padding: token.paddingLG } }}
            >
              <Flex justify="space-between" align="center" style={{ marginBottom: token.marginXS }}>
                <Text strong style={{ fontSize: token.fontSizeSM }}>
                  {stat.title}
                </Text>
                {stat.icon}
              </Flex>
              <div style={{ fontSize: 24, fontWeight: "bold", marginBottom: 4 }}>{stat.value}</div>
              <Text ellipsis type="secondary" style={{ fontSize: token.fontSizeSM }}>
                {stat.description}
              </Text>
            </Card>
          </Col>
        ))}
      </Row>

      <Row gutter={[16, 16]}>
        <Col xs={24} lg={14}>
          <Card
            className="dash-card-interactive"
            style={{ ...cardHoverStyle, height: "100%" }}
            title={
              <Flex align="center" gap={token.marginSM}>
                <Title level={5} style={{ margin: 0 }}>
                  {t("dashboard.timeline")}
                </Title>
                <Tag variant="filled" color="processing">
                  {t("dashboard.today")}
                </Tag>
              </Flex>
            }
          >
            <Timeline className="dash-timeline" items={timelineItems} />
          </Card>
        </Col>
        <Col xs={24} lg={10}>
          <Card
            className="dash-card-interactive"
            style={{ ...cardHoverStyle, height: "100%" }}
            title={
              <Title level={5} style={{ margin: 0 }}>
                {t("dashboard.recentSales")}
              </Title>
            }
          >
            <Flex
              vertical
              style={{
                ["--dash-recent-hover-bg" as string]: token.colorBgTextHover,
              }}
            >
              {recentSales.map((item) => (
                <Flex
                  key={item.email}
                  className="dash-recent-row"
                  align="center"
                  justify="space-between"
                  style={{
                    padding: `${token.paddingXS}px ${token.paddingSM}px`,
                    borderRadius: token.borderRadius,
                  }}
                >
                  <Flex align="center" gap={token.marginSM} style={{ minWidth: 0 }}>
                    <Avatar
                      size={40}
                      shape="circle"
                      style={{
                        flexShrink: 0,
                        backgroundColor: token.colorFillSecondary,
                        color: token.colorTextSecondary,
                        fontWeight: 600,
                        fontSize: token.fontSizeSM,
                      }}
                    >
                      {item.initials}
                    </Avatar>
                    <Flex vertical style={{ minWidth: 0 }}>
                      <Text strong style={{ fontSize: token.fontSizeSM, lineHeight: 1 }}>
                        {item.name}
                      </Text>
                      <Text type="secondary" style={{ fontSize: token.fontSizeSM }} ellipsis>
                        {item.email}
                      </Text>
                    </Flex>
                  </Flex>
                  <div style={{ fontWeight: 500 }}>{item.amount}</div>
                </Flex>
              ))}
            </Flex>
          </Card>
        </Col>
      </Row>
    </Flex>
  );
}
