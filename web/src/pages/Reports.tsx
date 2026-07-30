import { useState } from 'react'
import { Tabs, Table, Button, Space, Card, App, Tag, Progress } from 'antd'
import { DownloadOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  apiSignTrend, apiPaymentTrend, apiSalesRank, apiFunnel, apiExportReport,
  REPORT_LABEL, fenToYuan,
  // 以下为类型，运行时为 undefined（仅作类型标注，编译后擦除，无妨）
  MonthPoint, SalesRankRow, FunnelRow, ReportType,
} from '../api'

// 内联迷你条形（按列最大值归一化宽度）
function Bar({ value, max }: { value: number; max: number }) {
  const pct = max > 0 ? Math.max(2, Math.round((value / max) * 100)) : 0
  return (
    <div style={{ background: '#f0f5ff', borderRadius: 4, height: 18, width: '100%' }}>
      <div style={{ background: '#1677ff', borderRadius: 4, height: 18, width: `${pct}%` }} />
    </div>
  )
}

export default function Reports() {
  const { message } = App.useApp()
  const qc = useQueryClient()
  const [active, setActive] = useState<ReportType>('sign_trend')

  const { data: sign, isLoading: ls } = useQuery({ queryKey: ['report', 'sign'], queryFn: () => apiSignTrend(12) })
  const { data: pay, isLoading: lp } = useQuery({ queryKey: ['report', 'pay'], queryFn: () => apiPaymentTrend(12) })
  const { data: rank, isLoading: lr } = useQuery({ queryKey: ['report', 'rank'], queryFn: apiSalesRank })
  const { data: funnel, isLoading: lf } = useQuery({ queryKey: ['report', 'funnel'], queryFn: apiFunnel })

  const exportMut = useMutation({
    mutationFn: (type: ReportType) => apiExportReport(type),
    onSuccess: (blob, type) => {
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `${type}.csv`
      document.body.appendChild(a)
      a.click()
      a.remove()
      URL.revokeObjectURL(url)
      message.success('已导出 CSV')
    },
    onError: () => message.error('导出失败'),
  })

  const signMax = Math.max(0, ...(sign?.rows.map((r) => r.amount_cent) || []))
  const payMax = Math.max(0, ...(pay?.rows.map((r) => r.amount_cent) || []))
  const rankMax = Math.max(0, ...(rank?.rows.map((r) => r.signed_cent) || []))
  const funnelMax = Math.max(0, ...(funnel?.rows.map((r) => r.count) || []))

  const signCols = [
    { title: '月份', dataIndex: 'month', key: 'month' },
    { title: '签约金额(元)', key: 'amount', render: (_: any, r: MonthPoint) => `¥${fenToYuan(r.amount_cent)}` },
    { title: '', key: 'bar', width: 160, render: (_: any, r: MonthPoint) => <Bar value={r.amount_cent} max={signMax} /> },
  ]
  const payCols = [
    { title: '月份', dataIndex: 'month', key: 'month' },
    { title: '回款金额(元)', key: 'amount', render: (_: any, r: MonthPoint) => `¥${fenToYuan(r.amount_cent)}` },
    { title: '', key: 'bar', width: 160, render: (_: any, r: MonthPoint) => <Bar value={r.amount_cent} max={payMax} /> },
  ]
  const rankCols = [
    { title: '销售', dataIndex: 'owner_name', key: 'owner_name' },
    { title: '赢单数', dataIndex: 'won_deals', key: 'won_deals' },
    { title: '签约金额(元)', key: 'signed', render: (_: any, r: SalesRankRow) => `¥${fenToYuan(r.signed_cent)}` },
    { title: '回款金额(元)', key: 'paid', render: (_: any, r: SalesRankRow) => `¥${fenToYuan(r.paid_cent)}` },
    { title: '', key: 'bar', width: 160, render: (_: any, r: SalesRankRow) => <Bar value={r.signed_cent} max={rankMax} /> },
  ]
  const funnelCols = [
    { title: '阶段', dataIndex: 'label', key: 'label', render: (l: string, r: FunnelRow) => <Tag color={r.stage === 'won' ? 'success' : 'blue'}>{l}</Tag> },
    { title: '数量', dataIndex: 'count', key: 'count' },
    { title: '金额(元)', key: 'amount', render: (_: any, r: FunnelRow) => `¥${fenToYuan(r.amount_cent)}` },
    { title: '', key: 'bar', width: 160, render: (_: any, r: FunnelRow) => <Bar value={r.count} max={funnelMax} /> },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Space style={{ marginBottom: 16, justifyContent: 'space-between', width: '100%' }}>
        <span>报表中心（数据范围按您的权限过滤）</span>
        <Button icon={<DownloadOutlined />} loading={exportMut.isPending} onClick={() => exportMut.mutate(active)}>
          导出当前报表 CSV
        </Button>
      </Space>

      <Card>
        <Tabs
          activeKey={active}
          onChange={(k) => setActive(k as ReportType)}
          items={[
            {
              key: 'sign_trend',
              label: REPORT_LABEL.sign_trend,
              children: (
                <Table rowKey="month" loading={ls} dataSource={sign?.rows || []} columns={signCols} pagination={false} />
              ),
            },
            {
              key: 'payment_trend',
              label: REPORT_LABEL.payment_trend,
              children: (
                <Table rowKey="month" loading={lp} dataSource={pay?.rows || []} columns={payCols} pagination={false} />
              ),
            },
            {
              key: 'sales_rank',
              label: REPORT_LABEL.sales_rank,
              children: (
                <Table rowKey="owner_id" loading={lr} dataSource={rank?.rows || []} columns={rankCols} pagination={false} />
              ),
            },
            {
              key: 'funnel',
              label: REPORT_LABEL.funnel,
              children: (
                <Table rowKey="stage" loading={lf} dataSource={funnel?.rows || []} columns={funnelCols} pagination={false} />
              ),
            },
          ]}
        />
      </Card>
    </div>
  )
}
