import { useQuery } from '@tanstack/react-query'
import { Card, Col, Row, Statistic, Table, Tag, Empty, Spin } from 'antd'
import {
  BankOutlined,
  ArrowUpOutlined,
  RiseOutlined,
  WarningOutlined,
} from '@ant-design/icons'
import { apiDashboard, fenToYuan, CONTRACT_STATUS, DEAL_STAGES } from '../api'

export default function Dashboard() {
  const { data, isLoading } = useQuery({ queryKey: ['dashboard'], queryFn: apiDashboard })

  if (isLoading) {
    return <div style={{ padding: 64, textAlign: 'center' }}><Spin size="large" /></div>
  }
  if (!data) return <Empty description="暂无数据" />

  const { cards, expiring_contracts, overdue_plans, recent_won_deals } = data

  return (
    <div>
      <Row gutter={16}>
        <Col span={6}>
          <Card>
            <Statistic
              title="本月签约金额"
              value={fenToYuan(cards.signed_this_month_cent)}
              prefix={<BankOutlined />}
              valueStyle={{ color: '#1677ff' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="本月回款金额"
              value={fenToYuan(cards.paid_this_month_cent)}
              prefix={<ArrowUpOutlined />}
              valueStyle={{ color: '#52c41a' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="进行中商单"
              value={cards.open_deals}
              prefix={<RiseOutlined />}
              valueStyle={{ color: '#722ed1' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="逾期回款金额"
              value={fenToYuan(cards.overdue_amount_cent)}
              prefix={<WarningOutlined />}
              valueStyle={{ color: cards.overdue_amount_cent > 0 ? '#cf1322' : '#000' }}
            />
          </Card>
        </Col>
      </Row>

      <Row gutter={16} style={{ marginTop: 16 }}>
        <Col span={8}>
          <Card title="即将到期合同" size="small">
            <Table
              rowKey="id"
              size="small"
              pagination={false}
              dataSource={expiring_contracts}
              locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="无" /> }}
              columns={[
                { title: '合同', dataIndex: 'code' },
                { title: '客户', dataIndex: 'customer' },
                { title: '到期', dataIndex: 'expire_date' },
                {
                  title: '状态',
                  dataIndex: 'status',
                  render: (s: string) => <Tag color={CONTRACT_STATUS[s]?.color}>{CONTRACT_STATUS[s]?.label || s}</Tag>,
                },
              ]}
            />
          </Card>
        </Col>
        <Col span={8}>
          <Card title="逾期回款" size="small">
            <Table
              rowKey="id"
              size="small"
              pagination={false}
              dataSource={overdue_plans}
              locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="无" /> }}
              columns={[
                { title: '合同', dataIndex: 'contract_code' },
                { title: '期次', dataIndex: 'period_no' },
                { title: '到期', dataIndex: 'due_date' },
                {
                  title: '未收',
                  key: 'out',
                  render: (_: unknown, r: { outstanding_cent: number }) => (
                    <span style={{ color: '#cf1322' }}>{fenToYuan(r.outstanding_cent)}</span>
                  ),
                },
              ]}
            />
          </Card>
        </Col>
        <Col span={8}>
          <Card title="近期赢单" size="small">
            <Table
              rowKey="id"
              size="small"
              pagination={false}
              dataSource={recent_won_deals}
              locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="无" /> }}
              columns={[
                { title: '商单', dataIndex: 'code' },
                { title: '客户', dataIndex: 'customer' },
                {
                  title: '金额',
                  dataIndex: 'amount_cent',
                  render: (c: number) => fenToYuan(c),
                },
                {
                  title: '状态',
                  dataIndex: 'status',
                  render: (s: string) => <Tag color={DEAL_STAGES[s]?.color}>{DEAL_STAGES[s]?.label || s}</Tag>,
                },
              ]}
            />
          </Card>
        </Col>
      </Row>
    </div>
  )
}
