import { useEffect, useState } from 'react'
import {
  Table, Tag, Button, Space, Modal, Form, Input, InputNumber, Select, Popconfirm, App, DatePicker,
} from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, CheckOutlined, DownloadOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import dayjs from 'dayjs'
import {
  apiListPayrolls, apiCreatePayroll, apiGeneratePayrolls, apiUpdatePayroll,
  apiCalcPayroll, apiPayPayroll, apiDeletePayroll, apiExportPayrolls,
  apiListEmployees,
  PAYROLL_STATUS, Payroll, PayrollInput, Employee,
} from '../api'

// 分 → 元 展示
const yuan = (cents = 0) => (cents / 100).toFixed(2)

export default function PayrollPage() {
  const { message } = App.useApp()
  const qc = useQueryClient()
  const [period, setPeriod] = useState<dayjs.Dayjs>(dayjs())
  const [editOpen, setEditOpen] = useState(false)
  const [target, setTarget] = useState<Payroll | null>(null)
  const [form] = Form.useForm()

  const { data: list, isLoading } = useQuery({
    queryKey: ['payrolls', period.format('YYYY-MM')],
    queryFn: () => apiListPayrolls({ period: period.format('YYYY-MM') }),
  })
  const { data: emps } = useQuery({ queryKey: ['employees-all'], queryFn: () => apiListEmployees() })

  useEffect(() => {
    if (editOpen) {
      form.resetFields()
      if (target) {
        form.setFieldsValue({
          employee_id: target.employee_id,
          period: target.period,
          base: target.base_cent / 100,
          bonus: target.bonus_cent / 100,
          deduction: target.deduction_cent / 100,
          social: target.social_cent / 100,
          tax: target.tax_cent / 100,
          remark: target.remark,
        })
      } else {
        form.setFieldsValue({ period: period.format('YYYY-MM') })
      }
    }
  }, [editOpen, target])

  const invalidate = () => {
    qc.invalidateQueries({ queryKey: ['payrolls'] })
  }

  const genMut = useMutation({
    mutationFn: (p: string) => apiGeneratePayrolls(p),
    onSuccess: (r) => { message.success(`已生成 ${r.created} 条薪资（${r.period}）`); invalidate() },
    onError: (e: any) => message.error(e?.response?.data?.message || '生成失败'),
  })
  const saveMut = useMutation({
    mutationFn: (data: PayrollInput & { id?: string }) =>
      target ? apiUpdatePayroll(target.id, data) : apiCreatePayroll(data),
    onSuccess: () => { message.success('已保存'); setEditOpen(false); setTarget(null); invalidate() },
    onError: (e: any) => message.error(e?.response?.data?.message || '操作失败'),
  })
  const calcMut = useMutation({
    mutationFn: (id: string) => apiCalcPayroll(id),
    onSuccess: () => { message.success('已核算'); invalidate() },
    onError: (e: any) => message.error(e?.response?.data?.message || '核算失败'),
  })
  const payMut = useMutation({
    mutationFn: (id: string) => apiPayPayroll(id),
    onSuccess: () => { message.success('已发放'); invalidate() },
    onError: (e: any) => message.error(e?.response?.data?.message || '发放失败'),
  })
  const delMut = useMutation({
    mutationFn: (id: string) => apiDeletePayroll(id),
    onSuccess: () => { message.success('已删除'); invalidate() },
    onError: (e: any) => message.error(e?.response?.data?.message || '删除失败'),
  })
  const exportMut = useMutation({
    mutationFn: (p: string) => apiExportPayrolls(p),
    onSuccess: (r) => {
      const blob = new Blob(['\uFEFF' + r.csv], { type: 'text/csv;charset=utf-8' })
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `工资条_${r.period}.csv`
      a.click()
      URL.revokeObjectURL(url)
      message.success('工资条已导出')
    },
    onError: (e: any) => message.error(e?.response?.data?.message || '导出失败'),
  })

  const empOptions = (emps?.list || []).map((e: Employee) => ({ value: e.id, label: e.name }))

  const onSubmit = () => {
    form.validateFields().then((v: any) => {
      saveMut.mutate({
        employee_id: v.employee_id,
        period: v.period,
        base_cent: Math.round((v.base || 0) * 100),
        bonus_cent: Math.round((v.bonus || 0) * 100),
        deduction_cent: Math.round((v.deduction || 0) * 100),
        social_cent: Math.round((v.social || 0) * 100),
        tax_cent: Math.round((v.tax || 0) * 100),
        remark: v.remark,
      })
    })
  }

  const columns = [
    { title: '员工', key: 'employee', render: (_: any, r: Payroll) => r.employee?.name || '-' },
    { title: '期间', dataIndex: 'period', key: 'period' },
    { title: '基本工资(元)', dataIndex: 'base_cent', key: 'base', render: (c: number) => yuan(c) },
    { title: '奖金(元)', dataIndex: 'bonus_cent', key: 'bonus', render: (c: number) => yuan(c) },
    { title: '扣款(元)', dataIndex: 'deduction_cent', key: 'deduction', render: (c: number) => yuan(c) },
    { title: '社保(元)', dataIndex: 'social_cent', key: 'social', render: (c: number) => yuan(c) },
    { title: '个税(元)', dataIndex: 'tax_cent', key: 'tax', render: (c: number) => yuan(c) },
    { title: '实发(元)', dataIndex: 'net_cent', key: 'net', render: (c: number) => <b>{yuan(c)}</b> },
    {
      title: '状态', dataIndex: 'status', key: 'status',
      render: (s: string) => <Tag color={PAYROLL_STATUS[s as keyof typeof PAYROLL_STATUS]?.color}>{PAYROLL_STATUS[s as keyof typeof PAYROLL_STATUS]?.label}</Tag>,
    },
    {
      title: '操作', key: 'op', render: (_: any, r: Payroll) => (
        <Space>
          {r.status === 'draft' && (
            <Button size="small" icon={<EditOutlined />} onClick={() => { setTarget(r); setEditOpen(true) }}>编辑</Button>
          )}
          {r.status === 'draft' && (
            <Button size="small" type="link" icon={<CheckOutlined />} onClick={() => calcMut.mutate(r.id)}>核算</Button>
          )}
          {r.status === 'calced' && (
            <Button size="small" type="primary" icon={<CheckOutlined />} onClick={() => payMut.mutate(r.id)}>发放</Button>
          )}
          {r.status !== 'paid' && (
            <Popconfirm title="确认删除？" onConfirm={() => delMut.mutate(r.id)}>
              <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
            </Popconfirm>
          )}
        </Space>
      ),
    },
  ]

  return (
    <div>
      <Space style={{ marginBottom: 12 }} wrap>
        <DatePicker picker="month" value={period} onChange={(d) => d && setPeriod(d)} />
        <Button type="primary" icon={<PlusOutlined />} loading={genMut.isPending}
          onClick={() => genMut.mutate(period.format('YYYY-MM'))}>生成当月薪资</Button>
        <Button icon={<PlusOutlined />} onClick={() => { setTarget(null); setEditOpen(true) }}>新建</Button>
        <Button icon={<DownloadOutlined />} loading={exportMut.isPending}
          onClick={() => exportMut.mutate(period.format('YYYY-MM'))}>导出工资条</Button>
      </Space>
      <Table rowKey="id" loading={isLoading} dataSource={list || []} columns={columns} size="small" />
      <Modal title={target ? '编辑薪资（金额单位：元）' : '新建薪资（金额单位：元）'} open={editOpen} onOk={onSubmit}
        confirmLoading={saveMut.isPending} onCancel={() => { setEditOpen(false); setTarget(null) }} destroyOnClose>
        <Form form={form} layout="vertical">
          <Form.Item label="员工" name="employee_id" rules={[{ required: true, message: '请选择员工' }]}>
            <Select options={empOptions} showSearch optionFilterProp="label" placeholder="选择员工" disabled={!!target} />
          </Form.Item>
          <Form.Item label="期间" name="period" rules={[{ required: true, message: '请选择期间' }]}>
            <DatePicker picker="month" style={{ width: '100%' }} disabled={!!target} />
          </Form.Item>
          <Form.Item label="基本工资(元)" name="base"><InputNumber min={0} step={0.01} style={{ width: '100%' }} /></Form.Item>
          <Form.Item label="奖金(元)" name="bonus"><InputNumber min={0} step={0.01} style={{ width: '100%' }} /></Form.Item>
          <Form.Item label="扣款(元)" name="deduction"><InputNumber min={0} step={0.01} style={{ width: '100%' }} /></Form.Item>
          <Form.Item label="社保(元)" name="social"><InputNumber min={0} step={0.01} style={{ width: '100%' }} /></Form.Item>
          <Form.Item label="个税(元)" name="tax"><InputNumber min={0} step={0.01} style={{ width: '100%' }} /></Form.Item>
          <Form.Item label="备注" name="remark"><Input.TextArea rows={2} placeholder="可选" /></Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
