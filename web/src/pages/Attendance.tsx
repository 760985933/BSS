import { useEffect, useState } from 'react'
import {
  Tabs, Table, Tag, Button, Space, Modal, Form, Input, InputNumber, Select, Popconfirm, App, DatePicker,
} from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, CheckOutlined, CloseOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import dayjs from 'dayjs'
import {
  apiListSchedules, apiCreateSchedule, apiUpdateSchedule, apiDeleteSchedule,
  apiListLeaveRequests, apiCreateLeaveRequest, apiDeleteLeaveRequest, apiDecideLeaveRequest,
  apiListAttendances, apiUpsertAttendance, apiGenerateAttendance, apiDeleteAttendance,
  apiListEmployees,
  SCHEDULE_SHIFT, WEEKDAY_LABEL, LEAVE_TYPE, LEAVE_STATUS, ATT_STATUS,
  AttendanceSchedule, LeaveRequest, Attendance as AttendanceRecord, ScheduleInput, LeaveRequestInput, AttendanceInput, Employee,
} from '../api'

const shiftOptions = Object.entries(SCHEDULE_SHIFT).map(([v, label]) => ({ value: v, label }))
const weekdayOptions = [1, 2, 3, 4, 5, 6, 7].map((d) => ({ value: d, label: WEEKDAY_LABEL[d] }))
const leaveTypeOptions = Object.entries(LEAVE_TYPE).map(([v, label]) => ({ value: v, label }))
const attStatusOptions = Object.entries(ATT_STATUS).map(([v, m]) => ({ value: v, label: m.label }))

export default function Attendance() {
  const { message } = App.useApp()
  const qc = useQueryClient()
  const [schOpen, setSchOpen] = useState(false)
  const [schTarget, setSchTarget] = useState<AttendanceSchedule | null>(null)
  const [schForm] = Form.useForm()

  const [lvOpen, setLvOpen] = useState(false)
  const [lvForm] = Form.useForm()

  const [decTarget, setDecTarget] = useState<LeaveRequest | null>(null)
  const [decForm] = Form.useForm()

  const [attOpen, setAttOpen] = useState(false)
  const [attTarget, setAttTarget] = useState<AttendanceRecord | null>(null)
  const [attForm] = Form.useForm()
  const [genDate, setGenDate] = useState<dayjs.Dayjs | null>(dayjs())

  const { data: schedules, isLoading: schLoading } = useQuery({ queryKey: ['schedules'], queryFn: () => apiListSchedules() })
  const { data: leaves, isLoading: lvLoading } = useQuery({ queryKey: ['leave-requests'], queryFn: () => apiListLeaveRequests() })
  const { data: attendances, isLoading: attLoading } = useQuery({ queryKey: ['attendances'], queryFn: () => apiListAttendances() })
  const { data: emps } = useQuery({ queryKey: ['employees-all'], queryFn: () => apiListEmployees() })

  useEffect(() => {
    if (schOpen) {
      schForm.resetFields()
      if (schTarget) schForm.setFieldsValue(schTarget)
    }
  }, [schOpen, schTarget])
  useEffect(() => {
    if (attOpen) {
      attForm.resetFields()
      if (attTarget) attForm.setFieldsValue({ ...attTarget, date: attTarget.date ? dayjs(attTarget.date) : null })
    }
  }, [attOpen, attTarget])

  const invalidate = () => {
    qc.invalidateQueries({ queryKey: ['schedules'] })
    qc.invalidateQueries({ queryKey: ['leave-requests'] })
    qc.invalidateQueries({ queryKey: ['attendances'] })
  }

  const schMut = useMutation({
    mutationFn: (data: ScheduleInput & { id?: string }) =>
      schTarget ? apiUpdateSchedule(schTarget.id, data) : apiCreateSchedule(data),
    onSuccess: () => { message.success('已保存'); setSchOpen(false); setSchTarget(null); invalidate() },
    onError: (e: any) => message.error(e?.response?.data?.message || '操作失败'),
  })
  const delSch = useMutation({
    mutationFn: (id: string) => apiDeleteSchedule(id),
    onSuccess: () => { message.success('已删除'); invalidate() },
  })

  const lvMut = useMutation({
    mutationFn: (data: LeaveRequestInput) => apiCreateLeaveRequest(data),
    onSuccess: () => { message.success('已提交'); setLvOpen(false); invalidate() },
    onError: (e: any) => message.error(e?.response?.data?.message || '提交失败'),
  })
  const delLv = useMutation({
    mutationFn: (id: string) => apiDeleteLeaveRequest(id),
    onSuccess: () => { message.success('已删除'); invalidate() },
  })
  const decMut = useMutation({
    mutationFn: ({ id, approve, reason }: { id: string; approve: boolean; reason: string }) =>
      apiDecideLeaveRequest(id, approve, reason),
    onSuccess: () => { message.success('已审批'); setDecTarget(null); invalidate() },
    onError: (e: any) => message.error(e?.response?.data?.message || '审批失败'),
  })

  const attMut = useMutation({
    mutationFn: (data: AttendanceInput & { id?: string }) => apiUpsertAttendance(data),
    onSuccess: () => { message.success('已登记'); setAttOpen(false); setAttTarget(null); invalidate() },
    onError: (e: any) => message.error(e?.response?.data?.message || '操作失败'),
  })
  const delAtt = useMutation({
    mutationFn: (id: string) => apiDeleteAttendance(id),
    onSuccess: () => { message.success('已删除'); invalidate() },
  })
  const genMut = useMutation({
    mutationFn: (date: string) => apiGenerateAttendance(date),
    onSuccess: (r) => { message.success(`生成考勤 ${r.created} 条`); invalidate() },
    onError: (e: any) => message.error(e?.response?.data?.message || '生成失败'),
  })

  const empOptions = (emps?.list || []).map((e: Employee) => ({ value: e.id, label: e.name }))

  const onSchSubmit = () => {
    schForm.validateFields().then((v: any) => {
      schMut.mutate({
        employee_id: v.employee_id,
        weekday: v.weekday,
        start_time: v.start_time,
        end_time: v.end_time,
        shift_type: v.shift_type || 'regular',
      })
    })
  }
  const onLvSubmit = () => {
    lvForm.validateFields().then((v: any) => {
      lvMut.mutate({
        employee_id: v.employee_id,
        type: v.type,
        start_date: v.range?.[0]?.format('YYYY-MM-DD'),
        end_date: v.range?.[1]?.format('YYYY-MM-DD'),
        reason: v.reason,
      })
    })
  }
  const onAttSubmit = () => {
    attForm.validateFields().then((v: any) => {
      attMut.mutate({
        employee_id: v.employee_id,
        date: v.date?.format('YYYY-MM-DD'),
        schedule_id: v.schedule_id || undefined,
        status: v.status,
        leave_type: v.leave_type || undefined,
        remark: v.remark,
      })
    })
  }

  const schColumns = [
    { title: '员工', key: 'employee', render: (_: any, r: AttendanceSchedule) => r.employee?.name || '-' },
    { title: '星期', dataIndex: 'weekday', key: 'weekday', render: (d: number) => WEEKDAY_LABEL[d] || d },
    { title: '上班', dataIndex: 'start_time', key: 'start_time' },
    { title: '下班', dataIndex: 'end_time', key: 'end_time' },
    { title: '班别', dataIndex: 'shift_type', key: 'shift_type', render: (s: string) => SCHEDULE_SHIFT[s] || s },
    {
      title: '操作', key: 'op', render: (_: any, r: AttendanceSchedule) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => { setSchTarget(r); setSchOpen(true) }}>编辑</Button>
          <Popconfirm title="确认删除？" onConfirm={() => delSch.mutate(r.id)}>
            <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  const lvColumns = [
    { title: '单号', dataIndex: 'code', key: 'code' },
    { title: '员工', key: 'employee', render: (_: any, r: LeaveRequest) => r.employee?.name || '-' },
    { title: '类型', dataIndex: 'type', key: 'type', render: (t: string) => LEAVE_TYPE[t] || t },
    { title: '起', dataIndex: 'start_date', key: 'start_date', render: (s?: string) => s ? s.slice(0, 10) : '-' },
    { title: '止', dataIndex: 'end_date', key: 'end_date', render: (s?: string) => s ? s.slice(0, 10) : '-' },
    { title: '原因', dataIndex: 'reason', key: 'reason', ellipsis: true },
    { title: '状态', dataIndex: 'status', key: 'status', render: (s: string) => <Tag color={LEAVE_STATUS[s]?.color}>{LEAVE_STATUS[s]?.label}</Tag> },
    { title: '审批人', key: 'approver', render: (_: any, r: LeaveRequest) => r.approver?.name || '-' },
    {
      title: '操作', key: 'op', render: (_: any, r: LeaveRequest) => (
        <Space>
          {r.status === 'pending' && (
            <>
              <Button size="small" type="link" icon={<CheckOutlined />} onClick={() => decMut.mutate({ id: r.id, approve: true, reason: '' })}>通过</Button>
              <Button size="small" type="link" danger icon={<CloseOutlined />} onClick={() => setDecTarget(r)}>驳回</Button>
            </>
          )}
          <Popconfirm title="确认删除？" onConfirm={() => delLv.mutate(r.id)}>
            <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  const attColumns = [
    { title: '日期', dataIndex: 'date', key: 'date' },
    { title: '员工', key: 'employee', render: (_: any, r: AttendanceRecord) => r.employee?.name || '-' },
    { title: '状态', dataIndex: 'status', key: 'status', render: (s: string) => <Tag color={ATT_STATUS[s]?.color}>{ATT_STATUS[s]?.label}</Tag> },
    { title: '请假类型', dataIndex: 'leave_type', key: 'leave_type', render: (t?: string) => (t ? (LEAVE_TYPE[t] || t) : '-') },
    { title: '备注', dataIndex: 'remark', key: 'remark', ellipsis: true },
    {
      title: '操作', key: 'op', render: (_: any, r: AttendanceRecord) => (
        <Space>
          <Button size="small" icon={<EditOutlined />} onClick={() => { setAttTarget(r); setAttOpen(true) }}>编辑</Button>
          <Popconfirm title="确认删除？" onConfirm={() => delAtt.mutate(r.id)}>
            <Button size="small" danger icon={<DeleteOutlined />}>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <Tabs defaultActiveKey="schedule" items={[
        {
          key: 'schedule', label: '排班',
          children: (
            <div>
              <Space style={{ marginBottom: 12 }}>
                <Button type="primary" icon={<PlusOutlined />} onClick={() => { setSchTarget(null); setSchOpen(true) }}>新建排班</Button>
              </Space>
              <Table rowKey="id" loading={schLoading} dataSource={schedules || []} columns={schColumns} size="small" />
            </div>
          ),
        },
        {
          key: 'leave', label: '请假',
          children: (
            <div>
              <Space style={{ marginBottom: 12 }}>
                <Button type="primary" icon={<PlusOutlined />} onClick={() => { setLvOpen(true) }}>提交请假</Button>
              </Space>
              <Table rowKey="id" loading={lvLoading} dataSource={leaves || []} columns={lvColumns} size="small" />
            </div>
          ),
        },
        {
          key: 'att', label: '考勤记录',
          children: (
            <div>
              <Space style={{ marginBottom: 12 }} wrap>
                <DatePicker value={genDate} onChange={setGenDate} />
                <Button type="primary" icon={<PlusOutlined />} loading={genMut.isPending}
                  onClick={() => genDate && genMut.mutate(genDate.format('YYYY-MM-DD'))}>按排班生成当日考勤</Button>
                <Button icon={<PlusOutlined />} onClick={() => { setAttTarget(null); setAttOpen(true) }}>手动登记</Button>
              </Space>
              <Table rowKey="id" loading={attLoading} dataSource={attendances || []} columns={attColumns} size="small" />
            </div>
          ),
        },
      ]} />

      <Modal title={schTarget ? '编辑排班' : '新建排班'} open={schOpen} onOk={onSchSubmit}
        confirmLoading={schMut.isPending} onCancel={() => { setSchOpen(false); setSchTarget(null) }} destroyOnClose>
        <Form form={schForm} layout="vertical">
          <Form.Item label="员工" name="employee_id" rules={[{ required: true, message: '请选择员工' }]}>
            <Select options={empOptions} showSearch optionFilterProp="label" placeholder="选择员工" />
          </Form.Item>
          <Form.Item label="星期" name="weekday" rules={[{ required: true, message: '请选择星期' }]}>
            <Select options={weekdayOptions} />
          </Form.Item>
          <Form.Item label="上班时间" name="start_time" rules={[{ required: true, message: '必填' }]}>
            <Input placeholder="09:00" />
          </Form.Item>
          <Form.Item label="下班时间" name="end_time" rules={[{ required: true, message: '必填' }]}>
            <Input placeholder="18:00" />
          </Form.Item>
          <Form.Item label="班别" name="shift_type" initialValue="regular">
            <Select options={shiftOptions} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal title="提交请假" open={lvOpen} onOk={onLvSubmit}
        confirmLoading={lvMut.isPending} onCancel={() => setLvOpen(false)} destroyOnClose>
        <Form form={lvForm} layout="vertical">
          <Form.Item label="员工" name="employee_id" rules={[{ required: true, message: '请选择员工' }]}>
            <Select options={empOptions} showSearch optionFilterProp="label" placeholder="选择员工" />
          </Form.Item>
          <Form.Item label="请假类型" name="type" rules={[{ required: true, message: '请选择类型' }]}>
            <Select options={leaveTypeOptions} />
          </Form.Item>
          <Form.Item label="起止日期" name="range" rules={[{ required: true, message: '请选择起止日期' }]}>
            <DatePicker.RangePicker style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label="事由" name="reason"><Input.TextArea rows={3} placeholder="可选" /></Form.Item>
        </Form>
      </Modal>

      <Modal title="驳回请假" open={!!decTarget} onOk={() => {
        const reason = decForm.getFieldValue('reason')
        if (!reason || !decTarget) { message.warning('请填写驳回原因'); return }
        decMut.mutate({ id: decTarget.id, approve: false, reason })
      }} confirmLoading={decMut.isPending} onCancel={() => setDecTarget(null)} destroyOnClose>
        <Form form={decForm} layout="vertical">
          <Form.Item label="驳回原因" name="reason" rules={[{ required: true, message: '请填写驳回原因' }]}>
            <Input.TextArea rows={3} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal title={attTarget ? '编辑考勤' : '手动登记考勤'} open={attOpen} onOk={onAttSubmit}
        confirmLoading={attMut.isPending} onCancel={() => { setAttOpen(false); setAttTarget(null) }} destroyOnClose>
        <Form form={attForm} layout="vertical">
          <Form.Item label="员工" name="employee_id" rules={[{ required: true, message: '请选择员工' }]}>
            <Select options={empOptions} showSearch optionFilterProp="label" placeholder="选择员工" />
          </Form.Item>
          <Form.Item label="日期" name="date" rules={[{ required: true, message: '请选择日期' }]}>
            <DatePicker style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label="出勤状态" name="status" initialValue="normal" rules={[{ required: true }]}>
            <Select options={attStatusOptions} />
          </Form.Item>
          <Form.Item label="请假类型(请假时填)" name="leave_type">
            <Select options={leaveTypeOptions} allowClear placeholder="非请假可留空" />
          </Form.Item>
          <Form.Item label="备注" name="remark"><Input.TextArea rows={2} /></Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
