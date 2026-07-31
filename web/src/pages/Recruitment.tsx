import { useEffect, useState } from 'react'
import {
  Tabs, Table, Tag, Typography, Button, Space, Modal, Form, Input, InputNumber, Select, Popconfirm, App, Card, Progress, Empty,
} from 'antd'
import {
  PlusOutlined, EditOutlined, DeleteOutlined, SolutionOutlined, StepForwardOutlined,
} from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  apiListJobPosts, apiCreateJobPost, apiUpdateJobPost, apiDeleteJobPost,
  apiListCandidates, apiCreateCandidate, apiUpdateCandidate, apiDeleteCandidate, apiAdvanceCandidate, apiCandidatesFunnel,
  JOB_STATUS, CANDIDATE_STAGE,
  JobPost, Candidate, JobPostInput, CandidateInput,
} from '../api'

const stageOptions = Object.entries(CANDIDATE_STAGE).map(([v, m]) => ({ value: v, label: m.label }))
const jobStatusOptions = Object.entries(JOB_STATUS).map(([v, m]) => ({ value: v, label: m.label }))

function totalCount(funnel: { count: number }[]) {
  return funnel.reduce((s, f) => s + (f.count || 0), 0)
}

export default function Recruitment() {
  const { message } = App.useApp()
  const qc = useQueryClient()
  const [jobFormOpen, setJobFormOpen] = useState(false)
  const [jobTarget, setJobTarget] = useState<JobPost | null>(null)
  const [candFormOpen, setCandFormOpen] = useState(false)
  const [candTarget, setCandTarget] = useState<Candidate | null>(null)
  const [advanceTarget, setAdvanceTarget] = useState<Candidate | null>(null)
  const [advForm] = Form.useForm()

  const { data: jobs, isLoading: jobsLoading } = useQuery({ queryKey: ['job-posts'], queryFn: () => apiListJobPosts() })
  const { data: candidates, isLoading: candLoading } = useQuery({ queryKey: ['candidates'], queryFn: () => apiListCandidates() })
  const { data: funnel } = useQuery({ queryKey: ['funnel'], queryFn: () => apiCandidatesFunnel() })

  useEffect(() => {
    if (advanceTarget) {
      advForm.resetFields()
      advForm.setFieldsValue({ stage: '' })
    }
  }, [advanceTarget])

  const invalidate = () => {
    qc.invalidateQueries({ queryKey: ['job-posts'] })
    qc.invalidateQueries({ queryKey: ['candidates'] })
    qc.invalidateQueries({ queryKey: ['funnel'] })
  }

  // ---- 职位 ----
  const jobMut = useMutation({
    mutationFn: (data: JobPostInput & { id?: string }) =>
      jobTarget ? apiUpdateJobPost(jobTarget.id, data) : apiCreateJobPost(data),
    onSuccess: (r: any) => { message.success(r.message || '已保存'); setJobFormOpen(false); setJobTarget(null); invalidate() },
    onError: (e: any) => message.error(e?.response?.data?.message || '操作失败'),
  })
  const delJob = useMutation({
    mutationFn: (id: string) => apiDeleteJobPost(id),
    onSuccess: (r: any) => { message.success(r.message || '已删除'); invalidate() },
  })

  // ---- 候选人 ----
  const candMut = useMutation({
    mutationFn: (data: CandidateInput & { id?: string }) =>
      candTarget ? apiUpdateCandidate(candTarget.id, data) : apiCreateCandidate(data),
    onSuccess: (r: any) => { message.success(r.message || '已保存'); setCandFormOpen(false); setCandTarget(null); invalidate() },
    onError: (e: any) => message.error(e?.response?.data?.message || '操作失败'),
  })
  const delCand = useMutation({
    mutationFn: (id: string) => apiDeleteCandidate(id),
    onSuccess: (r: any) => { message.success(r.message || '已删除'); invalidate() },
  })
  const advanceMut = useMutation({
    mutationFn: ({ id, stage }: { id: string; stage: string }) => apiAdvanceCandidate(id, stage, true),
    onSuccess: (r: any) => { message.success(r.message || '阶段已更新'); setAdvanceTarget(null); invalidate() },
    onError: (e: any) => message.error(e?.response?.data?.message || '操作失败'),
  })

  return (
    <div>
      <Typography.Title level={4} style={{ marginTop: 0 }}>
        <SolutionOutlined /> 招聘管理
      </Typography.Title>

      <Tabs
        items={[
          {
            key: 'jobs',
            label: '招聘职位',
            children: (
              <Card>
                <Space style={{ marginBottom: 16, justifyContent: 'space-between', width: '100%' }} wrap>
                  <span>共 {jobs?.length || 0} 个职位</span>
                  <Button type="primary" icon={<PlusOutlined />} onClick={() => { setJobTarget(null); setJobFormOpen(true) }}>新建职位</Button>
                </Space>
                <Table<JobPost>
                  rowKey="id" loading={jobsLoading} dataSource={jobs || []} pagination={{ pageSize: 10, showTotal: (t) => `共 ${t} 个` }}
                  columns={[
                    { title: '编号', dataIndex: 'code', width: 140 },
                    { title: '职位', dataIndex: 'title' },
                    { title: '部门', dataIndex: 'dept', render: (d: string) => d || '—' },
                    { title: '编制', dataIndex: 'headcount', width: 80 },
                    { title: '状态', dataIndex: 'status', width: 100, render: (s: string) => <Tag color={JOB_STATUS[s]?.color}>{JOB_STATUS[s]?.label}</Tag> },
                    { title: '负责人', dataIndex: ['owner', 'name'], render: (_: any, r: JobPost) => r.owner?.name || '—' },
                    {
                      title: '操作', key: 'op', width: 160,
                      render: (_: any, r: JobPost) => (
                        <Space size="small">
                          <Button size="small" type="link" icon={<EditOutlined />} onClick={() => { setJobTarget(r); setJobFormOpen(true) }}>编辑</Button>
                          <Popconfirm title="确认删除该职位？其名下候选人将解除关联" onConfirm={() => delJob.mutate(r.id)}>
                            <Button size="small" type="link" danger icon={<DeleteOutlined />}>删除</Button>
                          </Popconfirm>
                        </Space>
                      ),
                    },
                  ]}
                />
              </Card>
            ),
          },
          {
            key: 'candidates',
            label: '候选人',
            children: (
              <Card>
                <Space style={{ marginBottom: 16, justifyContent: 'space-between', width: '100%' }} wrap>
                  <span>共 {candidates?.length || 0} 人</span>
                  <Button type="primary" icon={<PlusOutlined />} onClick={() => { setCandTarget(null); setCandFormOpen(true) }}>添加候选人</Button>
                </Space>
                <Table<Candidate>
                  rowKey="id" loading={candLoading} dataSource={candidates || []} pagination={{ pageSize: 10, showTotal: (t) => `共 ${t} 人` }}
                  columns={[
                    { title: '姓名', dataIndex: 'name' },
                    { title: '电话', dataIndex: 'phone', render: (p: string) => p || '—' },
                    { title: '邮箱', dataIndex: 'email', render: (e: string) => e || '—' },
                    { title: '应聘职位', dataIndex: ['job_post', 'title'], render: (_: any, r: Candidate) => r.job_post?.title || <span style={{ color: '#bbb' }}>未关联</span> },
                    { title: '阶段', dataIndex: 'stage', width: 100, render: (s: string) => <Tag color={CANDIDATE_STAGE[s]?.color}>{CANDIDATE_STAGE[s]?.label}</Tag> },
                    { title: '来源', dataIndex: 'source', render: (s: string) => s || '—' },
                    {
                      title: '操作', key: 'op', width: 200,
                      render: (_: any, r: Candidate) => (
                        <Space size="small">
                          <Button size="small" type="link" icon={<StepForwardOutlined />} onClick={() => setAdvanceTarget(r)}>阶段</Button>
                          <Button size="small" type="link" icon={<EditOutlined />} onClick={() => { setCandTarget(r); setCandFormOpen(true) }}>编辑</Button>
                          <Popconfirm title="确认删除该候选人？" onConfirm={() => delCand.mutate(r.id)}>
                            <Button size="small" type="link" danger icon={<DeleteOutlined />}>删除</Button>
                          </Popconfirm>
                        </Space>
                      ),
                    },
                  ]}
                />
              </Card>
            ),
          },
          {
            key: 'funnel',
            label: '招聘漏斗',
            children: (
              <Card>
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="各阶段候选人分布" style={{ marginBottom: 0 }} />
                <div style={{ marginTop: 16 }}>
                  {(funnel || []).map((f) => (
                    <div key={f.stage} style={{ marginBottom: 12 }}>
                      <Space style={{ width: '100%', justifyContent: 'space-between' }}>
                        <span>{CANDIDATE_STAGE[f.stage]?.label || f.stage}</span>
                        <span>{f.count} 人</span>
                      </Space>
                      <Progress
                        percent={totalCount(funnel || []) ? Math.round((f.count / totalCount(funnel || [])) * 100) : 0}
                        showInfo={false}
                        strokeColor={CANDIDATE_STAGE[f.stage]?.color}
                      />
                    </div>
                  ))}
                </div>
              </Card>
            ),
          },
        ]}
      />

      <JobForm open={jobFormOpen} target={jobTarget}
        onClose={() => { setJobFormOpen(false); setJobTarget(null) }}
        onSubmit={(data: any) => jobMut.mutate(data)} submitting={jobMut.isPending} />

      <CandidateForm open={candFormOpen} target={candTarget} jobs={jobs || []}
        onClose={() => { setCandFormOpen(false); setCandTarget(null) }}
        onSubmit={(data: any) => candMut.mutate(data)} submitting={candMut.isPending} />

      <Modal
        title={`阶段流转：${advanceTarget?.name || ''}`}
        open={!!advanceTarget}
        onCancel={() => setAdvanceTarget(null)}
        onOk={() => advForm.validateFields().then((v: any) => { if (advanceTarget) advanceMut.mutate({ id: advanceTarget.id, stage: v.stage }) })}
        okText="确认流转"
      >
        <Form form={advForm} layout="vertical">
          <Form.Item name="stage" label="目标阶段" rules={[{ required: true, message: '请选择目标阶段' }]}>
            <Select options={stageOptions.filter((o) => o.value !== advanceTarget?.stage)} placeholder="请选择" />
          </Form.Item>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
            回退或跳级将直接生效（已二次确认）。已入职 / 已淘汰为终态，不可再变更。
          </Typography.Paragraph>
        </Form>
      </Modal>
    </div>
  )
}

// 职位表单
function JobForm({ open, target, onClose, onSubmit, submitting }: any) {
  const [form] = Form.useForm()
  useEffect(() => {
    if (open) {
      form.resetFields()
      if (target) form.setFieldsValue(target)
    }
  }, [open, target])
  return (
    <Modal title={target ? '编辑职位' : '新建职位'} open={open} onCancel={onClose}
      onOk={() => form.validateFields().then((v) => onSubmit({ ...v, id: target?.id }))}
      confirmLoading={submitting} okText="保存">
      <Form form={form} layout="vertical">
        <Form.Item name="title" label="职位名称" rules={[{ required: true, message: '请输入职位名称' }]}>
          <Input placeholder="如：后端工程师" />
        </Form.Item>
        <Form.Item name="dept" label="部门"><Input placeholder="如：技术部" /></Form.Item>
        <Form.Item name="headcount" label="编制人数"><InputNumber min={0} style={{ width: '100%' }} /></Form.Item>
        <Form.Item name="status" label="状态" initialValue="open"><Select options={jobStatusOptions} /></Form.Item>
        <Form.Item name="description" label="职位描述"><Input.TextArea rows={3} /></Form.Item>
      </Form>
    </Modal>
  )
}

// 候选人表单
function CandidateForm({ open, target, jobs, onClose, onSubmit, submitting }: any) {
  const [form] = Form.useForm()
  useEffect(() => {
    if (open) {
      form.resetFields()
      if (target) form.setFieldsValue({ ...target, job_post_id: target.job_post_id || undefined })
    }
  }, [open, target])
  return (
    <Modal title={target ? '编辑候选人' : '添加候选人'} open={open} onCancel={onClose}
      onOk={() => form.validateFields().then((v) => onSubmit({ ...v, id: target?.id }))}
      confirmLoading={submitting} okText="保存">
      <Form form={form} layout="vertical">
        <Form.Item name="name" label="姓名" rules={[{ required: true, message: '请输入姓名' }]}><Input /></Form.Item>
        <Form.Item name="job_post_id" label="应聘职位">
          <Select allowClear placeholder="不关联" options={(jobs || []).map((j: JobPost) => ({ value: j.id, label: `${j.code} ${j.title}` }))} />
        </Form.Item>
        <Form.Item name="phone" label="电话"><Input /></Form.Item>
        <Form.Item name="email" label="邮箱"><Input /></Form.Item>
        <Form.Item name="source" label="来源"><Input placeholder="如：BOSS直聘 / 内推" /></Form.Item>
        <Form.Item name="resume_url" label="简历链接"><Input /></Form.Item>
      </Form>
    </Modal>
  )
}
