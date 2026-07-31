import { useQuery } from '@tanstack/react-query'
import { Card, Empty, Progress, Spin, Typography } from 'antd'
import { apiLostDealAnalysis } from '../api'

const REASON_LABELS: Record<string, string> = {
  no_purchase: '无购买意向',
  competitor: '竞争对手',
  budget: '预算不足',
  qualified_out: '不符合资质',
  other: '其他',
}

function Bar({ label, count, max }: { label: string; count: number; max: number }) {
  const pct = max > 0 ? Math.round((count / max) * 100) : 0
  return (
    <div style={{ marginBottom: 12 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between' }}>
        <span>{label}</span>
        <span>{count}</span>
      </div>
      <Progress percent={pct} showInfo={false} />
    </div>
  )
}

export default function LostDealAnalysis() {
  const { data, isLoading } = useQuery({ queryKey: ['lost-analysis'], queryFn: apiLostDealAnalysis })
  if (isLoading) return <Spin style={{ margin: 40 }} />
  if (!data || data.total === 0) return <Empty description="暂无输单商单" style={{ marginTop: 80 }} />

  const reasonMax = Math.max(1, ...data.by_reason.map((r) => r.count))
  const ownerMax = Math.max(1, ...data.by_owner.map((o) => o.count))
  const monthMax = Math.max(1, ...data.by_month.map((m) => m.count))

  return (
    <div style={{ padding: 24 }}>
      <h2>商单输单分析</h2>
      <Typography.Paragraph type="secondary">
        共 {data.total} 个输单商单，按输单原因 / 负责人 / 月份分布如下。
      </Typography.Paragraph>
      <div style={{ display: 'flex', gap: 16, flexWrap: 'wrap' }}>
        <Card title="按输单原因" style={{ flex: 1, minWidth: 280 }}>
          {data.by_reason.map((r) => (
            <Bar key={r.key} label={REASON_LABELS[r.key] || r.key} count={r.count} max={reasonMax} />
          ))}
        </Card>
        <Card title="按负责人" style={{ flex: 1, minWidth: 280 }}>
          {data.by_owner.map((o) => (
            <Bar key={o.owner_id} label={o.name || `员工${o.owner_id}`} count={o.count} max={ownerMax} />
          ))}
        </Card>
        <Card title="按月份" style={{ flex: 1, minWidth: 280 }}>
          {data.by_month.map((m) => (
            <Bar key={m.key} label={m.key} count={m.count} max={monthMax} />
          ))}
        </Card>
      </div>
    </div>
  )
}
