import { useState } from 'react'
import { Table, Tag, Button, Space, Modal, Form, Select, InputNumber, Input, App, Popconfirm } from 'antd'
import { PlusOutlined, CheckOutlined, StopOutlined, DeleteOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  apiListInvoices, apiCreateInvoice, apiIssueInvoice, apiVoidInvoice, apiDeleteInvoice,
  apiListContracts, apiMe, Invoice, InvoiceInput, INVOICE_STATUS, fenToYuan,
} from '../api'

const CAN_MANAGE = ['admin', 'finance', 'sales_lead']

export default function Invoices() {
  const { message } = App.useApp()
  const qc = useQueryClient()
  const [creating, setCreating] = useState(false)
  const [form] = Form.useForm()

  const { data: me } = useQuery({ queryKey: ['me'], queryFn: apiMe })
  const role = me?.role
  const canManage = !!role && CAN_MANAGE.includes(role)

  const { data, isLoading } = useQuery({
    queryKey: ['invoices'],
    queryFn: () => apiListInvoices({ page: 1, size: 100 }),
  })
  const { data: contracts } = useQuery({
    queryKey: ['contracts', 'signed', 'invoice'],
    queryFn: () => apiListContracts({ status: 'signed', page: 1, size: 100 }),
  })

  const createMut = useMutation({
    mutationFn: (v: InvoiceInput) => apiCreateInvoice(v),
    onSuccess: () => { message.success('已创建待开发票'); setCreating(false); form.resetFields(); qc.invalidateQueries({ queryKey: ['invoices'] }) },
    onError: (e: any) => message.error(e?.response?.data?.message || '创建失败'),
  })
  const issueMut = useMutation({
    mutationFn: (id: string) => apiIssueInvoice(id),
    onSuccess: () => { message.success('已开票'); qc.invalidateQueries({ queryKey: ['invoices'] }) },
    onError: (e: any) => message.error(e?.response?.data?.message || '操作失败'),
  })
  const voidMut = useMutation({
    mutationFn: (id: string) => apiVoidInvoice(id),
    onSuccess: () => { message.success('已作废'); qc.invalidateQueries({ queryKey: ['invoices'] }) },
    onError: (e: any) => message.error(e?.response?.data?.message || '操作失败'),
  })
  const delMut = useMutation({
    mutationFn: (id: string) => apiDeleteInvoice(id),
    onSuccess: () => { message.success('已删除'); qc.invalidateQueries({ queryKey: ['invoices'] }) },
    onError: (e: any) => message.error(e?.response?.data?.message || '删除失败'),
  })

  const columns = [
    { title: '单号', dataIndex: 'code', key: 'code' },
    {
      title: '合同', key: 'contract',
      render: (_: any, r: Invoice) =>
        r.contract ? `${r.contract.code} ${r.contract.title}` : `#${r.contract_id}`,
    },
    { title: '金额(元)', key: 'amount', render: (_: any, r: Invoice) => `¥${fenToYuan(r.amount_cent)}` },
    {
      title: '状态', dataIndex: 'status', key: 'status',
      render: (s: string) => { const m = INVOICE_STATUS[s as keyof typeof INVOICE_STATUS]; return <Tag color={m?.color}>{m?.label}</Tag> },
    },
    { title: '开票日期', dataIndex: 'issued_at', key: 'issued_at', render: (v: string) => v || '-' },
    { title: '备注', dataIndex: 'remark', key: 'remark', render: (v: string) => v || '-' },
    {
      title: '操作', key: 'op', render: (_: any, r: Invoice) => {
        if (!canManage) return '-'
        return (
          <Space>
            {r.status === 'draft' && <Button type="link" icon={<CheckOutlined />} onClick={() => issueMut.mutate(r.id)}>开票</Button>}
            {r.status !== 'voided' && <Button type="link" icon={<StopOutlined />} onClick={() => voidMut.mutate(r.id)}>作废</Button>}
            {r.status === 'draft' && (
              <Popconfirm title="确认删除该待开发票？" onConfirm={() => delMut.mutate(r.id)}>
                <Button type="link" danger icon={<DeleteOutlined />}>删除</Button>
              </Popconfirm>
            )}
          </Space>
        )
      },
    },
  ]

  return (
    <div style={{ padding: 24 }}>
      <Space style={{ marginBottom: 16, justifyContent: 'space-between', width: '100%' }}>
        <span>开票管理（累计金额受合同额约束）</span>
        {canManage && <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreating(true)}>新建发票</Button>}
      </Space>

      <Table rowKey="id" loading={isLoading} dataSource={data?.list || []} columns={columns} pagination={false} />

      <Modal title="新建发票" open={creating} onCancel={() => setCreating(false)} okText="创建"
        onOk={() => form.submit()} confirmLoading={createMut.isPending}>
        <Form form={form} layout="vertical" onFinish={(v) => {
          createMut.mutate({
            contract_id: String(v.contract_id),
            amount_cent: Math.round((v.amount_cent || 0) * 100),
            remark: v.remark,
          })
        }}>
          <Form.Item name="contract_id" label="关联合同（仅显示已签约）" rules={[{ required: true, message: '请选择合同' }]}>
            <Select showSearch optionFilterProp="label" placeholder="选择已签约合同"
              options={(contracts?.list || []).map((c: any) => ({ value: c.id, label: `${c.code} ${c.title}（¥${fenToYuan(c.amount_cent)}）` }))} />
          </Form.Item>
          <Form.Item name="amount_cent" label="开票金额（元）" rules={[{ required: true, message: '请输入金额' }]}>
            <InputNumber min={0} step={0.01} style={{ width: '100%' }} addonAfter="元" />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={2} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
