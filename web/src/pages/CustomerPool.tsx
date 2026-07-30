import { useState } from 'react'
import {
  Table, Tag, Typography, Button, Space, Modal, Input, Select, Popconfirm, App,
  Form, InputNumber, Switch, Alert, Timeline, Empty,
} from 'antd'
import { InboxOutlined, SettingOutlined, ReloadOutlined, HistoryOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import dayjs from 'dayjs'
import {
  apiListPool, apiClaimCustomer, apiAssignFromPool, apiRecyclePool, apiPoolLogs,
  apiGetPoolSettings, apiUpdatePoolSettings, apiListDicts, apiListEmployees, apiMe,
  POOL_ACTION,
} from '../api'
import type { Customer, PoolQuery, PoolSettings, RecycleResult } from '../api'

export default function CustomerPool() {
  const { message } = App.useApp()
  const qc = useQueryClient()
  const [query, setQuery] = useState<PoolQuery>({ page: 1, size: 20 })
  const [assignTarget, setAssignTarget] = useState<Customer | null>(null)
  const [assignOwner, setAssignOwner] = useState<string>()
  const [logTarget, setLogTarget] = useState<Customer | null>(null)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [preview, setPreview] = useState<RecycleResult | null>(null)
  const [form] = Form.useForm<Omit<PoolSettings, 'id' | 'updated_at'>>()

  const { data: me } = useQuery({ queryKey: ['me'], queryFn: apiMe })
  const role = me?.role
  const canClaim = role === 'admin' || role === 'sales' || role === 'sales_lead'
  const canManage = role === 'admin' || role === 'sales_lead'
  const isAdmin = role === 'admin'

  const { data, isLoading } = useQuery({ queryKey: ['pool', query], queryFn: () => apiListPool(query) })
  const { data: industries } = useQuery({ queryKey: ['dicts', 'industry'], queryFn: () => apiListDicts('industry') })
  const { data: sources } = useQuery({ queryKey: ['dicts', 'source'], queryFn: () => apiListDicts('source') })
  const { data: levels } = useQuery({ queryKey: ['dicts', 'level'], queryFn: () => apiListDicts('level') })
  const { data: employees } = useQuery({ queryKey: ['employees'], queryFn: () => apiListEmployees() })
  const { data: settings } = useQuery({ queryKey: ['pool-settings'], queryFn: apiGetPoolSettings })
  const { data: logs } = useQuery({
    queryKey: ['pool-logs', logTarget?.id],
    queryFn: () => apiPoolLogs(logTarget!.id),
    enabled: !!logTarget,
  })

  const invalidate = () => {
    qc.invalidateQueries({ queryKey: ['pool'] })
    qc.invalidateQueries({ queryKey: ['customers'] })
  }

  const claimMut = useMutation({
    mutationFn: (id: string) => apiClaimCustomer(id),
    onSuccess: (r) => { message.success(r.message || '领取成功'); invalidate() },
  })
  const assignMut = useMutation({
    mutationFn: () => apiAssignFromPool(assignTarget!.id, assignOwner!),
    onSuccess: () => {
      message.success('已指派')
      setAssignTarget(null); setAssignOwner(undefined)
      invalidate()
    },
  })
  const recycleMut = useMutation({
    mutationFn: (dry: boolean) => apiRecyclePool(dry),
    onSuccess: (r, dry) => {
      if (dry) {
        setPreview(r)
        if (r.total === 0) message.info('当前没有符合回收条件的客户')
      } else {
        message.success(`已回收 ${r.total} 个客户到公海`)
        setPreview(null)
        invalidate()
      }
    },
  })
  const saveSettingsMut = useMutation({
    mutationFn: (v: Omit<PoolSettings, 'id' | 'updated_at'>) => apiUpdatePoolSettings(v),
    onSuccess: () => {
      message.success('规则已保存')
      setSettingsOpen(false)
      qc.invalidateQueries({ queryKey: ['pool-settings'] })
    },
  })

  const openSettings = () => {
    if (settings) {
      form.setFieldsValue({
        enabled: settings.enabled,
        max_claim_per_sales: settings.max_claim_per_sales,
        idle_days_no_follow: settings.idle_days_no_follow,
        idle_days_no_deal: settings.idle_days_no_deal,
        protect_days: settings.protect_days,
      })
    }
    setSettingsOpen(true)
  }

  const dictOptions = (list?: { value: string }[]) => (list || []).map((d) => ({ value: d.value, label: d.value }))
  const empName = (id: string) => (employees?.list || []).find((e) => e.id === id)?.name || (id === '0' ? '公海' : `#${id}`)

  return (
    <div>
      <Space style={{ marginBottom: 16, width: '100%', justifyContent: 'space-between' }} wrap>
        <Typography.Title level={4} style={{ margin: 0 }}>
          <InboxOutlined /> 客户公海池
        </Typography.Title>
        <Space>
          {canManage && (
            <Button icon={<ReloadOutlined />} loading={recycleMut.isPending}
              onClick={() => recycleMut.mutate(true)}>
              试算回收
            </Button>
          )}
          {isAdmin && (
            <Button icon={<SettingOutlined />} onClick={openSettings}>回收规则</Button>
          )}
        </Space>
      </Space>

      {settings && !settings.enabled && canManage && (
        <Alert style={{ marginBottom: 12 }} type="info" showIcon
          message="自动回收未启用：当前仅能手动释放或回收客户。可在「回收规则」中开启每日自动回收。" />
      )}

      {preview && preview.total > 0 && (
        <Alert
          style={{ marginBottom: 12 }} type="warning" showIcon
          message={`试算结果：${preview.total} 个客户符合回收条件`}
          description={
            <>
              <div style={{ marginBottom: 8 }}>
                {preview.items.slice(0, 10).map((i) => (
                  <Tag key={i.customer_id}>{i.name}（{i.reason}）</Tag>
                ))}
                {preview.items.length > 10 && <Tag>…共 {preview.total} 个</Tag>}
              </div>
              <Space>
                <Popconfirm title={`确认将这 ${preview.total} 个客户回收到公海？`}
                  onConfirm={() => recycleMut.mutate(false)}>
                  <Button size="small" type="primary" danger>确认回收</Button>
                </Popconfirm>
                <Button size="small" onClick={() => setPreview(null)}>取消</Button>
              </Space>
            </>
          }
        />
      )}

      <Space style={{ marginBottom: 12 }} wrap>
        <Input.Search placeholder="客户名称" allowClear style={{ width: 200 }}
          onSearch={(v) => setQuery({ ...query, keyword: v || undefined, page: 1 })} />
        <Select placeholder="行业" allowClear style={{ width: 130 }} options={dictOptions(industries)}
          onChange={(v) => setQuery({ ...query, industry: v, page: 1 })} />
        <Select placeholder="来源" allowClear style={{ width: 130 }} options={dictOptions(sources)}
          onChange={(v) => setQuery({ ...query, source: v, page: 1 })} />
        <Select placeholder="等级" allowClear style={{ width: 110 }} options={dictOptions(levels)}
          onChange={(v) => setQuery({ ...query, level: v, page: 1 })} />
      </Space>

      <Table<Customer>
        rowKey="id"
        loading={isLoading}
        dataSource={data?.list || []}
        locale={{ emptyText: <Empty description="公海暂无客户" /> }}
        pagination={{
          current: query.page, pageSize: query.size, total: data?.total || 0,
          showTotal: (t) => `共 ${t} 条`,
          onChange: (page, size) => setQuery({ ...query, page, size }),
        }}
        columns={[
          { title: '编号', dataIndex: 'code', width: 130 },
          { title: '名称', dataIndex: 'name' },
          { title: '行业', dataIndex: 'industry', render: (v) => v || '-' },
          { title: '来源', dataIndex: 'source', render: (v) => v || '-' },
          { title: '等级', dataIndex: 'level', render: (v) => (v ? <Tag color="gold">{v}</Tag> : '-') },
          {
            title: '入池原因', dataIndex: 'pool_reason', width: 180,
            render: (v: string) => (v ? <Tag color="orange">{v}</Tag> : <Tag>新建未分配</Tag>),
          },
          {
            title: '最后跟进', dataIndex: 'last_followed_at', width: 110,
            render: (v?: string) => (v ? dayjs(v).format('YYYY-MM-DD') : '-'),
          },
          {
            title: '操作', key: 'op', width: 190,
            render: (_: unknown, c: Customer) => (
              <Space size="small">
                {canClaim && (
                  <Popconfirm title={`确认领取「${c.name}」？领取后该客户归入你的名下`}
                    onConfirm={() => claimMut.mutate(c.id)}>
                    <Button size="small" type="link">领取</Button>
                  </Popconfirm>
                )}
                {canManage && (
                  <Button size="small" type="link"
                    onClick={() => { setAssignTarget(c); setAssignOwner(undefined) }}>指派</Button>
                )}
                <Button size="small" type="link" icon={<HistoryOutlined />}
                  onClick={() => setLogTarget(c)}>流水</Button>
              </Space>
            ),
          },
        ]}
      />

      <Modal
        title={`指派客户「${assignTarget?.name}」`}
        open={!!assignTarget}
        onCancel={() => setAssignTarget(null)}
        onOk={() => assignOwner && assignMut.mutate()}
        okButtonProps={{ disabled: !assignOwner }}
        confirmLoading={assignMut.isPending}
        okText="确认指派" cancelText="取消" destroyOnHidden
      >
        <Typography.Paragraph type="secondary">
          指派不受领取上限限制，用于主管分配线索。
        </Typography.Paragraph>
        <Select
          placeholder="选择接收人" style={{ width: '100%' }} showSearch optionFilterProp="label"
          value={assignOwner} onChange={setAssignOwner}
          options={(employees?.list || [])
            .filter((e) => e.status === 'active')
            .map((e) => ({ value: e.id, label: `${e.name}（${e.dept || '未分配部门'}）` }))}
        />
      </Modal>

      <Modal
        title={`公海流水「${logTarget?.name}」`}
        open={!!logTarget}
        onCancel={() => setLogTarget(null)}
        footer={<Button onClick={() => setLogTarget(null)}>关闭</Button>}
        destroyOnHidden
      >
        {logs && logs.length > 0 ? (
          <Timeline
            items={logs.map((l) => ({
              color: POOL_ACTION[l.action]?.color || 'gray',
              children: (
                <div>
                  <Space>
                    <Tag color={POOL_ACTION[l.action]?.color}>{POOL_ACTION[l.action]?.label || l.action}</Tag>
                    <span>{dayjs(l.created_at).format('YYYY-MM-DD HH:mm')}</span>
                  </Space>
                  <div style={{ color: '#888', marginTop: 4 }}>
                    {empName(l.from_owner_id)} → {empName(l.to_owner_id)}
                    {l.reason ? `　${l.reason}` : ''}
                  </div>
                </div>
              ),
            }))}
          />
        ) : (
          <Empty description="暂无流水" />
        )}
      </Modal>

      <Modal
        title="公海回收规则"
        open={settingsOpen}
        onCancel={() => setSettingsOpen(false)}
        onOk={() => form.validateFields().then((v) => saveSettingsMut.mutate(v))}
        confirmLoading={saveSettingsMut.isPending}
        okText="保存" cancelText="取消"
      >
        <Alert style={{ marginBottom: 16 }} type="info" showIcon
          message="有进行中商单或有效合同的客户永不回收；天数填 0 表示停用该条规则。" />
        <Form form={form} layout="horizontal" labelCol={{ span: 12 }} wrapperCol={{ span: 10 }}>
          <Form.Item name="enabled" label="启用每日自动回收" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="max_claim_per_sales" label="单人持有客户上限（0=不限）">
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="idle_days_no_follow" label="超过多少天无跟进则回收">
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="idle_days_no_deal" label="领取后多少天未建商单则回收">
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="protect_days" label="领取保护期（天）">
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}
