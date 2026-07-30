import { useState } from 'react'
import { Badge, Button, Dropdown, Empty, List, Spin, Tag, App, Popconfirm } from 'antd'
import { BellOutlined } from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { apiListNotifications, apiMarkAllRead, apiMarkNotificationRead, apiUnreadCount, NOTIF_TYPE } from '../api'

export default function NotificationBell() {
  const { message } = App.useApp()
  const qc = useQueryClient()
  const [open, setOpen] = useState(false)

  const { data: cnt, isLoading: cntLoading } = useQuery({
    queryKey: ['notif-unread'],
    queryFn: apiUnreadCount,
    refetchInterval: 60_000,
  })
  const { data: list, isFetching } = useQuery({
    queryKey: ['notif-list', open],
    queryFn: () => apiListNotifications({ page: 1, size: 10 }),
    enabled: open,
  })

  const markRead = async (id: string) => {
    await apiMarkNotificationRead(id)
    qc.invalidateQueries({ queryKey: ['notif-unread'] })
    qc.invalidateQueries({ queryKey: ['notif-list'] })
  }
  const markAll = async () => {
    await apiMarkAllRead()
    message.success('已全部标记为已读')
    qc.invalidateQueries({ queryKey: ['notif-unread'] })
    qc.invalidateQueries({ queryKey: ['notif-list'] })
  }

  const items = list?.items || []
  const unread = cnt?.count || 0

  return (
    <Dropdown
      open={open}
      onOpenChange={setOpen}
      dropdownRender={() => (
        <div style={{ width: 360, background: '#fff', borderRadius: 8, boxShadow: '0 2px 12px rgba(0,0,0,.15)' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '12px 16px', borderBottom: '1px solid #f0f0f0' }}>
            <strong>通知（{unread} 条未读）</strong>
            <Popconfirm title="确认全部标记为已读？" onConfirm={markAll} okText="确认" cancelText="取消">
              <Button type="link" size="small">全部已读</Button>
            </Popconfirm>
          </div>
          <div style={{ maxHeight: 380, overflowY: 'auto' }}>
            {isFetching ? (
              <div style={{ padding: 32, textAlign: 'center' }}><Spin /></div>
            ) : items.length === 0 ? (
              <Empty description="暂无通知" style={{ padding: 32 }} />
            ) : (
              <List
                dataSource={items}
                renderItem={(n) => (
                  <List.Item
                    style={{ padding: '10px 16px', background: n.is_read ? '#fff' : '#f6faff' }}
                    actions={[
                      n.is_read ? null : (
                        <Button key="r" type="link" size="small" onClick={() => markRead(n.id)}>标记已读</Button>
                      ),
                    ].filter(Boolean)}
                  >
                    <List.Item.Meta
                      title={
                        <span>
                          <Tag color={NOTIF_TYPE[n.type]?.color}>{NOTIF_TYPE[n.type]?.label || n.type}</Tag>
                          {n.title}
                        </span>
                      }
                      description={n.content}
                    />
                  </List.Item>
                )}
              />
            )}
          </div>
        </div>
      )}
    >
      <Badge count={unread} size="small" offset={[-2, 4]}>
        <BellOutlined style={{ fontSize: 18, cursor: 'pointer', color: cntLoading ? '#bbb' : '#333' }} />
      </Badge>
    </Dropdown>
  )
}
