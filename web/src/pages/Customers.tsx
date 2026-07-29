import { useState } from 'react'
import {
  Table, Tag, Typography, Button, Space, Modal, Form, Input, Select, Popconfirm, App,
} from 'antd'
import { PlusOutlined, SwapOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import dayjs from 'dayjs'
import {
  apiListCustomers, apiCreateCustomer, apiUpdateCustomer, apiDeleteCustomer,
  apiTransferCustomer, apiListDicts, apiListEmployees, apiMe,
  Customer, CustomerInput, CustomerQuery,
} from '../api'

export default function Customers() {
  const { message } = App.useApp()
  const qc = useQueryClient()
  const nav = useNavigate()
  const [query, setQuery] = useState<CustomerQuery>({ page: 1, size: 20 })
  const [editing, setEditing] = useState<Customer | null>(null)
  const [creating, setCreating] = useState(false)
  const [transferTarget, setTransferTarget] = useState<Customer | null>(null)
  const [transferOwner, setTransferOwner] = useState<string>()
  const [form] = Form.useForm()

  const { data: me } = useQuery({ queryKey: ['me'], queryFn: apiMe })
  const canEdit = me?.role !== 'finance' // 财务只读（PRD §6）
  const { data, isLoading } = useQuery({ queryKey: ['customers', query], queryFn: () => apiListCustomers(query) })
  const { data: industries } = useQuery({ queryKey: ['dicts', 'industry'], queryFn: () => apiListDicts('industry') })
  const { data: sources } = useQuery({ queryKey: ['dicts', 'source'], queryFn: () => apiListDicts('source') })
  const { data: levels } = useQuery({ queryKey: ['dicts', 'level'], queryFn: () => apiListDicts('level') })
  const { data: employees } = useQuery({ queryKey: ['employees'], queryFn: () => apiListEmployees() })

  const invalidate = () => qc.invalidateQueries({ queryKey: ['customers'] })

  const saveMut = useMutation({
    mutationFn: (v: CustomerInput): Promise<unknown> =>
      editing ? apiUpdateCustomer(editing.id, v) : apiCreateCustomer(v),
    onSuccess: () => {
      message.success('已保存')
      setEditing(null); setCreating(false); form.resetFields()
      invalidate()
    },
  })
  const deleteMut = useMutation({
    mutationFn: (id: string) => apiDeleteCustomer(id),
    onSuccess: () => { message.success('已删除'); invalidate() },
  })
  const transferMut = useMutation({
    mutationFn: () => apiTransferCustomer(transferTarget!.id, transferOwner!),
    onSuccess: () => {
      message.success('已转移')
      setTransferTarget(null); setTransferOwner(undefined)
      invalidate()
    },
  })

  const openEdit = (c: Customer) => {
    setEditing(c)
    form.setFieldsValue({ name: c.name, industry: c.industry, source: c.source, level: c.level, remark: c.remark })
  }

  const dictOptions = (list?: { value: string }[]) => (list || []).map((d) => ({ value: d.value, label: d.value }))

  return (
    <div>
      <Space style={{ marginBottom: 16, width: '100%', justifyContent: 'space-between' }} wrap>
        <Typography.Title level={4} style={{ margin: 0 }}>客户管理</Typography.Title>
        {canEdit && (
          <Button type="primary" icon={<PlusOutlined />} onClick={() => { setCreating(true); form.resetFields() }}>
            新建客户
          </Button>
        )}
      </Space>
      <Space style={{ marginBottom: 12 }} wrap>
        <Input.Search placeholder="客户名称" allowClear style={{ width: 200 }}
          onSearch={(v) => setQuery({ ...query, keyword: v || undefined, page: 1 })} />
        <Select placeholder="行业" allowClear style={{ width: 130 }} options={dictOptions(industries)}
          onChange={(v) => setQuery({ ...query, industry: v, page: 1 })} />
        <Select placeholder="来源" allowClear style={{ width: 130 }} options={dictOptions(sources)}
          onChange={(v) => setQuery({ ...query, source: v, page: 1 })} />
        <Select placeholder="等级" allowClear style={{ width: 110 }} options={dictOptions(levels)}
          onChange={(v) => setQuery({ ...query, level: v, page: 1 })} />
        <Select placeholder="负责人" allowClear style={{ width: 130 }} showSearch optionFilterProp="label"
          options={(employees?.list || []).map((e) => ({ value: e.id, label: e.name }))}
          onChange={(v) => setQuery({ ...query, owner_id: v, page: 1 })} />
      </Space>

      <Table<Customer>
        rowKey="id"
        loading={isLoading}
        dataSource={data?.list || []}
        pagination={{
          current: query.page, pageSize: query.size, total: data?.total || 0,
          showTotal: (t) => `共 ${t} 条`,
          onChange: (page, size) => setQuery({ ...query, page, size }),
        }}
        columns={[
          { title: '编号', dataIndex: 'code', width: 130 },
          {
            title: '名称', dataIndex: 'name',
            render: (v: string, c) => <a onClick={() => nav(`/customers/${c.id}`)}>{v}</a>,
          },
          { title: '行业', dataIndex: 'industry', render: (v) => v || '-' },
          { title: '来源', dataIndex: 'source', render: (v) => v || '-' },
          { title: '等级', dataIndex: 'level', render: (v) => (v ? <Tag color="gold">{v}</Tag> : '-') },
          { title: '负责人', dataIndex: ['owner', 'name'], render: (_, c) => c.owner?.name || '-' },
          { title: '创建时间', dataIndex: 'created_at', width: 110, render: (v) => dayjs(v).format('YYYY-MM-DD') },
          ...(canEdit
            ? [{
                title: '操作', key: 'op', width: 200,
                render: (_: unknown, c: Customer) => (
                  <Space size="small">
                    <Button size="small" type="link" onClick={() => openEdit(c)}>编辑</Button>
                    <Button size="small" type="link" icon={<SwapOutlined />}
                      onClick={() => { setTransferTarget(c); setTransferOwner(undefined) }}>转移</Button>
                    <Popconfirm title="确认删除？名下有商单/合同的客户不可删" onConfirm={() => deleteMut.mutate(c.id)}>
                      <Button size="small" type="link" danger>删除</Button>
                    </Popconfirm>
                  </Space>
                ),
              }]
            : []),
        ]}
      />

      <Modal
        title={editing ? '编辑客户' : '新建客户'}
        open={creating || !!editing}
        onCancel={() => { setCreating(false); setEditing(null); form.resetFields() }}
        onOk={() => form.validateFields().then((v) => saveMut.mutate(v))}
        confirmLoading={saveMut.isPending}
        okText="保存" cancelText="取消" destroyOnClose
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="客户名称" rules={[{ required: true, message: '请输入客户名称' }]}>
            <Input placeholder="公司主体名称，全系统唯一" />
          </Form.Item>
          <Form.Item name="industry" label="行业">
            <Select allowClear options={dictOptions(industries)} />
          </Form.Item>
          <Form.Item name="source" label="来源">
            <Select allowClear options={dictOptions(sources)} />
          </Form.Item>
          <Form.Item name="level" label="等级">
            <Select allowClear options={dictOptions(levels)} />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={2} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={`转移客户「${transferTarget?.name}」`}
        open={!!transferTarget}
        onCancel={() => setTransferTarget(null)}
        onOk={() => transferOwner && transferMut.mutate()}
        confirmLoading={transferMut.isPending}
        okText="确认转移" cancelText="取消" destroyOnClose
      >
        <Typography.Paragraph type="secondary">转移后该客户归新负责人所有，变更将记录审计日志。</Typography.Paragraph>
        <Select
          placeholder="选择新负责人" style={{ width: '100%' }} showSearch optionFilterProp="label"
          value={transferOwner} onChange={setTransferOwner}
          options={(employees?.list || []).filter((e) => e.status === 'active').map((e) => ({ value: e.id, label: `${e.name}（${e.dept || '未分配部门'}）` }))}
        />
      </Modal>
    </div>
  )
}
