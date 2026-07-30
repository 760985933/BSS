import { useState } from 'react'
import { Table, Tag, Select, Input, DatePicker, Button, Space, Card, App } from 'antd'
import { SearchOutlined } from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { apiListAuditLogs, AUDIT_ACTION } from '../api'
import type { AuditLog } from '../api'
import type { Dayjs } from 'dayjs'

const { RangePicker } = DatePicker

const entityOptions = [
  { value: 'customer', label: '客户' },
  { value: 'deal', label: '商单' },
  { value: 'contract', label: '合同' },
  { value: 'payment_plan', label: '回款计划' },
  { value: 'payment_record', label: '回款记录' },
  { value: 'invoice', label: '发票' },
  { value: 'approval', label: '审批' },
  { value: 'employee', label: '员工' },
]

export default function Audit() {
  const { message } = App.useApp()
  const qc = useQueryClient()
  const [entityType, setEntityType] = useState<string>()
  const [entityId, setEntityId] = useState<string>()
  const [action, setAction] = useState<string>()
  const [range, setRange] = useState<[Dayjs, Dayjs] | null>(null)
  const [page, setPage] = useState(1)
  const size = 20

  const params: Record<string, any> = { page, size }
  if (entityType) params.entity_type = entityType
  if (entityId) params.entity_id = entityId
  if (action) params.action = action
  if (range) {
    params.start = range[0].format('YYYY-MM-DD')
    params.end = range[1].format('YYYY-MM-DD')
  }

  const { data, isLoading } = useQuery({
    queryKey: ['audit', params],
    queryFn: () => apiListAuditLogs(params),
  })

  const columns = [
    { title: '时间', dataIndex: 'created_at', key: 'created_at', width: 200, render: (v: string) => v?.replace('T', ' ').slice(0, 19) },
    {
      title: '操作', dataIndex: 'action', key: 'action', width: 100,
      render: (a: string) => { const m = AUDIT_ACTION[a] || { label: a, color: 'default' }; return <Tag color={m.color}>{m.label}</Tag> },
    },
    {
      title: '实体', key: 'entity', width: 160,
      render: (_: any, r: AuditLog) => {
        const label = entityOptions.find((e) => e.value === r.entity_type)?.label || r.entity_type
        return `${label} #${r.entity_id}`
      },
    },
    { title: '操作人', dataIndex: 'operator_id', key: 'operator_id', width: 90, render: (v: string) => `#${v}` },
    {
      title: '详情', key: 'detail',
      render: (_: any, r: AuditLog) => (
        <div>
          {r.before_json && <div style={{ fontSize: 12, color: '#999' }}>前：<code>{r.before_json.slice(0, 80)}</code></div>}
          {r.after_json && <div style={{ fontSize: 12, color: '#666' }}>后：<code>{r.after_json.slice(0, 80)}</code></div>}
        </div>
      ),
    },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Card style={{ marginBottom: 16 }}>
        <Space wrap>
          <Select placeholder="实体类型" allowClear style={{ width: 140 }} value={entityType}
            onChange={setEntityType} options={entityOptions} />
          <Input placeholder="实体 ID" style={{ width: 120 }} value={entityId}
            onChange={(e) => setEntityId(e.target.value)} allowClear />
          <Select placeholder="操作类型" allowClear style={{ width: 130 }} value={action}
            onChange={setAction}
            options={Object.entries(AUDIT_ACTION).map(([v, m]) => ({ value: v, label: m.label }))} />
          <RangePicker value={range as any} onChange={(v) => setRange(v as any)} />
          <Button type="primary" icon={<SearchOutlined />} onClick={() => { setPage(1); qc.invalidateQueries({ queryKey: ['audit'] }) }}>
            查询
          </Button>
        </Space>
      </Card>

      <Table rowKey="id" loading={isLoading} dataSource={data?.list || []} columns={columns}
        pagination={{
          current: page, pageSize: size, total: data?.total || 0,
          onChange: (p) => setPage(p),
        }} />
    </div>
  )
}
