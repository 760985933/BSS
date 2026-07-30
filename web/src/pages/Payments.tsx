import { useState } from 'react'
import { Table, Tag, Typography, Space, Button, Input, Select } from 'antd'
import { DollarOutlined } from '@ant-design/icons'
import { useQuery } from '@tanstack/react-query'
import { apiListContracts, apiListCustomers, Contract, ContractQuery, CONTRACT_STATUS } from '../api'
import PaymentCenter from '../components/PaymentCenter'

export default function Payments() {
  const [query, setQuery] = useState<ContractQuery>({ page: 1, size: 20 })
  const [openId, setOpenId] = useState<string | null>(null)
  const [openContract, setOpenContract] = useState<Contract | null>(null)

  const { data, isLoading } = useQuery({ queryKey: ['contracts', query], queryFn: () => apiListContracts(query) })
  const { data: customers } = useQuery({ queryKey: ['customers', 'all'], queryFn: () => apiListCustomers({ page: 1, size: 100 }) })

  const openPayment = (c: Contract) => { setOpenContract(c); setOpenId(c.id) }

  return (
    <div>
      <Typography.Title level={4} style={{ marginBottom: 16 }}>回款管理</Typography.Title>

      <Space style={{ marginBottom: 12 }} wrap>
        <Input.Search placeholder="合同标题 / 单号" allowClear style={{ width: 220 }}
          onSearch={(v) => setQuery({ ...query, keyword: v || undefined, page: 1 })} />
        <Select placeholder="状态" allowClear style={{ width: 130 }}
          options={Object.entries(CONTRACT_STATUS).map(([value, v]) => ({ value, label: v.label }))}
          onChange={(v) => setQuery({ ...query, status: v, page: 1 })} />
        <Select placeholder="客户" allowClear style={{ width: 200 }} showSearch optionFilterProp="label"
          options={(customers?.list || []).map((c) => ({ value: c.id, label: c.name }))}
          onChange={(v) => setQuery({ ...query, customer_id: v, page: 1 })} />
      </Space>

      <Table<Contract>
        rowKey="id"
        loading={isLoading}
        dataSource={data?.list || []}
        pagination={{
          current: query.page, pageSize: query.size, total: data?.total || 0,
          showTotal: (t) => `共 ${t} 条`,
          onChange: (page, size) => setQuery({ ...query, page, size }),
        }}
        columns={[
          { title: '单号', dataIndex: 'code', width: 140 },
          { title: '标题', dataIndex: 'title' },
          { title: '客户', dataIndex: ['customer', 'name'], render: (_, c) => c.customer?.name || '-' },
          { title: '合同额(元)', dataIndex: 'amount_cent', align: 'right' as const, render: (v) => (v / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2 }) },
          {
            title: '状态', dataIndex: 'status', width: 100,
            render: (v: string) => <Tag color={CONTRACT_STATUS[v]?.color}>{CONTRACT_STATUS[v]?.label || v}</Tag>,
          },
          { title: '负责人', dataIndex: ['owner', 'name'], width: 90, render: (_, c) => c.owner?.name || '-' },
          {
            title: '操作', key: 'op', width: 100,
            render: (_, c) => (
              <Button size="small" type="link" icon={<DollarOutlined />} onClick={() => openPayment(c)}>回款</Button>
            ),
          },
        ]}
      />

      <PaymentCenter contract={openContract} open={!!openId} onClose={() => setOpenId(null)} />
    </div>
  )
}
