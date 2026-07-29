import { Card, Col, Row, Statistic, Typography } from 'antd'

const cards = [
  { title: '本月新签合同额', suffix: 'Sprint 6 接入' },
  { title: '本月回款额', suffix: 'Sprint 6 接入' },
  { title: '当前应收余额', suffix: 'Sprint 6 接入' },
  { title: '逾期金额', suffix: 'Sprint 6 接入' },
]

export default function Dashboard() {
  return (
    <div>
      <Typography.Title level={4}>经营仪表盘</Typography.Title>
      <Row gutter={[16, 16]}>
        {cards.map((c) => (
          <Col xs={24} sm={12} lg={6} key={c.title}>
            <Card>
              <Statistic title={c.title} value="--" suffix={<span style={{ fontSize: 12, color: '#999' }}>{c.suffix}</span>} />
            </Card>
          </Col>
        ))}
      </Row>
      <Card style={{ marginTop: 16 }}>
        <Typography.Text type="secondary">
          M0 工程基座已就绪：登录鉴权、RBAC 数据范围、审计日志、数据库 migration 均已打通。
          客户 / 商单 / 合同 / 回款模块将在 Sprint 2–6 依次上线。
        </Typography.Text>
      </Card>
    </div>
  )
}
