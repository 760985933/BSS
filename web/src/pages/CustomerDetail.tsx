import { useState } from 'react'
import {
  Card, Descriptions, Tag, Typography, Button, Table, Space, Modal, Form,
  Input, Switch, Popconfirm, Tabs, App, Empty,
} from 'antd'
import { ArrowLeftOutlined, PlusOutlined, StarFilled } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate, useParams } from 'react-router-dom'
import dayjs from 'dayjs'
import {
  apiGetCustomer, apiListContacts, apiCreateContact, apiUpdateContact,
  apiDeleteContact, apiMe, Contact, ContactInput,
} from '../api'

const placeholder = (label: string) => (
  <Empty description={`${label}将在后续 Sprint 上线`} image={Empty.PRESENTED_IMAGE_SIMPLE} />
)

export default function CustomerDetail() {
  const { id = '' } = useParams()
  const nav = useNavigate()
  const { message } = App.useApp()
  const qc = useQueryClient()
  const [editing, setEditing] = useState<Contact | null>(null)
  const [creating, setCreating] = useState(false)
  const [form] = Form.useForm()

  const { data: me } = useQuery({ queryKey: ['me'], queryFn: apiMe })
  const canEdit = me?.role !== 'finance'
  const { data: customer } = useQuery({ queryKey: ['customer', id], queryFn: () => apiGetCustomer(id) })
  const { data: contacts, isLoading } = useQuery({ queryKey: ['contacts', id], queryFn: () => apiListContacts(id) })

  const invalidate = () => qc.invalidateQueries({ queryKey: ['contacts', id] })

  const saveMut = useMutation({
    mutationFn: (v: ContactInput): Promise<unknown> =>
      editing ? apiUpdateContact(id, editing.id, v) : apiCreateContact(id, v),
    onSuccess: () => {
      message.success('已保存')
      setEditing(null); setCreating(false); form.resetFields()
      invalidate()
    },
  })
  const deleteMut = useMutation({
    mutationFn: (cid: string) => apiDeleteContact(id, cid),
    onSuccess: () => { message.success('已删除'); invalidate() },
  })

  const openEdit = (c: Contact) => {
    setEditing(c)
    form.setFieldsValue({ name: c.name, phone: c.phone, email: c.email, position: c.position, is_primary: c.is_primary, remark: c.remark })
  }

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => nav('/customers')}>返回</Button>
        <Typography.Title level={4} style={{ margin: 0 }}>
          {customer?.name} <Tag color="blue">{customer?.code}</Tag>
        </Typography.Title>
      </Space>

      <Card title="基本信息" style={{ marginBottom: 16 }}>
        <Descriptions column={{ xs: 1, sm: 2, md: 3 }} size="small">
          <Descriptions.Item label="行业">{customer?.industry || '-'}</Descriptions.Item>
          <Descriptions.Item label="来源">{customer?.source || '-'}</Descriptions.Item>
          <Descriptions.Item label="等级">{customer?.level || '-'}</Descriptions.Item>
          <Descriptions.Item label="负责人">{customer?.owner?.name || '-'}</Descriptions.Item>
          <Descriptions.Item label="创建时间">{customer ? dayjs(customer.created_at).format('YYYY-MM-DD HH:mm') : '-'}</Descriptions.Item>
          <Descriptions.Item label="备注">{customer?.remark || '-'}</Descriptions.Item>
        </Descriptions>
      </Card>

      <Tabs
        items={[
          {
            key: 'contacts',
            label: `联系人 (${contacts?.length || 0})`,
            children: (
              <Card
                title="联系人"
                extra={canEdit && (
                  <Button size="small" type="primary" icon={<PlusOutlined />}
                    onClick={() => { setCreating(true); form.resetFields() }}>添加</Button>
                )}
              >
                <Table<Contact>
                  rowKey="id"
                  size="small"
                  loading={isLoading}
                  dataSource={contacts || []}
                  pagination={false}
                  columns={[
                    {
                      title: '姓名', dataIndex: 'name',
                      render: (v: string, c) => (
                        <Space>{v}{c.is_primary && <StarFilled style={{ color: '#faad14' }} title="首要联系人" />}</Space>
                      ),
                    },
                    { title: '职位', dataIndex: 'position', render: (v) => v || '-' },
                    { title: '手机', dataIndex: 'phone', render: (v) => v || '-' },
                    { title: '邮箱', dataIndex: 'email', render: (v) => v || '-' },
                    { title: '备注', dataIndex: 'remark', render: (v) => v || '-' },
                    ...(canEdit
                      ? [{
                          title: '操作', key: 'op', width: 140,
                          render: (_: unknown, c: Contact) => (
                            <Space size="small">
                              <Button size="small" type="link" onClick={() => openEdit(c)}>编辑</Button>
                              <Popconfirm title="删除该联系人？" onConfirm={() => deleteMut.mutate(c.id)}>
                                <Button size="small" type="link" danger>删除</Button>
                              </Popconfirm>
                            </Space>
                          ),
                        }]
                      : []),
                  ]}
                />
              </Card>
            ),
          },
          { key: 'deals', label: '商单', children: placeholder('商单（Sprint 3）') },
          { key: 'contracts', label: '合同', children: placeholder('合同（Sprint 4）') },
          { key: 'payments', label: '回款', children: placeholder('回款（Sprint 5）') },
        ]}
      />

      <Modal
        title={editing ? '编辑联系人' : '添加联系人'}
        open={creating || !!editing}
        onCancel={() => { setCreating(false); setEditing(null); form.resetFields() }}
        onOk={() => form.validateFields().then((v) => saveMut.mutate(v))}
        confirmLoading={saveMut.isPending}
        okText="保存" cancelText="取消" destroyOnClose
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="姓名" rules={[{ required: true, message: '请输入姓名' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="position" label="职位"><Input /></Form.Item>
          <Form.Item name="phone" label="手机"><Input /></Form.Item>
          <Form.Item name="email" label="邮箱" rules={[{ type: 'email', message: '邮箱格式不正确' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="is_primary" label="首要联系人" valuePropName="checked"
            tooltip="每个客户至多一个首要联系人，设置后自动替换原首要联系人">
            <Switch />
          </Form.Item>
          <Form.Item name="remark" label="备注"><Input.TextArea rows={2} /></Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
