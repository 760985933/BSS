import { useState } from 'react'
import type { ReactNode } from 'react'
import {
  Drawer, Table, Tag, Typography, Button, Space, Modal, Form, InputNumber, DatePicker,
  Select, Input, Popconfirm, App, Statistic,
} from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import dayjs, { Dayjs } from 'dayjs'
import {
  apiListPlans, apiCreatePlan, apiUpdatePlan, apiDeletePlan,
  apiListRecords, apiCreateRecords, apiDeleteRecord, apiPaymentSummary, apiMe,
  PLAN_STATUS, PAY_METHODS, fenToYuan, Contract, PaymentPlan, PaymentRecord, PaymentSummary,
} from '../api'

interface Props {
  contract: Contract | null
  open: boolean
  onClose: () => void
}

export default function PaymentCenter({ contract, open, onClose }: Props) {
  const { message } = App.useApp()
  const qc = useQueryClient()
  const [planModal, setPlanModal] = useState<{ open: boolean; editing?: PaymentPlan }>({ open: false })
  const [recModal, setRecModal] = useState(false)
  const [planForm] = Form.useForm()
  const [recForm] = Form.useForm()

  const baseKeys = ['payments', contract?.id]
  const { data: plans } = useQuery({ queryKey: [...baseKeys, 'plans'], queryFn: () => apiListPlans(contract!.id), enabled: !!contract })
  const { data: records } = useQuery({ queryKey: [...baseKeys, 'records'], queryFn: () => apiListRecords(contract!.id), enabled: !!contract })
  const { data: summary } = useQuery({ queryKey: [...baseKeys, 'summary'], queryFn: () => apiPaymentSummary(contract!.id), enabled: !!contract })

  const invalidate = () => qc.invalidateQueries({ queryKey: baseKeys })

  const openPlan = (p?: PaymentPlan) => {
    if (p) planForm.setFieldsValue({ period_no: p.period_no, due_date: dayjs(p.due_date), amount: p.amount_cent / 100 })
    else planForm.resetFields()
    setPlanModal({ open: true, editing: p })
  }
  const openRec = () => { recForm.resetFields(); setRecModal(true) }

  return (
    <Drawer title={contract ? `回款管理 · ${contract.code} ${contract.title}` : '回款管理'} width={780} open={open} onClose={onClose}>
      {contract && (
        <Space direction="vertical" style={{ width: '100%' }} size="large">
          <div>
            <Typography.Text type="secondary">客户：</Typography.Text>
            <Typography.Text>{contract.customer?.name || '-'}</Typography.Text>
            <Typography.Text type="secondary" style={{ marginLeft: 16 }}>合同额：</Typography.Text>
            <Typography.Text>¥{fenToYuan(contract.amount_cent)}</Typography.Text>
          </div>

          <Space size="large" wrap>
            <Statistic title="应收" value={fenToYuan((summary as PaymentSummary)?.receivable_cent ?? 0)} prefix="¥" />
            <Statistic title="已收" value={fenToYuan((summary as PaymentSummary)?.received_cent ?? 0)} prefix="¥" valueStyle={{ color: '#3f8600' }} />
            <Statistic title="余额" value={fenToYuan((summary as PaymentSummary)?.balance_cent ?? 0)} prefix="¥" />
            <Statistic title="逾期额" value={fenToYuan((summary as PaymentSummary)?.overdue_cent ?? 0)} prefix="¥" valueStyle={{ color: '#cf1322' }} />
          </Space>

          <PaymentPlanSection contract={contract} plans={plans || []} onMutate={invalidate} openPlan={openPlan} />
          <PaymentRecordSection contract={contract} plans={plans || []} records={records || []} onMutate={invalidate} openRec={openRec} />
        </Space>
      )}

      <Modal
        title={planModal.editing ? '编辑回款计划' : '新增回款计划'}
        open={planModal.open}
        onCancel={() => setPlanModal({ open: false })}
        onOk={() => planForm.validateFields().then((v: { period_no: number; due_date: Dayjs; amount: number }) => {
          const payload = {
            period_no: v.period_no,
            due_date: v.due_date.format('YYYY-MM-DD'),
            amount_cent: Math.round((v.amount || 0) * 100),
          }
          const fn = planModal.editing
            ? apiUpdatePlan(contract!.id, planModal.editing.id, payload)
            : apiCreatePlan(contract!.id, payload)
          fn.then(() => { message.success('已保存'); setPlanModal({ open: false }); invalidate() })
            .catch(() => { /* 拦截器已提示 */ })
        })}
        okText="保存" cancelText="取消"
      >
        <Form form={planForm} layout="vertical">
          <Form.Item name="period_no" label="期次" rules={[{ required: true, message: '请输入期次' }]}>
            <InputNumber min={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="due_date" label="到期日" rules={[{ required: true, message: '请选择到期日' }]}>
            <DatePicker style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="amount" label="计划金额（元）" rules={[{ required: true, message: '请输入金额' }]}>
            <InputNumber min={0} precision={2} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="录入回款"
        open={recModal}
        onCancel={() => setRecModal(false)}
        onOk={() => recForm.validateFields().then((v: { plan_id?: string; amount: number; paid_at: Dayjs; method?: string; remark?: string }) => {
          const rec = {
            plan_id: v.plan_id ? v.plan_id : null,
            amount_cent: Math.round((v.amount || 0) * 100),
            paid_at: v.paid_at.format('YYYY-MM-DD'),
            method: v.method || '',
            remark: v.remark || '',
          }
          apiCreateRecords(contract!.id, [rec])
            .then(() => { message.success('已录入回款'); setRecModal(false); invalidate() })
            .catch(() => { /* 拦截器已提示 */ })
        })}
        okText="确认录入" cancelText="取消"
      >
        <Form form={recForm} layout="vertical">
          <Form.Item name="plan_id" label="核销期次" tooltip="选择要核销的回款计划，或选「不核销计划」登记一笔通用回款">
            <Select allowClear placeholder="不核销计划"
              options={[
                { value: '', label: '不核销计划' },
                ...(plans || []).map((p) => ({
                  value: p.id,
                  label: `第${p.period_no}期 · ${fenToYuan(p.amount_cent)} · ${PLAN_STATUS[p.status]?.label || p.status}`,
                })),
              ]} />
          </Form.Item>
          <Form.Item name="amount" label="回款金额（元）" rules={[{ required: true, message: '请输入金额' }]}>
            <InputNumber min={0} precision={2} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="paid_at" label="收款日期" rules={[{ required: true, message: '请选择收款日期' }]}>
            <DatePicker style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="method" label="收款方式">
            <Select allowClear placeholder="请选择" options={PAY_METHODS} />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={2} />
          </Form.Item>
        </Form>
      </Modal>
    </Drawer>
  )
}

function PaymentPlanSection({ contract, plans, onMutate, openPlan }: {
  contract: Contract
  plans: PaymentPlan[]
  onMutate: () => void
  openPlan: (p?: PaymentPlan) => void
}) {
  const { message } = App.useApp()
  const delMut = useMutation({
    mutationFn: (id: string) => apiDeletePlan(contract.id, id),
    onSuccess: () => { message.success('已删除'); onMutate() },
  })

  return (
    <div>
      <Space style={{ width: '100%', justifyContent: 'space-between', marginBottom: 8 }}>
        <Typography.Title level={5} style={{ margin: 0 }}>回款计划</Typography.Title>
        <PlanManageButton contract={contract} onClick={() => openPlan(undefined)} />
      </Space>
      <Table<PaymentPlan>
        rowKey="id" size="small" dataSource={plans} pagination={false}
        columns={[
          { title: '期次', dataIndex: 'period_no', width: 70 },
          {
            title: '到期日', dataIndex: 'due_date', width: 130,
            render: (v: string, p) => (
              <span>{v}{p.is_overdue && <Tag color="red" style={{ marginLeft: 6 }}>逾期</Tag>}</span>
            ),
          },
          { title: '金额(元)', dataIndex: 'amount_cent', align: 'right' as const, render: (v) => fenToYuan(v) },
          {
            title: '状态', dataIndex: 'status', width: 100,
            render: (v: string) => <Tag color={PLAN_STATUS[v]?.color}>{PLAN_STATUS[v]?.label || v}</Tag>,
          },
          {
            title: '操作', key: 'op', width: 130,
            render: (_, p) => (
              <PlanManageButton contract={contract}>
                {(canManage) => canManage && (
                  <Space size="small">
                    <Button size="small" type="link" icon={<EditOutlined />} disabled={p.status !== 'pending'} onClick={() => openPlan(p)}>编辑</Button>
                    <Popconfirm title="确认删除该计划？" onConfirm={() => delMut.mutate(p.id)}>
                      <Button size="small" type="link" danger disabled={p.status !== 'pending'}>删除</Button>
                    </Popconfirm>
                  </Space>
                )}
              </PlanManageButton>
            ),
          },
        ]}
      />
    </div>
  )
}

// PlanManageButton 仅非财务角色（销售/主管/admin）可管理计划
function PlanManageButton({ contract, children, onClick }: {
  contract: Contract
  children?: (canManage: boolean) => ReactNode
  onClick?: () => void
}) {
  const { data: me } = useQuery({ queryKey: ['me'], queryFn: apiMe })
  const canManage = me?.role === 'admin' || me?.role === 'sales' || me?.role === 'sales_lead'
  if (!canManage) return null
  if (children) return <>{children(canManage)}</>
  return <Button type="primary" icon={<PlusOutlined />} size="small" onClick={onClick}>新增计划</Button>
}

function PaymentRecordSection({ contract, plans, records, onMutate, openRec }: {
  contract: Contract
  plans: PaymentPlan[]
  records: PaymentRecord[]
  onMutate: () => void
  openRec: () => void
}) {
  const { message } = App.useApp()
  const delMut = useMutation({
    mutationFn: (id: string) => apiDeleteRecord(contract.id, id),
    onSuccess: () => { message.success('已删除'); onMutate() },
  })

  const planMap = new Map(plans.map((p) => [p.id, p]))
  const methodLabel = (v: string) => PAY_METHODS.find((m) => m.value === v)?.label || v || '-'

  return (
    <div>
      <Space style={{ width: '100%', justifyContent: 'space-between', marginBottom: 8 }}>
        <Typography.Title level={5} style={{ margin: 0 }}>回款记录</Typography.Title>
        <RecordButton contract={contract} onClick={openRec} />
      </Space>
      <Table<PaymentRecord>
        rowKey="id" size="small" dataSource={records} pagination={false}
        columns={[
          {
            title: '核销期次', width: 90,
            render: (_, r) => (r.plan_id && planMap.get(r.plan_id) ? `第${planMap.get(r.plan_id)!.period_no}期` : '不核销'),
          },
          { title: '金额(元)', dataIndex: 'amount_cent', align: 'right' as const, render: (v) => fenToYuan(v) },
          { title: '收款日', dataIndex: 'paid_at', width: 120 },
          { title: '方式', dataIndex: 'method', width: 100, render: (v) => methodLabel(v) },
          { title: '备注', dataIndex: 'remark', ellipsis: true },
          {
            title: '操作', key: 'op', width: 80,
            render: (_, r) => (
              <RecordButton contract={contract}>
                {(canRec) => canRec && (
                  <Popconfirm title="确认删除该回款记录？" onConfirm={() => delMut.mutate(r.id)}>
                    <Button size="small" type="link" danger icon={<DeleteOutlined />}>删除</Button>
                  </Popconfirm>
                )}
              </RecordButton>
            ),
          },
        ]}
      />
    </div>
  )
}

// RecordButton 仅财务/admin 可录入/删除回款
function RecordButton({ contract, children, onClick }: {
  contract: Contract
  children?: (canRec: boolean) => ReactNode
  onClick?: () => void
}) {
  const { data: me } = useQuery({ queryKey: ['me'], queryFn: apiMe })
  const canRec = me?.role === 'admin' || me?.role === 'finance'
  if (!canRec) return null
  if (children) return <>{children(canRec)}</>
  return <Button type="primary" icon={<PlusOutlined />} size="small" onClick={onClick}>录入回款</Button>
}
