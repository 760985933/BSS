import { useEffect, useState } from 'react'
import {
  Table, Tag, Typography, Button, Space, Modal, Form, Input, Select, DatePicker, InputNumber,
  Popconfirm, App, Drawer, Descriptions, Tabs, Alert, Empty,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined, TeamOutlined, EyeOutlined,
} from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import dayjs, { Dayjs } from 'dayjs'
import {
  apiListProjects, apiGetProject, apiCreateProject, apiUpdateProject, apiDeleteProject,
  apiListProjectMembers, apiAddProjectMember, apiUpdateProjectMember, apiRemoveProjectMember,
  apiAddProjectTask, apiUpdateProjectTask, apiRemoveProjectTask,
  PROJECT_STATUS, TASK_STATUS,
  Project, ProjectMember, ProjectTask, ProjectInput, ProjectQuery,
  apiListEmployees, apiListCustomers, apiMe,
} from '../api'
import type { Employee, Customer } from '../api'

const statusOptions = Object.entries(PROJECT_STATUS).map(([v, m]) => ({ value: v, label: m.label }))
const taskStatusOptions = Object.entries(TASK_STATUS).map(([v, m]) => ({ value: v, label: m.label }))

export default function Projects() {
  const { message } = App.useApp()
  const qc = useQueryClient()
  const [query, setQuery] = useState<ProjectQuery>({ page: 1, size: 20 })
  const [editTarget, setEditTarget] = useState<Project | null>(null)
  const [editOpen, setEditOpen] = useState(false)
  const [detailId, setDetailId] = useState<string | null>(null)

  const { data: me } = useQuery({ queryKey: ['me'], queryFn: apiMe })
  const { data, isLoading } = useQuery({
    queryKey: ['projects', query],
    queryFn: () => apiListProjects(query),
  })

  const invalidate = () => qc.invalidateQueries({ queryKey: ['projects'] })

  const delMut = useMutation({
    mutationFn: (id: string) => apiDeleteProject(id),
    onSuccess: (r) => { message.success(r.message || '已删除'); invalidate() },
  })

  return (
    <div>
      <Space style={{ marginBottom: 16, width: '100%', justifyContent: 'space-between' }} wrap>
        <Typography.Title level={4} style={{ margin: 0 }}>
          <TeamOutlined /> 项目 / 交付管理
        </Typography.Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditTarget(null); setEditOpen(true) }}>
          新建项目
        </Button>
      </Space>

      <Space style={{ marginBottom: 12 }} wrap>
        <Input.Search placeholder="项目名称" allowClear style={{ width: 200 }}
          onSearch={(v) => setQuery({ ...query, keyword: v || undefined, page: 1 })} />
        <Select placeholder="状态" allowClear style={{ width: 140 }} options={statusOptions}
          onChange={(v) => setQuery({ ...query, status: v, page: 1 })} />
      </Space>

      <Table<Project>
        rowKey="id" loading={isLoading}
        dataSource={data?.list || []}
        pagination={{
          current: query.page, pageSize: query.size, total: data?.total || 0,
          showTotal: (t) => `共 ${t} 个`,
          onChange: (page, size) => setQuery({ ...query, page, size }),
        }}
        columns={[
          { title: '编号', dataIndex: 'code', width: 140 },
          { title: '名称', dataIndex: 'name' },
          {
            title: '客户', dataIndex: ['customer', 'name'],
            render: (_, p) => p.customer?.name || <span style={{ color: '#bbb' }}>—</span>,
          },
          { title: '负责人', dataIndex: ['owner', 'name'], render: (_, p) => p.owner?.name || '—' },
          {
            title: '状态', dataIndex: 'status', width: 100,
            render: (s: Project['status']) => <Tag color={PROJECT_STATUS[s]?.color}>{PROJECT_STATUS[s]?.label}</Tag>,
          },
          {
            title: '起止', width: 200,
            render: (_, p) => [p.start_date, p.end_date].filter(Boolean).join(' ~ ') || '—',
          },
          {
            title: '操作', key: 'op', width: 180,
            render: (_, p) => (
              <Space size="small">
                <Button size="small" type="link" icon={<EyeOutlined />} onClick={() => setDetailId(p.id)}>详情</Button>
                <Button size="small" type="link" icon={<EditOutlined />} onClick={() => { setEditTarget(p); setEditOpen(true) }}>编辑</Button>
                <Popconfirm title="确认删除该项目？" onConfirm={() => delMut.mutate(p.id)}>
                  <Button size="small" type="link" danger icon={<DeleteOutlined />}>删除</Button>
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />

      <ProjectForm
        open={editOpen}
        target={editTarget}
        defaultOwner={me?.id}
        onClose={() => setEditOpen(false)}
        onDone={() => { setEditOpen(false); invalidate() }}
      />
      <ProjectDetail
        projectId={detailId}
        onClose={() => setDetailId(null)}
        onChanged={() => qc.invalidateQueries({ queryKey: ['projects'] })}
      />
    </div>
  )
}

// ---------- 新建/编辑项目 ----------
function ProjectForm({ open, target, defaultOwner, onClose, onDone }: {
  open: boolean; target: Project | null; defaultOwner?: string; onClose: () => void; onDone: () => void
}) {
  const { message } = App.useApp()
  const [form] = Form.useForm<ProjectInput & { start?: Dayjs | null; end?: Dayjs | null }>()
  const [saving, setSaving] = useState(false)

  const { data: employees } = useQuery({ queryKey: ['employees'], queryFn: () => apiListEmployees() })
  const { data: customers } = useQuery({ queryKey: ['customers-all'], queryFn: () => apiListCustomers({ page: 1, size: 999 }) })

  useEffect(() => {
    if (!open) return
    if (target) {
      form.setFieldsValue({
        name: target.name,
        customer_id: target.customer_id || undefined,
        owner_id: target.owner_id,
        status: target.status,
        start: target.start_date ? dayjs(target.start_date) : null,
        end: target.end_date ? dayjs(target.end_date) : null,
        description: target.description,
      })
    } else {
      form.setFieldsValue({ status: 'planning', owner_id: defaultOwner })
    }
  }, [open, target, defaultOwner, form])

  const submit = async () => {
    const v = await form.validateFields()
    const payload: ProjectInput = {
      name: v.name,
      owner_id: v.owner_id,
      customer_id: v.customer_id || null,
      status: v.status || 'planning',
      start_date: v.start ? v.start.format('YYYY-MM-DD') : undefined,
      end_date: v.end ? v.end.format('YYYY-MM-DD') : undefined,
      description: v.description,
    }
    setSaving(true)
    try {
      if (target) {
        await apiUpdateProject(target.id, payload)
        message.success('已保存')
      } else {
        await apiCreateProject(payload)
        message.success('已创建')
      }
      onDone()
    } catch {
      /* 由拦截器提示 */
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal
      title={target ? '编辑项目' : '新建项目'}
      open={open} onCancel={onClose} onOk={submit}
      confirmLoading={saving} okText="保存" cancelText="取消" destroyOnClose
    >
      <Form form={form} layout="vertical">
        <Form.Item name="name" label="项目名称" rules={[{ required: true, message: '请输入项目名称' }]}>
          <Input placeholder="如：XX 系统交付项目" />
        </Form.Item>
        <Form.Item name="customer_id" label="关联客户">
          <Select placeholder="可选" allowClear showSearch optionFilterProp="label"
            options={(customers?.list || []).map((c: Customer) => ({ value: c.id, label: c.name }))} />
        </Form.Item>
        <Form.Item name="owner_id" label="项目经理" rules={[{ required: true, message: '请选择项目经理' }]}>
          <Select placeholder="选择负责人" showSearch optionFilterProp="label"
            options={(employees?.list || []).filter((e: Employee) => e.status === 'active')
              .map((e: Employee) => ({ value: e.id, label: `${e.name}（${e.dept || '未分配'}）` }))} />
        </Form.Item>
        <Form.Item name="status" label="状态" initialValue="planning">
          <Select options={statusOptions} />
        </Form.Item>
        <Space size="large">
          <Form.Item name="start" label="开始日期"><DatePicker style={{ width: 160 }} /></Form.Item>
          <Form.Item name="end" label="预计结束"><DatePicker style={{ width: 160 }} /></Form.Item>
        </Space>
        <Form.Item name="description" label="项目说明">
          <Input.TextArea rows={3} />
        </Form.Item>
      </Form>
    </Modal>
  )
}

// ---------- 项目详情（任务 / 里程碑 / 成员） ----------
function ProjectDetail({ projectId, onClose, onChanged }: {
  projectId: string | null; onClose: () => void; onChanged: () => void
}) {
  const { data: project, refetch } = useQuery({
    queryKey: ['project-detail', projectId],
    queryFn: () => apiGetProject(projectId!),
    enabled: !!projectId,
  })

  const changed = () => { refetch(); onChanged() }

  if (!projectId) return null

  const tasks = project?.tasks || []
  const milestones = tasks.filter((t) => t.kind === 'milestone')
  const plainTasks = tasks.filter((t) => t.kind === 'task')

  return (
    <Drawer
      width={760} open={!!projectId} onClose={onClose}
      title={project ? `${project.code} ${project.name}` : '项目详情'}
    >
      {project ? (
        <>
          <Descriptions size="small" column={2} bordered style={{ marginBottom: 12 }}>
            <Descriptions.Item label="状态">
              <Tag color={PROJECT_STATUS[project.status]?.color}>{PROJECT_STATUS[project.status]?.label}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label="客户">{project.customer?.name || '—'}</Descriptions.Item>
            <Descriptions.Item label="项目经理">{project.owner?.name || '—'}</Descriptions.Item>
            <Descriptions.Item label="起止">
              {[project.start_date, project.end_date].filter(Boolean).join(' ~ ') || '—'}
            </Descriptions.Item>
            <Descriptions.Item label="说明" span={2}>{project.description || '—'}</Descriptions.Item>
          </Descriptions>

          <Tabs
            items={[
              {
                key: 'tasks', label: `任务（${plainTasks.length}）`,
                children: <TaskTab projectId={project.id} tasks={plainTasks} onChanged={changed} />,
              },
              {
                key: 'milestones', label: `里程碑（${milestones.length}）`,
                children: <TaskTab projectId={project.id} tasks={milestones} onChanged={changed} />,
              },
              {
                key: 'members', label: `成员（${project.members?.length || 0}）`,
                children: <MemberTab projectId={project.id} members={project.members || []} onChanged={changed} />,
              },
            ]}
          />
        </>
      ) : <Empty description="加载中" />}
    </Drawer>
  )
}

// ---------- 任务 / 里程碑子页 ----------
function TaskTab({ projectId, tasks, onChanged }: {
  projectId: string; tasks: ProjectTask[]; onChanged: () => void
}) {
  const { message } = App.useApp()
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<ProjectTask | null>(null)
  const [form] = Form.useForm<{
    title: string; assignee_id?: string; due?: Dayjs | null; status: 'todo' | 'doing' | 'done'; estimate_days: number; kind: 'task' | 'milestone'
  }>()
  const { data: employees } = useQuery({ queryKey: ['employees'], queryFn: () => apiListEmployees() })
  const isMilestone = tasks[0]?.kind === 'milestone'

  const openAdd = () => {
    setEditing(null)
    form.setFieldsValue({ status: 'todo', kind: isMilestone ? 'milestone' : 'task', estimate_days: 0 })
    setOpen(true)
  }
  const openEdit = (t: ProjectTask) => {
    setEditing(t)
    form.setFieldsValue({
      title: t.title, assignee_id: t.assignee_id || undefined, status: t.status,
      due: t.due_date ? dayjs(t.due_date) : null, estimate_days: t.estimate_days, kind: t.kind,
    })
    setOpen(true)
  }

  const submit = async () => {
    const v = await form.validateFields()
    const payload = {
      kind: v.kind, title: v.title, assignee_id: v.assignee_id || null,
      due_date: v.due ? v.due.format('YYYY-MM-DD') : undefined, status: v.status, estimate_days: v.estimate_days,
    }
    try {
      if (editing) {
        await apiUpdateProjectTask(projectId, editing.id, payload)
        message.success('已保存')
      } else {
        await apiAddProjectTask(projectId, payload)
        message.success('已添加')
      }
      setOpen(false); onChanged()
    } catch { /* 拦截器 */ }
  }

  const changeStatus = async (t: ProjectTask, status: 'todo' | 'doing' | 'done') => {
    await apiUpdateProjectTask(projectId, t.id, {
      kind: t.kind, title: t.title, assignee_id: t.assignee_id, due_date: t.due_date, status, estimate_days: t.estimate_days,
    }).then(onChanged).catch(() => {})
  }
  const del = (t: ProjectTask) => apiRemoveProjectTask(projectId, t.id).then(onChanged).catch(() => {})

  return (
    <div>
      <Button type="primary" size="small" icon={<PlusOutlined />} onClick={openAdd} style={{ marginBottom: 12 }}>
        新增{isMilestone ? '里程碑' : '任务'}
      </Button>
      <Table<ProjectTask>
        rowKey="id" size="small" dataSource={tasks} pagination={false}
        locale={{ emptyText: `暂无${isMilestone ? '里程碑' : '任务'}` }}
        columns={[
          { title: '标题', dataIndex: 'title', render: (v, t) => <a onClick={() => openEdit(t)}>{v}</a> },
          { title: '负责人', dataIndex: ['assignee', 'name'], render: (_, t) => t.assignee?.name || '—' },
          { title: '截止', dataIndex: 'due_date', render: (v) => v || '—' },
          { title: '预估人天', dataIndex: 'estimate_days', width: 90, render: (v) => v || 0 },
          {
            title: '状态', dataIndex: 'status', width: 160,
            render: (s: ProjectTask['status'], t) => (
              <Select size="small" value={s} style={{ width: 110 }} options={taskStatusOptions}
                onChange={(v) => changeStatus(t, v)} />
            ),
          },
          {
            title: '', key: 'op', width: 60,
            render: (_, t) => (
              <Popconfirm title="删除？" onConfirm={() => del(t)}>
                <Button size="small" type="link" danger>删</Button>
              </Popconfirm>
            ),
          },
        ]}
      />

      <Modal title={editing ? '编辑' : `新增${isMilestone ? '里程碑' : '任务'}`}
        open={open} onCancel={() => setOpen(false)} onOk={submit} okText="保存" cancelText="取消" destroyOnClose>
        <Form form={form} layout="vertical">
          <Form.Item name="title" label="标题" rules={[{ required: true, message: '请输入标题' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="assignee_id" label="负责人">
            <Select placeholder="可选" allowClear showSearch optionFilterProp="label"
              options={(employees?.list || []).filter((e: Employee) => e.status === 'active')
                .map((e: Employee) => ({ value: e.id, label: `${e.name}（${e.dept || ''}）` }))} />
          </Form.Item>
          <Space size="large">
            <Form.Item name="due" label="截止日期"><DatePicker style={{ width: 160 }} /></Form.Item>
            <Form.Item name="estimate_days" label="预估人天"><InputNumber min={0} step={0.5} style={{ width: 120 }} /></Form.Item>
          </Space>
          <Form.Item name="status" label="状态"><Select options={taskStatusOptions} /></Form.Item>
          <Form.Item name="kind" hidden><Input /></Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

// ---------- 成员(人天)子页 ----------
function MemberTab({ projectId, members, onChanged }: {
  projectId: string; members: ProjectMember[]; onChanged: () => void
}) {
  const { message } = App.useApp()
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<ProjectMember | null>(null)
  const [form] = Form.useForm<{ employee_id?: string; role: string; planned_days: number; actual_days: number }>()
  const { data: employees } = useQuery({ queryKey: ['employees'], queryFn: () => apiListEmployees() })

  const planned = members.reduce((s, m) => s + (m.planned_days || 0), 0)
  const actual = members.reduce((s, m) => s + (m.actual_days || 0), 0)

  const openAdd = () => { setEditing(null); form.setFieldsValue({ role: '', planned_days: 0, actual_days: 0 }); setOpen(true) }
  const openEdit = (m: ProjectMember) => {
    setEditing(m)
    form.setFieldsValue({ employee_id: m.employee_id, role: m.role, planned_days: m.planned_days, actual_days: m.actual_days })
    setOpen(true)
  }

  const submit = async () => {
    const v = await form.validateFields()
    try {
      if (editing) {
        await apiUpdateProjectMember(projectId, editing.id, { role: v.role, planned_days: v.planned_days, actual_days: v.actual_days })
        message.success('已保存')
      } else {
        await apiAddProjectMember(projectId, { employee_id: v.employee_id!, role: v.role, planned_days: v.planned_days, actual_days: v.actual_days })
        message.success('已添加')
      }
      setOpen(false); onChanged()
    } catch { /* 拦截器 */ }
  }
  const del = (m: ProjectMember) => apiRemoveProjectMember(projectId, m.id).then(onChanged).catch(() => {})

  return (
    <div>
      <Alert style={{ marginBottom: 12 }} type="info" showIcon
        message={`人天汇总：计划 ${planned.toFixed(1)} 人天 / 实际 ${actual.toFixed(1)} 人天`} />
      <Button type="primary" size="small" icon={<PlusOutlined />} onClick={openAdd} style={{ marginBottom: 12 }}>添加成员</Button>
      <Table<ProjectMember>
        rowKey="id" size="small" dataSource={members} pagination={false}
        locale={{ emptyText: '暂无成员' }}
        columns={[
          { title: '成员', dataIndex: ['employee', 'name'], render: (_, m) => m.employee?.name || '—' },
          { title: '项目角色', dataIndex: 'role', render: (v) => v || '—' },
          { title: '计划人天', dataIndex: 'planned_days', width: 100, render: (v) => (v || 0).toFixed(1) },
          { title: '实际人天', dataIndex: 'actual_days', width: 100, render: (v) => (v || 0).toFixed(1) },
          {
            title: '操作', key: 'op', width: 120,
            render: (_, m) => (
              <Space size="small">
                <Button size="small" type="link" onClick={() => openEdit(m)}>编辑</Button>
                <Popconfirm title="移除成员？" onConfirm={() => del(m)}>
                  <Button size="small" type="link" danger>移除</Button>
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />

      <Modal title={editing ? '编辑成员' : '添加成员'} open={open} onCancel={() => setOpen(false)} onOk={submit} okText="保存" cancelText="取消" destroyOnClose>
        <Form form={form} layout="vertical">
          <Form.Item name="employee_id" label="成员" rules={[{ required: true, message: '请选择成员' }]}>
            <Select placeholder="选择员工" showSearch optionFilterProp="label" disabled={!!editing}
              options={(employees?.list || []).filter((e: Employee) => e.status === 'active')
                .map((e: Employee) => ({ value: e.id, label: `${e.name}（${e.dept || ''}）` }))} />
          </Form.Item>
          <Form.Item name="role" label="项目角色"><Input placeholder="如：开发 / 测试 / 负责" /></Form.Item>
          <Space size="large">
            <Form.Item name="planned_days" label="计划人天"><InputNumber min={0} step={0.5} style={{ width: 120 }} /></Form.Item>
            <Form.Item name="actual_days" label="实际人天"><InputNumber min={0} step={0.5} style={{ width: 120 }} /></Form.Item>
          </Space>
        </Form>
      </Modal>
    </div>
  )
}
