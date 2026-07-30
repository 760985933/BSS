import { useState } from 'react'
import {
  Table, Tag, Typography, Button, Space, Modal, Form, Input, Select, Popconfirm, App,
} from 'antd'
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  apiListEmployees, apiCreateEmployee, apiUpdateEmployee, apiSetEmployeeStatus,
  apiResetEmployeePassword, apiListDicts, apiMe,
  apiOffboardPreview, apiOffboard, OffboardPreview,
  Employee, EmployeeInput,
} from '../api'

const roleMap: Record<string, { label: string; color: string }> = {
  admin: { label: '管理员', color: 'red' },
  sales: { label: '销售', color: 'blue' },
  sales_lead: { label: '销售主管', color: 'geekblue' },
  finance: { label: '财务', color: 'green' },
  hr: { label: 'HR', color: 'purple' },
}
const roleOptions = Object.entries(roleMap).map(([value, v]) => ({ value, label: v.label }))

export default function Employees() {
  const { message, modal } = App.useApp()
  const qc = useQueryClient()
  const [keyword, setKeyword] = useState('')
  const [editing, setEditing] = useState<Employee | null>(null)
  const [creating, setCreating] = useState(false)
  const [obTarget, setObTarget] = useState<Employee | null>(null)
  const [obPreview, setObPreview] = useState<OffboardPreview | null>(null)
  const [obSuccessor, setObSuccessor] = useState<string>('')
  const [form] = Form.useForm()

  const { data: me } = useQuery({ queryKey: ['me'], queryFn: apiMe })
  const canManage = me?.role === 'admin' || me?.role === 'hr'
  const isAdmin = me?.role === 'admin'

  const { data, isLoading } = useQuery({
    queryKey: ['employees', keyword],
    queryFn: () => apiListEmployees(keyword || undefined),
  })
  const { data: depts } = useQuery({ queryKey: ['dicts', 'dept'], queryFn: () => apiListDicts('dept') })

  const invalidate = () => qc.invalidateQueries({ queryKey: ['employees'] })

  const saveMut = useMutation({
    mutationFn: async (v: EmployeeInput & { email?: string }) => {
      if (editing) return apiUpdateEmployee(editing.id, v)
      return apiCreateEmployee(v as EmployeeInput & { email: string })
    },
    onSuccess: (res) => {
      if (!editing && 'initial_password' in res) {
        modal.success({
          title: '员工已创建',
          content: `初始密码：${res.initial_password}（首次登录强制改密），请线下告知本人。`,
        })
      } else {
        message.success('已保存')
      }
      setEditing(null)
      setCreating(false)
      form.resetFields()
      invalidate()
    },
  })

  const statusMut = useMutation({
    mutationFn: ({ id, active }: { id: string; active: boolean }) => apiSetEmployeeStatus(id, active),
    onSuccess: () => { message.success('状态已更新'); invalidate() },
  })

  const resetMut = useMutation({
    mutationFn: (id: string) => apiResetEmployeePassword(id),
    onSuccess: (res) => {
      modal.success({
        title: '密码已重置',
        content: `新初始密码：${res.initial_password}（首次登录强制改密），请线下告知本人。`,
      })
    },
  })

  // 离职交接：停用前先预览名下数据，有数据则弹窗选交接人
  const previewMut = useMutation({
    mutationFn: (e: Employee) => apiOffboardPreview(e.id),
    onSuccess: (prev, e) => {
      if (!prev.has_data) {
        statusMut.mutate({ id: e.id, active: false })
      } else {
        setObTarget(e)
        setObPreview(prev)
        setObSuccessor('')
      }
    },
    onError: (err: any) => message.error(err?.response?.data?.message || '查询交接信息失败'),
  })

  const offboardMut = useMutation({
    mutationFn: ({ id, successor }: { id: string; successor: string }) => apiOffboard(id, successor),
    onSuccess: () => {
      message.success('已转移名下数据并停用账号')
      setObTarget(null)
      setObPreview(null)
      invalidate()
    },
    onError: (err: any) => message.error(err?.response?.data?.message || '交接失败'),
  })

  const successorOptions = (data?.list || [])
    .filter((e) => obTarget && e.id !== obTarget.id && e.status === 'active')
    .map((e) => ({ value: e.id, label: `${e.name}（${e.email}）` }))

  const openEdit = (e: Employee) => {
    setEditing(e)
    form.setFieldsValue({ name: e.name, phone: e.phone, dept: e.dept, position: e.position, role: e.role, email: e.email })
  }

  return (
    <div>
      <Space style={{ marginBottom: 16, width: '100%', justifyContent: 'space-between' }}>
        <Typography.Title level={4} style={{ margin: 0 }}>员工管理</Typography.Title>
        <Space>
          <Input.Search
            placeholder="姓名 / 邮箱"
            allowClear
            style={{ width: 220 }}
            onSearch={setKeyword}
          />
          {canManage && (
            <Button type="primary" icon={<PlusOutlined />} onClick={() => { setCreating(true); form.resetFields() }}>
              新建员工
            </Button>
          )}
        </Space>
      </Space>

      <Table<Employee>
        rowKey="id"
        loading={isLoading}
        dataSource={data?.list || []}
        pagination={false}
        columns={[
          { title: '姓名', dataIndex: 'name' },
          { title: '邮箱', dataIndex: 'email' },
          { title: '手机', dataIndex: 'phone', render: (v) => v || '-' },
          { title: '部门', dataIndex: 'dept', render: (v) => v || '-' },
          { title: '职位', dataIndex: 'position', render: (v) => v || '-' },
          {
            title: '角色', dataIndex: 'role',
            render: (v: string) => <Tag color={roleMap[v]?.color}>{roleMap[v]?.label || v}</Tag>,
          },
          {
            title: '状态', dataIndex: 'status',
            render: (v: string) => (v === 'active' ? <Tag color="success">在职</Tag> : <Tag>停用</Tag>),
          },
          ...(canManage
            ? [{
                title: '操作', key: 'op', width: 220,
                render: (_: unknown, e: Employee) => (
                  <Space size="small">
                    <Button size="small" type="link" onClick={() => openEdit(e)}>编辑</Button>
                    {e.status === 'active' && e.id !== me?.id ? (
                      <Popconfirm title="停用前将检查名下数据；若有数据需先交接" onConfirm={() => previewMut.mutate(e)}>
                        <Button size="small" type="link" danger>停用</Button>
                      </Popconfirm>
                    ) : e.status !== 'active' ? (
                      <Button size="small" type="link" onClick={() => statusMut.mutate({ id: e.id, active: true })}>
                        <ReloadOutlined /> 启用
                      </Button>
                    ) : null}
                    {isAdmin && (
                      <Popconfirm title="重置为初始密码？" onConfirm={() => resetMut.mutate(e.id)}>
                        <Button size="small" type="link">重置密码</Button>
                      </Popconfirm>
                    )}
                  </Space>
                ),
              }]
            : []),
        ]}
      />

      <Modal
        title={editing ? '编辑员工' : '新建员工'}
        open={creating || !!editing}
        onCancel={() => { setCreating(false); setEditing(null); form.resetFields() }}
        onOk={() => form.validateFields().then((v) => saveMut.mutate(v))}
        confirmLoading={saveMut.isPending}
        okText="保存"
        cancelText="取消"
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="姓名" rules={[{ required: true, message: '请输入姓名' }]}>
            <Input />
          </Form.Item>
          <Form.Item
            name="email"
            label="邮箱（登录账号）"
            rules={[{ required: true, message: '请输入邮箱' }, { type: 'email', message: '邮箱格式不正确' }]}
          >
            <Input disabled={!!editing} />
          </Form.Item>
          <Form.Item name="phone" label="手机">
            <Input />
          </Form.Item>
          <Form.Item name="dept" label="部门" rules={[{ required: true, message: '请选择部门' }]}>
            <Select
              placeholder="选择部门（在系统配置中维护）"
              options={(depts || []).map((d) => ({ value: d.value, label: d.value }))}
              showSearch
            />
          </Form.Item>
          <Form.Item name="position" label="职位">
            <Input />
          </Form.Item>
          <Form.Item name="role" label="角色" rules={[{ required: true, message: '请选择角色' }]}>
            <Select options={roleOptions} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="离职交接"
        open={!!obTarget}
        onCancel={() => { setObTarget(null); setObPreview(null) }}
        okText="确认交接并停用"
        okButtonProps={{ disabled: !obSuccessor }}
        confirmLoading={offboardMut.isPending}
        onOk={() => obTarget && offboardMut.mutate({ id: obTarget.id, successor: obSuccessor })}
      >
        <p>该员工名下有数据需在停用前转移给交接人：</p>
        <ul>
          <li>客户：{obPreview?.customers ?? 0} 个</li>
          <li>商单：{obPreview?.deals ?? 0} 个</li>
          <li>合同：{obPreview?.contracts ?? 0} 个</li>
        </ul>
        <Form.Item label="交接人（启用状态的员工）" required style={{ marginTop: 12 }}>
          <Select
            placeholder="选择交接人"
            value={obSuccessor || undefined}
            onChange={setObSuccessor}
            options={successorOptions}
            showSearch
            optionFilterProp="label"
          />
        </Form.Item>
      </Modal>
    </div>
  )
}
