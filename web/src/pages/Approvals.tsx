import { useState } from 'react'
import { Table, Tag, Button, Space, Modal, Form, Select, InputNumber, Input, App, Popconfirm } from 'antd'
import { PlusOutlined, CheckOutlined, CloseOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  apiListApprovals, apiCreateApproval, apiApproveApproval, apiRejectApproval,
  apiListContracts, apiListDeals, apiMe,
  Approval, ApprovalInput, APPROVAL_KIND, APPROVAL_STATUS, fenToYuan,
} from '../api'

const CAN_SUBMIT = ['admin', 'sales', 'sales_lead']
const CAN_APPROVE = ['admin', 'finance', 'sales_lead']

export default function Approvals() {
  const { message } = App.useApp()
  const qc = useQueryClient()
  const [query, setQuery] = useState<{ status?: string; kind?: string }>({})
  const [creating, setCreating] = useState(false)
  const [rejecting, setRejecting] = useState<Approval | null>(null)
  const [rejectReason, setRejectReason] = useState('')
  const [form] = Form.useForm()

  const { data: me } = useQuery({ queryKey: ['me'], queryFn: apiMe })
  const role = me?.role
  const canSubmit = !!role && CAN_SUBMIT.includes(role)
  const canApprove = !!role && CAN_APPROVE.includes(role)

  const { data, isLoading } = useQuery({
    queryKey: ['approvals', query],
    queryFn: () => apiListApprovals({ page: 1, size: 100, status: query.status as any, kind: query.kind as any }),
  })

  const approveMut = useMutation({
    mutationFn: (id: string) => apiApproveApproval(id),
    onSuccess: () => { message.success('已通过'); qc.invalidateQueries({ queryKey: ['approvals'] }) },
    onError: (e: any) => message.error(e?.response?.data?.message || '操作失败'),
  })
  const rejectMut = useMutation({
    mutationFn: ({ id, reason }: { id: string; reason: string }) => apiRejectApproval(id, reason),
    onSuccess: () => { message.success('已驳回'); setRejecting(null); setRejectReason(''); qc.invalidateQueries({ queryKey: ['approvals'] }) },
    onError: (e: any) => message.error(e?.response?.data?.message || '操作失败'),
  })
  const createMut = useMutation({
    mutationFn: (v: ApprovalInput) => apiCreateApproval(v),
    onSuccess: () => { message.success('已提交审批'); setCreating(false); form.resetFields(); qc.invalidateQueries({ queryKey: ['approvals'] }) },
    onError: (e: any) => message.error(e?.response?.data?.message || '提交失败'),
  })

  const kind = Form.useWatch('kind', form)
  const { data: pendingContracts } = useQuery({
    queryKey: ['contracts', 'pending', 'approval'],
    queryFn: () => apiListContracts({ status: 'pending', page: 1, size: 100 }),
    enabled: creating,
  })
  const { data: negotiatingDeals } = useQuery({
    queryKey: ['deals', 'negotiating', 'approval'],
    queryFn: () => apiListDeals({ status: 'negotiating', page: 1, size: 100 }),
    enabled: creating,
  })

  const columns = [
    { title: '单号', dataIndex: 'code', key: 'code' },
    { title: '类型', dataIndex: 'kind', key: 'kind', render: (k: string) => APPROVAL_KIND[k as keyof typeof APPROVAL_KIND] || k },
    { title: '对象', key: 'entity', render: (_: any, r: Approval) => `${r.entity_type === 'contract' ? '合同' : '商单'} #${r.entity_id}` },
    {
      title: '折扣金额', key: 'amount', render: (_: any, r: Approval) =>
        r.kind === 'deal_discount' ? `¥${fenToYuan(r.amount_cent)}` : '-',
    },
    {
      title: '状态', dataIndex: 'status', key: 'status',
      render: (s: string) => { const m = APPROVAL_STATUS[s as keyof typeof APPROVAL_STATUS]; return <Tag color={m?.color}>{m?.label}</Tag> },
    },
    { title: '备注', dataIndex: 'note', key: 'note', render: (v: string) => v || '-' },
    {
      title: '操作', key: 'op', render: (_: any, r: Approval) =>
        r.status === 'pending' && canApprove ? (
          <Space>
            <Button type="link" icon={<CheckOutlined />} onClick={() => approveMut.mutate(r.id)}>通过</Button>
            <Button type="link" danger icon={<CloseOutlined />} onClick={() => setRejecting(r)}>驳回</Button>
          </Space>
        ) : '-',
    },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Space style={{ marginBottom: 16, justifyContent: 'space-between', width: '100%' }}>
        <Space>
          <Select
            allowClear placeholder="按状态" style={{ width: 140 }}
            value={query.status} onChange={(v) => setQuery((q) => ({ ...q, status: v }))}
            options={[
              { value: 'pending', label: '待审批' },
              { value: 'approved', label: '已通过' },
              { value: 'rejected', label: '已驳回' },
            ]}
          />
          <Select
            allowClear placeholder="按类型" style={{ width: 140 }}
            value={query.kind} onChange={(v) => setQuery((q) => ({ ...q, kind: v }))}
            options={[
              { value: 'contract_sign', label: '合同签约' },
              { value: 'deal_discount', label: '商单折扣' },
            ]}
          />
        </Space>
        {canSubmit && <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreating(true)}>提交审批</Button>}
      </Space>

      <Table
        rowKey="id"
        loading={isLoading}
        dataSource={data?.list || []}
        columns={columns}
        pagination={false}
      />

      <Modal title="提交审批" open={creating} onCancel={() => setCreating(false)} okText="提交"
        onOk={() => form.submit()} confirmLoading={createMut.isPending}>
        <Form form={form} layout="vertical" onFinish={(v) => {
          const input: ApprovalInput = {
            entity_type: v.kind === 'contract_sign' ? 'contract' : 'deal',
            entity_id: String(v.entity_id),
            kind: v.kind,
            note: v.note,
          }
          if (v.kind === 'deal_discount') input.amount_cent = v.amount_cent || 0
          createMut.mutate(input)
        }}>
          <Form.Item name="kind" label="审批类型" rules={[{ required: true }]}>
            <Select
              options={[
                { value: 'contract_sign', label: '合同签约审批（待签合同）' },
                { value: 'deal_discount', label: '商单折扣审批（谈判中商单）' },
              ]}
            />
          </Form.Item>
          {kind === 'contract_sign' && (
            <Form.Item name="entity_id" label="待签合同" rules={[{ required: true, message: '请选择合同' }]}>
              <Select
                showSearch optionFilterProp="label" placeholder="选择 pending 状态合同"
                options={(pendingContracts?.list || []).map((c: any) => ({ value: c.id, label: `${c.code} ${c.title}` }))}
              />
            </Form.Item>
          )}
          {kind === 'deal_discount' && (
            <>
              <Form.Item name="entity_id" label="谈判中商单" rules={[{ required: true, message: '请选择商单' }]}>
                <Select
                  showSearch optionFilterProp="label" placeholder="选择 negotiating 状态商单"
                  options={(negotiatingDeals?.list || []).map((d: any) => ({ value: d.id, label: `${d.code} ${d.title}` }))}
                />
              </Form.Item>
              <Form.Item name="amount_cent" label="折扣金额（元，分单位内部换算）" rules={[{ required: true }]}>
                <InputNumber min={0} style={{ width: '100%' }} addonAfter="元"
                  onChange={(v) => form.setFieldValue('amount_cent', Math.round((v || 0) * 100))} />
              </Form.Item>
            </>
          )}
          <Form.Item name="note" label="说明">
            <Input.TextArea rows={2} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal title="驳回审批" open={!!rejecting} onCancel={() => setRejecting(null)} okText="确认驳回"
        okButtonProps={{ danger: true }} onOk={() => rejectMut.mutate({ id: rejecting!.id, reason: rejectReason })}>
        <Input.TextArea rows={3} placeholder="请填写驳回原因" value={rejectReason}
          onChange={(e) => setRejectReason(e.target.value)} />
      </Modal>
    </div>
  )
}
