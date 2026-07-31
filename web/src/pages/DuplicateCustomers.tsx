import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Card, Radio, List, Button, Tag, Empty, Space, message, Modal, Spin } from 'antd'
import { apiFindDuplicates, apiMergeCustomers, DuplicateGroup } from '../api'

export default function DuplicateCustomers() {
  const qc = useQueryClient()
  const { data, isLoading } = useQuery({ queryKey: ['duplicates'], queryFn: apiFindDuplicates })
  // 每个重复组选中的主客户 id
  const [primary, setPrimary] = useState<Record<number, string>>({})

  const doMerge = (g: DuplicateGroup, idx: number) => {
    const pid = primary[idx]
    if (!pid) {
      message.warning('请先选择主客户')
      return
    }
    const primaryName = g.customers.find((c) => c.id === pid)?.name ?? ''
    const others = g.customers.filter((c) => c.id !== pid)
    if (others.length === 0) {
      message.warning('请选择至少一个从客户')
      return
    }
    Modal.confirm({
      title: '确认合并客户',
      content: `将把其余 ${others.length} 个客户并入主客户「${primaryName}」。从客户的联系人、商单、合同将迁移到主客户，从客户随后被软删除（不可撤销）。`,
      okText: '合并',
      okButtonProps: { danger: true },
      onOk: async () => {
        for (const o of others) {
          await apiMergeCustomers(Number(pid), Number(o.id))
        }
        message.success('合并完成')
        setPrimary((p) => {
          const n = { ...p }
          delete n[idx]
          return n
        })
        qc.invalidateQueries({ queryKey: ['duplicates'] })
      },
    })
  }

  if (isLoading) return <Spin style={{ margin: 40 }} />
  if (!data || data.length === 0) {
    return <Empty description="未发现重复客户（基于共享手机 / 邮箱识别）" style={{ marginTop: 80 }} />
  }

  return (
    <div style={{ padding: 24 }}>
      <h2>客户查重合并</h2>
      <p style={{ color: '#888' }}>
        基于联系人手机 / 邮箱的硬证据识别疑似重复客户。合并会把从客户的联系人、商单、合同迁移到主客户后软删从客户；
        回款计划与记录随合同自动归属主客户。
      </p>
      {data.map((g, idx) => (
        <Card
          key={`${g.field}-${g.value}-${idx}`}
          style={{ marginBottom: 16 }}
          title={`重复${g.field === 'phone' ? '手机' : '邮箱'}：${g.value}`}
        >
          <Radio.Group
            value={primary[idx]}
            onChange={(e) => setPrimary((p) => ({ ...p, [idx]: e.target.value }))}
          >
            <List
              dataSource={g.customers}
              renderItem={(c) => (
                <List.Item>
                  <Radio value={c.id}>
                    <Space>
                      <span>{c.name}</span>
                      {c.owner?.name && <Tag>{c.owner.name}</Tag>}
                      {c.level && <Tag color="blue">{c.level}</Tag>}
                      {c.industry && <Tag color="default">{c.industry}</Tag>}
                    </Space>
                  </Radio>
                </List.Item>
              )}
            />
          </Radio.Group>
          <Button
            type="primary"
            danger
            disabled={!primary[idx]}
            onClick={() => doMerge(g, idx)}
            style={{ marginTop: 8 }}
          >
            合并到主客户
          </Button>
        </Card>
      ))}
    </div>
  )
}
