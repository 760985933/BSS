import { useEffect, useState } from 'react'
import {
  Tabs, Table, Tag, Button, Space, Modal, Form, Input, InputNumber, Select, Popconfirm, App, DatePicker,
} from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import dayjs from 'dayjs'
import {
  apiListLaborContracts, apiCreateLaborContract, apiUpdateLaborContract, apiDeleteLaborContract, apiTransitionContract,
  apiListOnboardings, apiCreateOnboarding, apiUpdateOnboarding, apiDeleteOnboarding,
  apiListEmployees, apiListCandidates,
  LC_STATUS, LC_TYPE, OB_STEP, OB_STATUS,
  LaborContract, Onboarding, LaborContractInput, OnboardingInput, Employee, Candidate,
} from '../api'

const lcTypeOptions = Object.entries(LC_TYPE).map(([v, label]) => ({ value: v, label }))
const stepOptions = [
  { value: 'pending', label: '待办' },
  { value: 'done', label: '完成' },
]
const lcTransitions: Record<string, { to: string; label: string; danger?: boolean }[]> = {
  draft: [{ to: 'active', label: '生效' }],
  active: [
    { to: 'expired', label: '标记到期' },
    { to: 'renewed', label: '标记续签' },
    { to: 'terminated', label: '解除', danger: true },
  ],
}

export default function Hr() {
  const { message } = App.useApp()
  const qc = useQueryClient()
  const [lcOpen, setLcOpen] = useState(false)
  const [lcTarget, setLcTarget] = useState<LaborContract | null>(null)
  const [terminateTarget, setTerminateTarget] = useState<LaborContract | null>(null)
  const [termForm] = Form.useForm()
  const [obOpen, setObOpen] = useState(false)
  const [obTarget, setObTarget] = useState<Onboarding | null>(null)
  const [lcForm] = Form.useForm()
  const [obForm] = Form.useForm()

  const { data: contracts, isLoading: lcLoading } = useQuery({ queryKey: ['labor-contracts'], queryFn: () => apiListLaborContracts() })
  const { data: onboardings, isLoading: obLoading } = useQuery({ queryKey: ['onboardings'], queryFn: () => apiListOnboardings() })
  const { data: emps } = useQuery({ queryKey: ['employees-all'], queryFn: () => apiListEmployees() })
  const { data: candidates } = useQuery({ queryKey: ['candidates-all'], queryFn: () => apiListCandidates() })

  useEffect(() => {
    if (lcOpen) {
      lcForm.resetFields()
      if (lcTarget) {
        lcForm.setFieldsValue({
          employee_id: lcTarget.employee_id,
          type: lcTarget.type,
          start_date: lcTarget.start_date ? dayjs(lcTarget.start_date) : null,
          end_date: lcTarget.end_date ? dayjs(lcTarget.end_date) : null,
          sign_date: lcTarget.sign_date ? dayjs(lcTarget.sign_date) : null,
          probation_months: lcTarget.probation_months,
        })
      }
    }
  }, [lcOpen, lcTarget])

  useEffect(() => {
    if (obOpen) {
      obForm.resetFields()
      if (obTarget) {
        obForm.setFieldsValue({
          employee_id: obTarget.employee_id,
          candidate_id: obTarget.candidate_id,
          step_profile: obTarget.step_profile,
          step_equip: obTarget.step_equip,
          step_training: obTarget.step_training,
          step_probation: obTarget.step_probation,
        })
      }
    }
  }, [obOpen, obTarget])

  const invalidate = () => {
    qc.invalidateQueries({ queryKey: ['labor-contracts'] })
    qc.invalidateQueries({ queryKey: ['onboardings'] })
  }

  const lcMut = useMutation({
    mutationFn: (data: LaborContractInput & { id?: string }) =>
      lcTarget ? apiUpdateLaborContract(lcTarget.id, data) : apiCreateLaborContract(data),
    onSuccess: () => { message.success('已保存'); setLcOpen(false); setLcTarget(null); invalidate() },
    onError: (e: any) => message.error(e?.response?.data?.message || '操作失败'),
  })
  const delLc = useMutation({
    mutationFn: (id: string) => apiDeleteLaborContract(id),
    onSuccess: () => { message.success('已删除'); invalidate() },
  })
  const transMut = useMutation({
    mutationFn: ({ id, to, reason, force }: { id: string; to: string; reason: string; force: boolean }) =>
      apiTransitionContract(id, { to, reason, force }),
    onSuccess: () => { message.success('状态已更新'); setTerminateTarget(null); invalidate() },
    onError: (e: any) => message.error(e?.response?.data?.message || '操作失败'),
  })
  const obMut = useMutation({
    mutationFn: (data: OnboardingInput & { id?: string }) =>
      obTarget ? apiUpdateOnboarding(obTarget.id, data) : apiCreateOnboarding(data),
    onSuccess: () => { message.success('已保存'); setObOpen(false); setObTarget(null); invalidate() },
    onError: (e: any) => message.error(e?.response?.data?.message || '操作失败'),
  })
  const delOb = useMutation({
    mutationFn: (id: string) => apiDeleteOnboarding(id),
    onSuccess: () => { message.success('已删除'); invalidate() },
  })

  const empOptions = (emps?.list || []).map((e: Employee) => ({ value: e.id, label: e.name }))
  const candOptions = (candidates || []).map((c: Candidate) => ({ value: c.id, label: c.name }))

  const onLcSubmit = () => {
    lcForm.validateFields().then((v: any) => {
      lcMut.mutate({
        employee_id: v.employee_id,
        type: v.type,
        start_date: v.start_date ? v.start_date.format('YYYY-MM-DD') : '',
        end_date: v.end_date ? v.end_date.format('YYYY-MM-DD') : '',
        sign_date: v.sign_date ? v.sign_date.format('YYYY-MM-DD') : '',
        probation_months: v.probation_months || 0,
      })
    })
  }
  const onObSubmit = () => {
    obForm.validateFields().then((v: any) => {
      obMut.mutate({
        employee_id: v.employee_id,
        candidate_id: v.candidate_id || undefined,
        step_profile: v.step_profile || 'pending',
        step_equip: v.step_equip || 'pending',
        step_training: v.step_training || 'pending',
        step_probation: v.step_probation || 'pending',
      })
    })
  }

  const lcColumns = [
    { title: '编号', dataIndex: 'code', key: 'code' },
    { title: '员工', key: 'employee', render: (_: any, r: LaborContract) => r.employee?.name || '-' },
    { title: '类型', dataIndex: 'type', key: 'type', render: (t: string) => LC_TYPE[t] || t },
    { title: '期限', key: 'period', render: (_: any, r: LaborContract) => `${r.start_date ? r.start_date.slice(0, 10) : '-'} ~ ${r.end_date ? r.end_date.slice(0, 10) : '无固定'}` },
    { title: '试用期(月)', dataIndex: 'probation_months', key: 'probation_months' },
    { title: '状态', dataIndex: 'status', key: 'status', render: (s: string) => <Tag color={LC_STATUS[s]?.color}>{LC_STATUS[s]?.label}</Tag> },
    {
      title: '操作', key: 'op', render: (_: any, r: LaborContract) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => { setLcTarget(r); setLcOpen(true) }}>编辑</Button>
          {(lcTransitions[r.status] || []).map((t) => (
            <Button size="small" danger={t.danger} key={t.to} onClick={() => {
              if (t.to === 'terminated') setTerminateTarget(r)
              else transMut.mutate({ id: r.id, to: t.to, reason: '', force: false })
            }}>{t.label}</Button>
          ))}
          <Popconfirm title="确认删除？" onConfirm={() => delLc.mutate(r.id)}>
            <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  const obColumns = [
    { title: '编号', dataIndex: 'code', key: 'code' },
    { title: '员工', key: 'employee', render: (_: any, r: Onboarding) => r.employee?.name || '-' },
    { title: '资料', dataIndex: 'step_profile', key: 'step_profile', render: (s: string) => <Tag color={OB_STEP[s]?.color}>{OB_STEP[s]?.label}</Tag> },
    { title: '设备', dataIndex: 'step_equip', key: 'step_equip', render: (s: string) => <Tag color={OB_STEP[s]?.color}>{OB_STEP[s]?.label}</Tag> },
    { title: '培训', dataIndex: 'step_training', key: 'step_training', render: (s: string) => <Tag color={OB_STEP[s]?.color}>{OB_STEP[s]?.label}</Tag> },
    { title: '试用', dataIndex: 'step_probation', key: 'step_probation', render: (s: string) => <Tag color={OB_STEP[s]?.color}>{OB_STEP[s]?.label}</Tag> },
    { title: '状态', dataIndex: 'status', key: 'status', render: (s: string) => <Tag color={OB_STATUS[s]?.color}>{OB_STATUS[s]?.label}</Tag> },
    {
      title: '操作', key: 'op', render: (_: any, r: Onboarding) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => { setObTarget(r); setObOpen(true) }}>编辑</Button>
          <Popconfirm title="确认删除？" onConfirm={() => delOb.mutate(r.id)}>
            <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <Tabs defaultActiveKey="lc" items={[
        {
          key: 'lc', label: '劳动合同',
          children: (
            <div>
              <Space style={{ marginBottom: 12 }}>
                <Button type="primary" icon={<PlusOutlined />} onClick={() => { setLcTarget(null); setLcOpen(true) }}>新建合同</Button>
              </Space>
              <Table rowKey="id" loading={lcLoading} dataSource={contracts || []} columns={lcColumns} size="small" />
            </div>
          ),
        },
        {
          key: 'ob', label: '入职管理',
          children: (
            <div>
              <Space style={{ marginBottom: 12 }}>
                <Button type="primary" icon={<PlusOutlined />} onClick={() => { setObTarget(null); setObOpen(true) }}>新建入职</Button>
              </Space>
              <Table rowKey="id" loading={obLoading} dataSource={onboardings || []} columns={obColumns} size="small" />
            </div>
          ),
        },
      ]} />

      <Modal title={lcTarget ? '编辑劳动合同' : '新建劳动合同'} open={lcOpen} onOk={onLcSubmit}
        confirmLoading={lcMut.isPending} onCancel={() => { setLcOpen(false); setLcTarget(null) }} destroyOnClose>
        <Form form={lcForm} layout="vertical">
          <Form.Item label="员工" name="employee_id" rules={[{ required: true, message: '请选择员工' }]}>
            <Select options={empOptions} showSearch optionFilterProp="label" placeholder="选择员工" />
          </Form.Item>
          <Form.Item label="合同类型" name="type" initialValue="fixed" rules={[{ required: true }]}>
            <Select options={lcTypeOptions} />
          </Form.Item>
          <Form.Item label="开始日期" name="start_date"><DatePicker style={{ width: '100%' }} /></Form.Item>
          <Form.Item label="结束日期" name="end_date"><DatePicker style={{ width: '100%' }} /></Form.Item>
          <Form.Item label="签署日期" name="sign_date"><DatePicker style={{ width: '100%' }} /></Form.Item>
          <Form.Item label="试用期(月)" name="probation_months" initialValue={0}><InputNumber min={0} style={{ width: '100%' }} /></Form.Item>
        </Form>
      </Modal>

      <Modal title="解除劳动合同时需填写原因" open={!!terminateTarget} onOk={() => {
        const reason = termForm.getFieldValue('reason')
        if (!reason || !terminateTarget) { message.warning('请填写解除原因'); return }
        transMut.mutate({ id: terminateTarget.id, to: 'terminated', reason, force: false })
      }} confirmLoading={transMut.isPending} onCancel={() => setTerminateTarget(null)} destroyOnClose>
        <Form form={termForm} layout="vertical">
          <Form.Item label="解除原因" name="reason" rules={[{ required: true, message: '请填写解除原因' }]}>
            <Input.TextArea rows={3} placeholder="如：协商一致解除 / 严重违纪" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal title={obTarget ? '编辑入职' : '新建入职'} open={obOpen} onOk={onObSubmit}
        confirmLoading={obMut.isPending} onCancel={() => { setObOpen(false); setObTarget(null) }} destroyOnClose>
        <Form form={obForm} layout="vertical">
          <Form.Item label="员工" name="employee_id" rules={[{ required: true, message: '请选择员工' }]}>
            <Select options={empOptions} showSearch optionFilterProp="label" placeholder="选择员工" />
          </Form.Item>
          <Form.Item label="来源候选人(可选)" name="candidate_id">
            <Select options={candOptions} showSearch optionFilterProp="label" placeholder="可选关联候选人" allowClear />
          </Form.Item>
          <Form.Item label="资料登记" name="step_profile" initialValue="pending"><Select options={stepOptions} /></Form.Item>
          <Form.Item label="设备领用" name="step_equip" initialValue="pending"><Select options={stepOptions} /></Form.Item>
          <Form.Item label="入职培训" name="step_training" initialValue="pending"><Select options={stepOptions} /></Form.Item>
          <Form.Item label="试用期" name="step_probation" initialValue="pending"><Select options={stepOptions} /></Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
