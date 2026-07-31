import { useEffect, useState } from 'react'
import {
  Card, Form, Input, InputNumber, Switch, Button, Space, Typography, Table, Tag,
  Select, Alert, App, Divider,
} from 'antd'
import { SendOutlined, SaveOutlined, ReloadOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  apiGetNotifySettings, apiUpdateNotifySettings, apiTestNotifyChannel, apiListNotifyLogs,
  type NotifySettings, type NotifyLog, type NotifyChannel,
} from '../api'

const TYPE_OPTIONS = [
  { value: 'contract_expiring', label: '合同到期' },
  { value: 'payment_overdue', label: '回款逾期' },
]

// 通知渠道配置（admin）：站内信之外的邮件 / 企业微信外发。
// 两个渠道默认关闭，关闭时提醒扫描不会产生任何外发行为。
export default function NotifyChannels() {
  const { message } = App.useApp()
  const qc = useQueryClient()
  const [form] = Form.useForm()
  const [testTo, setTestTo] = useState('')

  const { data: settings, isLoading } = useQuery({
    queryKey: ['notify-settings'],
    queryFn: apiGetNotifySettings,
  })

  useEffect(() => {
    if (!settings) return
    form.setFieldsValue({
      ...settings,
      types: settings.types ? settings.types.split(',').map((s) => s.trim()).filter(Boolean) : [],
    })
  }, [settings, form])

  const saveMut = useMutation({
    mutationFn: (v: Partial<NotifySettings> & { types?: string[] }) =>
      apiUpdateNotifySettings({ ...v, types: (v.types || []).join(',') }),
    onSuccess: () => {
      message.success('配置已保存')
      qc.invalidateQueries({ queryKey: ['notify-settings'] })
    },
  })

  const testMut = useMutation({
    mutationFn: ({ channel, to }: { channel: NotifyChannel; to?: string }) =>
      apiTestNotifyChannel(channel, to),
    onSuccess: (r) => {
      message.success(r.message || '已发送')
      qc.invalidateQueries({ queryKey: ['notify-logs'] })
    },
    onSettled: () => qc.invalidateQueries({ queryKey: ['notify-logs'] }),
  })

  return (
    <div style={{ maxWidth: 960 }}>
      <Typography.Title level={4}>通知渠道</Typography.Title>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="站内通知始终生效；这里配置的是额外的外发渠道"
        description="每日 09:00 的提醒扫描（合同到期 / 回款逾期）生成站内信后，会按下方开关同步推送到邮件与企业微信群。渠道关闭时不产生任何外发请求。测试前请先保存配置。"
      />

      <Form
        form={form}
        layout="vertical"
        disabled={isLoading}
        onFinish={(v) => saveMut.mutate(v)}
        initialValues={{ smtp_port: 465, smtp_tls: true, types: [] }}
      >
        <Card title="邮件（SMTP）" style={{ marginBottom: 16 }}>
          <Form.Item name="email_enabled" label="启用邮件通知" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Space size="large" align="start" wrap>
            <Form.Item name="smtp_host" label="SMTP 主机" style={{ width: 260 }}>
              <Input placeholder="smtp.exmail.qq.com" />
            </Form.Item>
            <Form.Item name="smtp_port" label="端口" style={{ width: 120 }}>
              <InputNumber min={1} max={65535} style={{ width: '100%' }} />
            </Form.Item>
            <Form.Item
              name="smtp_tls"
              label="隐式 TLS"
              valuePropName="checked"
              tooltip="465 端口开启；25/587 端口关闭，会自动尝试 STARTTLS"
            >
              <Switch />
            </Form.Item>
          </Space>
          <Space size="large" align="start" wrap>
            <Form.Item name="smtp_username" label="账号" style={{ width: 260 }}>
              <Input placeholder="通常与发件人相同" autoComplete="off" />
            </Form.Item>
            <Form.Item
              name="smtp_password"
              label="密码 / 授权码"
              style={{ width: 260 }}
              tooltip="保存后仅显示掩码；不修改请保持原样"
            >
              <Input.Password autoComplete="new-password" />
            </Form.Item>
            <Form.Item name="smtp_from" label="发件人" style={{ width: 260 }}>
              <Input placeholder="bss@yourcompany.com" />
            </Form.Item>
          </Space>
          <Divider style={{ margin: '8px 0 16px' }} />
          <Space>
            <Input
              placeholder="测试收件人（留空发给自己）"
              value={testTo}
              onChange={(e) => setTestTo(e.target.value)}
              style={{ width: 260 }}
            />
            <Button
              icon={<SendOutlined />}
              loading={testMut.isPending}
              onClick={() => testMut.mutate({ channel: 'email', to: testTo.trim() })}
            >
              发送测试邮件
            </Button>
          </Space>
        </Card>

        <Card title="企业微信群机器人" style={{ marginBottom: 16 }}>
          <Form.Item name="wecom_enabled" label="启用企业微信通知" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item
            name="wecom_webhook"
            label="Webhook 地址"
            tooltip="企业微信群 → 群机器人 → 添加机器人 → 复制 Webhook 地址"
          >
            <Input placeholder="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=..." />
          </Form.Item>
          <Button
            icon={<SendOutlined />}
            loading={testMut.isPending}
            onClick={() => testMut.mutate({ channel: 'wecom' })}
          >
            发送测试消息
          </Button>
        </Card>

        <Card title="推送范围" style={{ marginBottom: 16 }}>
          <Form.Item
            name="types"
            label="仅推送以下类型"
            tooltip="不选 = 全部类型都推送"
            style={{ maxWidth: 420 }}
          >
            <Select mode="multiple" allowClear placeholder="全部类型" options={TYPE_OPTIONS} />
          </Form.Item>
        </Card>

        <Space style={{ marginBottom: 24 }}>
          <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={saveMut.isPending}>
            保存配置
          </Button>
          <Button onClick={() => qc.invalidateQueries({ queryKey: ['notify-settings'] })}>重置</Button>
        </Space>
      </Form>

      <NotifyLogTable />
    </div>
  )
}

function NotifyLogTable() {
  const qc = useQueryClient()
  const [page, setPage] = useState(1)
  const [channel, setChannel] = useState<string | undefined>()
  const [status, setStatus] = useState<string | undefined>()

  const { data, isFetching } = useQuery({
    queryKey: ['notify-logs', page, channel, status],
    queryFn: () => apiListNotifyLogs({ page, size: 10, channel, status }),
  })

  const columns = [
    {
      title: '时间',
      dataIndex: 'created_at',
      width: 170,
      render: (v: string) => new Date(v).toLocaleString('zh-CN'),
    },
    {
      title: '渠道',
      dataIndex: 'channel',
      width: 100,
      render: (v: string) => (v === 'email' ? '邮件' : '企业微信'),
    },
    { title: '标题', dataIndex: 'title', ellipsis: true },
    { title: '接收方', dataIndex: 'target', width: 200, ellipsis: true },
    {
      title: '结果',
      dataIndex: 'status',
      width: 90,
      render: (v: string) => (v === 'success' ? <Tag color="green">成功</Tag> : <Tag color="red">失败</Tag>),
    },
    { title: '错误', dataIndex: 'error', ellipsis: true },
  ]

  return (
    <Card
      title="外发日志"
      extra={
        <Space>
          <Select
            allowClear placeholder="渠道" style={{ width: 120 }} value={channel}
            onChange={(v) => { setChannel(v); setPage(1) }}
            options={[{ value: 'email', label: '邮件' }, { value: 'wecom', label: '企业微信' }]}
          />
          <Select
            allowClear placeholder="结果" style={{ width: 110 }} value={status}
            onChange={(v) => { setStatus(v); setPage(1) }}
            options={[{ value: 'success', label: '成功' }, { value: 'failed', label: '失败' }]}
          />
          <Button icon={<ReloadOutlined />} onClick={() => qc.invalidateQueries({ queryKey: ['notify-logs'] })} />
        </Space>
      }
    >
      <Table<NotifyLog>
        rowKey="id"
        size="small"
        loading={isFetching}
        columns={columns}
        dataSource={data?.list || []}
        pagination={{
          current: page, pageSize: 10, total: data?.total || 0,
          showTotal: (t) => `共 ${t} 条`, onChange: setPage,
        }}
      />
    </Card>
  )
}
