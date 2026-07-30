import { useState } from 'react'
import {
  Table, Tag, Typography, Button, Space, Modal, Form, Input, Select, Popconfirm,
  App, InputNumber, DatePicker, Upload,
} from 'antd'
import {
  PlusOutlined, RiseOutlined, PaperClipOutlined, DownloadOutlined, DeleteOutlined, UploadOutlined,
} from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import dayjs, { Dayjs } from 'dayjs'
import {
  apiListContracts, apiCreateContract, apiUpdateContract, apiChangeContractStatus,
  apiDeleteContract, apiListCustomers, apiListDeals, apiMe,
  apiListContractAttachments, apiUploadContractAttachment, apiDownloadAttachment, apiDeleteAttachment,
  Contract, ContractInput, ContractQuery, CONTRACT_STATUS, CONTRACT_FLOW, Attachment, fenToYuan,
} from '../api'

// 终态（signed 及以后）：金额/客户/关联商单只读
const isLocked = (s?: string) => !!s && ['signed', 'performing', 'completed', 'terminated', 'expired'].includes(s)
// 可删除状态：仅签约前的草稿/待签/已取消
const isDeletable = (s?: string) => !!s && ['draft', 'pending', 'cancelled'].includes(s)

export default function Contracts() {
  const { message } = App.useApp()
  const qc = useQueryClient()
  const [query, setQuery] = useState<ContractQuery>({ page: 1, size: 20 })
  const [editing, setEditing] = useState<Contract | null>(null)
  const [creating, setCreating] = useState(false)
  const [transiting, setTransiting] = useState<Contract | null>(null)
  const [transitTo, setTransitTo] = useState<string>()
  const [terminateReason, setTerminateReason] = useState<string>()
  const [attachId, setAttachId] = useState<string | null>(null)
  const [form] = Form.useForm()

  const { data: me } = useQuery({ queryKey: ['me'], queryFn: apiMe })
  const canEdit = me?.role !== 'finance'
  const { data, isLoading } = useQuery({ queryKey: ['contracts', query], queryFn: () => apiListContracts(query) })
  const { data: customers } = useQuery({ queryKey: ['customers', 'all'], queryFn: () => apiListCustomers({ page: 1, size: 100 }) })
  const { data: wonDeals } = useQuery({ queryKey: ['deals', 'won'], queryFn: () => apiListDeals({ status: 'won', page: 1, size: 100 }) })

  const invalidate = () => {
    qc.invalidateQueries({ queryKey: ['contracts'] })
  }
  const canTransit = (c: Contract) => canEdit && (CONTRACT_FLOW[c.status] || []).length > 0

  const saveMut = useMutation({
    mutationFn: (v: ContractInput): Promise<unknown> =>
      editing ? apiUpdateContract(editing.id, v) : apiCreateContract(v),
    onSuccess: () => {
      message.success('已保存')
      setEditing(null); setCreating(false); form.resetFields()
      invalidate()
    },
  })
  const deleteMut = useMutation({
    mutationFn: (id: string) => apiDeleteContract(id),
    onSuccess: () => { message.success('已删除'); invalidate() },
  })
  const transitMut = useMutation({
    mutationFn: () => apiChangeContractStatus(transiting!.id, transitTo!, transitTo === 'terminated' ? terminateReason : undefined),
    onSuccess: () => {
      message.success('状态已更新')
      setTransiting(null); setTransitTo(undefined); setTerminateReason(undefined)
      invalidate()
    },
  })

  const openEdit = (c: Contract) => {
    setEditing(c)
    form.setFieldsValue({
      customer_id: c.customer_id,
      title: c.title,
      amount: c.amount_cent / 100,
      sign_date: c.sign_date ? dayjs(c.sign_date) : null,
      start_date: c.start_date ? dayjs(c.start_date) : null,
      expire_date: c.expire_date ? dayjs(c.expire_date) : null,
      remark: c.remark,
      deal_ids: (c.deals || []).map((d) => d.id),
    })
  }

  const submitForm = () => {
    form.validateFields().then((v) => {
      const payload: ContractInput = {
        customer_id: v.customer_id,
        title: v.title,
        amount_cent: Math.round((v.amount || 0) * 100),
        sign_date: v.sign_date ? (v.sign_date as Dayjs).format('YYYY-MM-DD') : '',
        start_date: v.start_date ? (v.start_date as Dayjs).format('YYYY-MM-DD') : '',
        expire_date: v.expire_date ? (v.expire_date as Dayjs).format('YYYY-MM-DD') : '',
        remark: v.remark || '',
        deal_ids: (v.deal_ids || []).map((id: string) => parseInt(id, 10)),
      }
      saveMut.mutate(payload)
    })
  }

  return (
    <div>
      <Space style={{ marginBottom: 16, width: '100%', justifyContent: 'space-between' }} wrap>
        <Typography.Title level={4} style={{ margin: 0 }}>合同管理</Typography.Title>
        {canEdit && (
          <Button type="primary" icon={<PlusOutlined />} onClick={() => { setCreating(true); form.resetFields() }}>
            新建合同
          </Button>
        )}
      </Space>

      <Space style={{ marginBottom: 12 }} wrap>
        <Input.Search placeholder="合同标题" allowClear style={{ width: 200 }}
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
          { title: '单号', dataIndex: 'code', width: 130 },
          { title: '标题', dataIndex: 'title' },
          { title: '客户', dataIndex: ['customer', 'name'], render: (_, c) => c.customer?.name || '-' },
          { title: '金额(元)', dataIndex: 'amount_cent', align: 'right' as const, render: (v) => fenToYuan(v) },
          {
            title: '状态', dataIndex: 'status', width: 100,
            render: (v: string) => <Tag color={CONTRACT_STATUS[v]?.color}>{CONTRACT_STATUS[v]?.label || v}</Tag>,
          },
          { title: '负责人', dataIndex: ['owner', 'name'], width: 90, render: (_, c) => c.owner?.name || '-' },
          ...(canEdit
            ? [{
                title: '操作', key: 'op', width: 260,
                render: (_: unknown, c: Contract) => (
                  <Space size="small">
                    <Button size="small" type="link" onClick={() => openEdit(c)}>编辑</Button>
                    {canTransit(c) && (
                      <Button size="small" type="link" icon={<RiseOutlined />}
                        onClick={() => { setTransiting(c); setTransitTo(undefined); setTerminateReason(undefined) }}>推进</Button>
                    )}
                    {isDeletable(c.status) && (
                      <Popconfirm title="确认删除该合同？" onConfirm={() => deleteMut.mutate(c.id)}>
                        <Button size="small" type="link" danger>删除</Button>
                      </Popconfirm>
                    )}
                    <Button size="small" type="link" icon={<PaperClipOutlined />} onClick={() => setAttachId(c.id)}>附件</Button>
                  </Space>
                ),
              }]
            : [{
                title: '操作', key: 'op', width: 80,
                render: (_: unknown, c: Contract) => (
                  <Button size="small" type="link" icon={<PaperClipOutlined />} onClick={() => setAttachId(c.id)}>附件</Button>
                ),
              }]),
        ]}
      />

      {/* 新建/编辑 */}
      <Modal
        title={editing ? (isLocked(editing.status) ? '编辑合同（已签约，金额/客户/关联商单只读）' : '编辑合同') : '新建合同'}
        open={creating || !!editing}
        onCancel={() => { setCreating(false); setEditing(null); form.resetFields() }}
        onOk={submitForm}
        confirmLoading={saveMut.isPending}
        okText="保存" cancelText="取消"
      >
        <Form form={form} layout="vertical">
          <Form.Item name="customer_id" label="关联客户" rules={[{ required: true, message: '请选择客户' }]}>
            <Select showSearch optionFilterProp="label" disabled={!!editing}
              options={(customers?.list || []).map((c) => ({ value: c.id, label: c.name }))} />
          </Form.Item>
          <Form.Item name="title" label="合同标题" rules={[{ required: true, message: '请输入标题' }]}>
            <Input disabled={!!editing && isLocked(editing.status)} />
          </Form.Item>
          <Form.Item name="amount" label="合同金额（元）">
            <InputNumber min={0} precision={2} style={{ width: '100%' }} disabled={!!editing && isLocked(editing.status)} />
          </Form.Item>
          <Form.Item name="sign_date" label="签约日期"><DatePicker style={{ width: '100%' }} /></Form.Item>
          <Form.Item name="start_date" label="开始日期"><DatePicker style={{ width: '100%' }} /></Form.Item>
          <Form.Item name="expire_date" label="到期日期"><DatePicker style={{ width: '100%' }} /></Form.Item>
          <Form.Item name="deal_ids" label="关联赢单商单" tooltip="仅可关联已赢单(won)且与合同同客户的商单">
            <Select mode="multiple" allowClear showSearch optionFilterProp="label"
              disabled={!!editing && isLocked(editing.status)}
              options={(wonDeals?.list || []).map((d) => ({ value: d.id, label: `${d.code} ${d.title}` }))} />
          </Form.Item>
          <Form.Item name="remark" label="备注"><Input.TextArea rows={2} /></Form.Item>
        </Form>
      </Modal>

      {/* 状态流转 */}
      <Modal
        title={`合同状态流转「${transiting?.title}」`}
        open={!!transiting}
        onCancel={() => { setTransiting(null); setTransitTo(undefined); setTerminateReason(undefined) }}
        onOk={() => transitTo && transitMut.mutate()}
        okButtonProps={{ disabled: !transitTo || (transitTo === 'terminated' && !terminateReason) }}
        confirmLoading={transitMut.isPending}
        okText="确认流转" cancelText="取消"
      >
        <Typography.Paragraph>
          当前状态：<Tag color={CONTRACT_STATUS[transiting?.status || '']?.color}>
            {CONTRACT_STATUS[transiting?.status || '']?.label}
          </Tag>
        </Typography.Paragraph>
        <Select
          placeholder="选择目标状态" style={{ width: '100%' }} value={transitTo} onChange={setTransitTo}
          options={(CONTRACT_FLOW[transiting?.status || ''] || []).map((s) => ({ value: s, label: CONTRACT_STATUS[s].label }))}
        />
        {transitTo === 'terminated' && (
          <Form.Item label="终止原因" required style={{ marginTop: 16, marginBottom: 0 }}>
            <Input.TextArea value={terminateReason} onChange={(e) => setTerminateReason(e.target.value)} rows={2}
              placeholder="终止合同必须填写原因" />
          </Form.Item>
        )}
      </Modal>

      {/* 附件 */}
      <Modal title="合同附件" open={!!attachId} onCancel={() => setAttachId(null)} footer={null} width={660}>
        {attachId && <AttachmentPanel contractId={attachId} canEdit={canEdit} />}
      </Modal>
    </div>
  )
}

// 附件面板：列表 + 上传（白名单/20MB 由后端校验）+ 下载（鉴权）+ 删除
function AttachmentPanel({ contractId, canEdit }: { contractId: string; canEdit: boolean }) {
  const { message } = App.useApp()
  const qc = useQueryClient()
  const { data: list, isLoading } = useQuery({
    queryKey: ['attachments', contractId],
    queryFn: () => apiListContractAttachments(contractId),
  })
  const uploadMut = useMutation({
    mutationFn: (file: File) => apiUploadContractAttachment(contractId, file),
    onSuccess: () => { message.success('上传成功'); qc.invalidateQueries({ queryKey: ['attachments', contractId] }) },
  })
  const delMut = useMutation({
    mutationFn: (id: string) => apiDeleteAttachment(id),
    onSuccess: () => { message.success('已删除'); qc.invalidateQueries({ queryKey: ['attachments', contractId] }) },
  })
  const doDownload = async (att: Attachment) => {
    try {
      const blob = await apiDownloadAttachment(att.id)
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = att.file_name
      a.click()
      URL.revokeObjectURL(url)
    } catch {
      // 拦截器已提示
    }
  }
  return (
    <div>
      <Space style={{ marginBottom: 12 }} wrap>
        <Upload beforeUpload={(file) => { uploadMut.mutate(file); return false }} showUploadList={false} disabled={!canEdit}>
          <Button type="primary" icon={<UploadOutlined />} disabled={!canEdit}>上传附件</Button>
        </Upload>
        <Typography.Text type="secondary">支持 pdf/doc/docx/xls/xlsx/ppt/pptx/图片/zip/rar/txt/csv，单个不超过 20MB</Typography.Text>
      </Space>
      <Table<Attachment>
        rowKey="id" loading={isLoading} size="small"
        dataSource={list || []}
        pagination={false}
        columns={[
          { title: '文件名', dataIndex: 'file_name' },
          { title: '大小', dataIndex: 'file_size', width: 110, render: (v: number) => `${(v / 1024).toFixed(1)} KB` },
          { title: '上传时间', dataIndex: 'created_at', width: 170 },
          {
            title: '操作', key: 'op', width: 140,
            render: (_, att) => (
              <Space size="small">
                <Button size="small" type="link" icon={<DownloadOutlined />} onClick={() => doDownload(att)}>下载</Button>
                {canEdit && (
                  <Button size="small" type="link" danger icon={<DeleteOutlined />} onClick={() => delMut.mutate(att.id)}>删除</Button>
                )}
              </Space>
            ),
          },
        ]}
      />
    </div>
  )
}
