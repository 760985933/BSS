import { useState } from 'react'
import {
  Table, Tag, Typography, Button, Space, Modal, Form, Input, Select, Popconfirm,
  App, Progress, InputNumber, DatePicker, Statistic, Card, Row, Col,
} from 'antd'
import { PlusOutlined, RiseOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import dayjs, { Dayjs } from 'dayjs'
import {
  apiListDeals, apiCreateDeal, apiUpdateDeal, apiChangeDealStatus, apiDeleteDeal,
  apiDealForecast, apiListCustomers, apiMe,
  Deal, DealInput, DealQuery, DEAL_STAGES, DEAL_FLOW, LOST_REASONS, fenToYuan,
} from '../api'

interface WarningBody { warning?: boolean; message?: string }

// 阶段顺序（判定回退：目标阶段序号 < 当前阶段序号）
const STAGE_ORDER = ['prospecting', 'qualifying', 'proposal', 'negotiating']
const isRollback = (from: string, to: string) => {
  const fi = STAGE_ORDER.indexOf(from)
  const ti = STAGE_ORDER.indexOf(to)
  return fi >= 0 && ti >= 0 && ti < fi
}

export default function Deals() {
  const { message, modal } = App.useApp()
  const qc = useQueryClient()
  const [query, setQuery] = useState<DealQuery>({ page: 1, size: 20 })
  const [editing, setEditing] = useState<Deal | null>(null)
  const [creating, setCreating] = useState(false)
  const [transiting, setTransiting] = useState<Deal | null>(null)
  const [transitTo, setTransitTo] = useState<string>()
  const [lostDeal, setLostDeal] = useState<Deal | null>(null)
  const [lostReason, setLostReason] = useState<string>()
  const [form] = Form.useForm()

  const { data: me } = useQuery({ queryKey: ['me'], queryFn: apiMe })
  const canEdit = me?.role !== 'finance'
  const { data, isLoading } = useQuery({ queryKey: ['deals', query], queryFn: () => apiListDeals(query) })
  const { data: forecast } = useQuery({ queryKey: ['dealForecast'], queryFn: apiDealForecast })
  const { data: customers } = useQuery({
    queryKey: ['customers', 'all'],
    queryFn: () => apiListCustomers({ page: 1, size: 100 }),
  })

  const invalidate = () => {
    qc.invalidateQueries({ queryKey: ['deals'] })
    qc.invalidateQueries({ queryKey: ['dealForecast'] })
  }

  // 状态流转（含软校验 force 确认）
  const doTransit = async (deal: Deal, to: string, reason?: string, force = false) => {
    try {
      await apiChangeDealStatus(deal.id, to, reason, force)
      message.success(`已流转到「${DEAL_STAGES[to]?.label}」`)
      setTransiting(null); setTransitTo(undefined)
      setLostDeal(null); setLostReason(undefined)
      invalidate()
    } catch (e) {
      const body = e as WarningBody
      if (body?.warning) {
        modal.confirm({
          title: '退出标准提示',
          content: body.message,
          okText: '确认推进',
          cancelText: '取消',
          onOk: () => doTransit(deal, to, reason, true),
        })
      }
    }
  }

  const saveMut = useMutation({
    mutationFn: (v: DealInput): Promise<unknown> =>
      editing ? apiUpdateDeal(editing.id, v) : apiCreateDeal(v),
    onSuccess: () => {
      message.success('已保存')
      setEditing(null); setCreating(false); form.resetFields()
      invalidate()
    },
  })
  const deleteMut = useMutation({
    mutationFn: (id: string) => apiDeleteDeal(id),
    onSuccess: () => { message.success('已删除'); invalidate() },
  })

  const openEdit = (d: Deal) => {
    setEditing(d)
    form.setFieldsValue({
      customer_id: d.customer_id, title: d.title, amount: d.amount_cent / 100,
      probability: d.probability,
      expected_sign_date: d.expected_sign_date ? dayjs(d.expected_sign_date) : null,
      remark: d.remark,
    })
  }

  // 防御式：入参允许 null（新建时 editing/transiting 均为 null）
  const closed = (d?: Deal | null) => !!d && (d.status === 'won' || d.status === 'lost')

  const submitForm = () => {
    form.validateFields().then((v) => {
      const payload: DealInput = {
        customer_id: v.customer_id, title: v.title,
        amount_cent: Math.round((v.amount || 0) * 100),
        probability: v.probability,
        expected_sign_date: v.expected_sign_date ? (v.expected_sign_date as Dayjs).format('YYYY-MM-DD') : '',
        remark: v.remark || '',
      }
      saveMut.mutate(payload)
    })
  }

  return (
    <div>
      <Space style={{ marginBottom: 16, width: '100%', justifyContent: 'space-between' }} wrap>
        <Typography.Title level={4} style={{ margin: 0 }}>商单管理</Typography.Title>
        {canEdit && (
          <Button type="primary" icon={<PlusOutlined />} onClick={() => { setCreating(true); form.resetFields() }}>
            新建商单
          </Button>
        )}
      </Space>

      {forecast && (
        <Row gutter={16} style={{ marginBottom: 16 }}>
          <Col span={8}><Card size="small"><Statistic title="进行中商单" value={forecast.open_count} suffix="单" /></Card></Col>
          <Col span={8}><Card size="small"><Statistic title="进行中金额合计" value={fenToYuan(forecast.total_cent)} prefix="¥" /></Card></Col>
          <Col span={8}>
            <Card size="small">
              <Statistic title="加权预测金额（金额×概率）" value={fenToYuan(forecast.weighted_cent)} prefix="¥"
                valueStyle={{ color: '#1677ff' }} />
            </Card>
          </Col>
        </Row>
      )}

      <Space style={{ marginBottom: 12 }} wrap>
        <Input.Search placeholder="商单标题" allowClear style={{ width: 200 }}
          onSearch={(v) => setQuery({ ...query, keyword: v || undefined, page: 1 })} />
        <Select placeholder="阶段" allowClear style={{ width: 130 }}
          options={Object.entries(DEAL_STAGES).map(([value, v]) => ({ value, label: v.label }))}
          onChange={(v) => setQuery({ ...query, status: v, page: 1 })} />
        <Select placeholder="客户" allowClear style={{ width: 200 }} showSearch optionFilterProp="label"
          options={(customers?.list || []).map((c) => ({ value: c.id, label: c.name }))}
          onChange={(v) => setQuery({ ...query, customer_id: v, page: 1 })} />
      </Space>

      <Table<Deal>
        rowKey="id"
        loading={isLoading}
        dataSource={data?.list || []}
        pagination={{
          current: query.page, pageSize: query.size, total: data?.total || 0,
          showTotal: (t) => `共 ${t} 条`,
          onChange: (page, size) => setQuery({ ...query, page, size }),
        }}
        columns={[
          { title: '单号', dataIndex: 'code', width: 130 },
          { title: '标题', dataIndex: 'title' },
          { title: '客户', dataIndex: ['customer', 'name'], render: (_, d) => d.customer?.name || '-' },
          { title: '金额(元)', dataIndex: 'amount_cent', align: 'right' as const, render: (v) => fenToYuan(v) },
          {
            title: '阶段', dataIndex: 'status', width: 110,
            render: (v: string) => <Tag color={DEAL_STAGES[v]?.color}>{DEAL_STAGES[v]?.label || v}</Tag>,
          },
          {
            title: '赢率', dataIndex: 'probability', width: 100,
            render: (v: number, d) => (
              <Progress percent={v} size="small" status={d.status === 'lost' ? 'exception' : undefined} />
            ),
          },
          { title: '预计签约', dataIndex: 'expected_sign_date', width: 110, render: (v) => v || '-' },
          { title: '负责人', dataIndex: ['owner', 'name'], width: 90, render: (_, d) => d.owner?.name || '-' },
          ...(canEdit
            ? [{
                title: '操作', key: 'op', width: 210,
                render: (_: unknown, d: Deal) => (
                  <Space size="small">
                    {!closed(d) && (
                      <>
                        <Button size="small" type="link" onClick={() => openEdit(d)}>编辑</Button>
                        <Button size="small" type="link" icon={<RiseOutlined />}
                          onClick={() => { setTransiting(d); setTransitTo(undefined) }}>推进</Button>
                        <Popconfirm title="确认删除该商单？" onConfirm={() => deleteMut.mutate(d.id)}>
                          <Button size="small" type="link" danger>删除</Button>
                        </Popconfirm>
                      </>
                    )}
                    {closed(d) && (
                      <Button size="small" type="link" onClick={() => openEdit(d)}>备注</Button>
                    )}
                  </Space>
                ),
              }]
            : []),
        ]}
      />

      {/* 新建/编辑 */}
      <Modal
        title={editing ? (closed(editing) ? '编辑备注（商单已关闭）' : '编辑商单') : '新建商单'}
        open={creating || !!editing}
        onCancel={() => { setCreating(false); setEditing(null); form.resetFields() }}
        onOk={submitForm}
        confirmLoading={saveMut.isPending}
        okText="保存" cancelText="取消" destroyOnClose
      >
        <Form form={form} layout="vertical">
          <Form.Item name="customer_id" label="关联客户" rules={[{ required: true, message: '请选择客户' }]}>
            <Select showSearch optionFilterProp="label" disabled={!!editing}
              options={(customers?.list || []).map((c) => ({ value: c.id, label: c.name }))} />
          </Form.Item>
          <Form.Item name="title" label="商单标题" rules={[{ required: !closed(editing), message: '请输入标题' }]}>
            <Input disabled={!!editing && closed(editing)} placeholder="例：云杉科技 CRM 实施项目" />
          </Form.Item>
          <Form.Item name="amount" label="金额（元）">
            <InputNumber min={0} precision={2} style={{ width: '100%' }} disabled={!!editing && closed(editing)} />
          </Form.Item>
          <Form.Item name="probability" label="赢单概率（%）" tooltip="留空按阶段默认带出">
            <InputNumber min={0} max={100} style={{ width: '100%' }} disabled={!!editing && closed(editing)} />
          </Form.Item>
          <Form.Item name="expected_sign_date" label="预计签约日期">
            <DatePicker style={{ width: '100%' }} disabled={!!editing && closed(editing)} />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={2} />
          </Form.Item>
        </Form>
      </Modal>

      {/* 阶段推进 */}
      <Modal
        title={`推进商单「${transiting?.title}」`}
        open={!!transiting}
        onCancel={() => setTransiting(null)}
        onOk={() => {
          if (!transitTo) return
          if (transitTo === 'lost') {
            setLostDeal(transiting); setTransiting(null)
          } else {
            doTransit(transiting!, transitTo)
          }
        }}
        okButtonProps={{ disabled: !transitTo }}
        okText="确认流转" cancelText="取消" destroyOnClose
      >
        <Typography.Paragraph>
          当前阶段：<Tag color={DEAL_STAGES[transiting?.status || '']?.color}>
            {DEAL_STAGES[transiting?.status || '']?.label}
          </Tag>
        </Typography.Paragraph>
        <Select
          placeholder="选择目标阶段" style={{ width: '100%' }} value={transitTo} onChange={setTransitTo}
          options={(DEAL_FLOW[transiting?.status || ''] || []).map((s) => ({
            value: s,
            label: `${DEAL_STAGES[s].label}${isRollback(transiting?.status || '', s) ? '（回退）' : ''}`,
          }))}
        />
      </Modal>

      {/* 输单原因 */}
      <Modal
        title={`输单登记「${lostDeal?.title}」`}
        open={!!lostDeal}
        onCancel={() => { setLostDeal(null); setLostReason(undefined) }}
        onOk={() => lostReason && doTransit(lostDeal!, 'lost', lostReason)}
        okButtonProps={{ disabled: !lostReason, danger: true }}
        okText="确认输单" cancelText="取消" destroyOnClose
      >
        <Typography.Paragraph type="secondary">输单为终态，不可恢复；输单原因将用于二期输单分析。</Typography.Paragraph>
        <Select placeholder="选择输单原因（必填）" style={{ width: '100%' }}
          value={lostReason} onChange={setLostReason} options={LOST_REASONS} />
      </Modal>
    </div>
  )
}
