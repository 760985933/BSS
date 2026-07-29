import { useState } from 'react'
import { Card, Typography, Tag, Input, Button, Space, Popconfirm, App } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { apiListDicts, apiAddDict, apiRemoveDict } from '../api'

// 系统配置页（admin）：一期仅部门枚举维护；行业/来源/等级等字典后续模块上线时复用此页
export default function System() {
  const { message } = App.useApp()
  const qc = useQueryClient()
  const [newDept, setNewDept] = useState('')

  const { data: depts, isLoading } = useQuery({ queryKey: ['dicts', 'dept'], queryFn: () => apiListDicts('dept') })

  const addMut = useMutation({
    mutationFn: (value: string) => apiAddDict('dept', value),
    onSuccess: () => {
      message.success('已添加')
      setNewDept('')
      qc.invalidateQueries({ queryKey: ['dicts', 'dept'] })
    },
  })

  const removeMut = useMutation({
    mutationFn: (id: string) => apiRemoveDict(id),
    onSuccess: () => {
      message.success('已删除')
      qc.invalidateQueries({ queryKey: ['dicts', 'dept'] })
    },
  })

  return (
    <div style={{ maxWidth: 640 }}>
      <Typography.Title level={4}>系统配置</Typography.Title>
      <Card title="部门管理" loading={isLoading}>
        <Typography.Paragraph type="secondary">
          单层部门枚举，员工档案的「部门」字段从这里取值；销售主管的数据权限按部门隔离。部门下仍有员工时不可删除。
        </Typography.Paragraph>
        <Space wrap style={{ marginBottom: 16 }}>
          {(depts || []).map((d) => (
            <Popconfirm key={d.id} title={`删除部门「${d.value}」？`} onConfirm={() => removeMut.mutate(d.id)}>
              <Tag closable closeIcon style={{ padding: '4px 10px', fontSize: 14 }}>{d.value}</Tag>
            </Popconfirm>
          ))}
          {depts?.length === 0 && <Typography.Text type="secondary">尚未配置部门</Typography.Text>}
        </Space>
        <Space.Compact style={{ width: 320 }}>
          <Input
            placeholder="新部门名称"
            value={newDept}
            onChange={(e) => setNewDept(e.target.value)}
            onPressEnter={() => newDept.trim() && addMut.mutate(newDept.trim())}
          />
          <Button
            type="primary"
            icon={<PlusOutlined />}
            loading={addMut.isPending}
            onClick={() => newDept.trim() && addMut.mutate(newDept.trim())}
          >
            添加
          </Button>
        </Space.Compact>
      </Card>
    </div>
  )
}
