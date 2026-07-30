import { useState } from 'react'
import { Layout, Menu, Dropdown, Avatar, Modal, Form, Input, App } from 'antd'
import {
  DashboardOutlined,
  TeamOutlined,
  ShopOutlined,
  RiseOutlined,
  FileDoneOutlined,
  DollarOutlined,
  SettingOutlined,
  LogoutOutlined,
  KeyOutlined,
  UserOutlined,
} from '@ant-design/icons'
import { Outlet, useNavigate, useLocation } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { apiChangePassword, apiMe } from '../api'
import { useAuth } from '../auth/AuthContext'
import NotificationBell from '../components/NotificationBell'

const { Sider, Header, Content } = Layout

const roleLabel: Record<string, string> = {
  admin: '管理员',
  sales: '销售',
  sales_lead: '销售主管',
  finance: '财务',
  hr: 'HR',
}

export default function MainLayout() {
  const nav = useNavigate()
  const loc = useLocation()
  const { message } = App.useApp()
  const { logout, mustChangePwd, clearMustChange } = useAuth()
  const [pwdOpen, setPwdOpen] = useState(false)
  const [form] = Form.useForm()

  const { data: me } = useQuery({ queryKey: ['me'], queryFn: apiMe })

  // 首启 admin 强制改密
  const forceChange = mustChangePwd

  const doLogout = () => {
    logout()
    nav('/login', { replace: true })
  }

  const submitPwd = async () => {
    const v = await form.validateFields()
    await apiChangePassword(v.old_password, v.new_password)
    message.success('密码已更新，请重新登录')
    clearMustChange()
    doLogout()
  }

  const menuItems = [
    { key: '/', icon: <DashboardOutlined />, label: '仪表盘' },
    { key: '/customers', icon: <ShopOutlined />, label: '客户' },
    { key: '/deals', icon: <RiseOutlined />, label: '商单' },
    { key: '/contracts', icon: <FileDoneOutlined />, label: '合同' },
    { key: '/payments', icon: <DollarOutlined />, label: '回款' },
    { key: '/employees', icon: <TeamOutlined />, label: '员工' },
    ...(me?.role === 'admin'
      ? [{ key: '/system', icon: <SettingOutlined />, label: '系统配置' }]
      : []),
  ]

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider theme="dark">
        <div style={{ color: '#fff', fontWeight: 600, padding: 16, fontSize: 16 }}>BSS</div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[loc.pathname]}
          items={menuItems}
          onClick={({ key }) => nav(key)}
        />
      </Sider>
      <Layout>
        <Header style={{ background: '#fff', padding: '0 24px', display: 'flex', justifyContent: 'flex-end', alignItems: 'center', gap: 20 }}>
          <NotificationBell />
          <Dropdown
            menu={{
              items: [
                { key: 'pwd', icon: <KeyOutlined />, label: '修改密码', onClick: () => setPwdOpen(true) },
                { key: 'logout', icon: <LogoutOutlined />, label: '退出登录', onClick: doLogout },
              ],
            }}
          >
            <span style={{ cursor: 'pointer' }}>
              <Avatar size="small" icon={<UserOutlined />} style={{ marginRight: 8 }} />
              {me?.name || '...'}
              <span style={{ color: '#999', marginLeft: 8, fontSize: 12 }}>{roleLabel[me?.role || ''] || ''}</span>
            </span>
          </Dropdown>
        </Header>
        <Content style={{ margin: 16, padding: 24, background: '#fff', borderRadius: 8 }}>
          <Outlet />
        </Content>
      </Layout>

      <Modal
        title={forceChange ? '首次登录请修改密码' : '修改密码'}
        open={pwdOpen || forceChange}
        onOk={submitPwd}
        onCancel={() => setPwdOpen(false)}
        cancelButtonProps={{ style: { display: forceChange ? 'none' : undefined } }}
        closable={!forceChange}
        maskClosable={!forceChange}
        keyboard={!forceChange}
        okText="确认修改"
        cancelText="取消"
      >
        <Form form={form} layout="vertical">
          <Form.Item name="old_password" label="原密码" rules={[{ required: true, message: '请输入原密码' }]}>
            <Input.Password />
          </Form.Item>
          <Form.Item
            name="new_password"
            label="新密码"
            rules={[
              { required: true, message: '请输入新密码' },
              { min: 8, message: '新密码长度至少 8 位' },
            ]}
          >
            <Input.Password />
          </Form.Item>
        </Form>
      </Modal>
    </Layout>
  )
}
