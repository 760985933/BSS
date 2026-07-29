import { Button, Card, Form, Input } from 'antd'
import { LockOutlined, MailOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { useState } from 'react'
import { apiLogin } from '../api'
import { useAuth } from '../auth/AuthContext'

export default function Login() {
  const nav = useNavigate()
  const { login } = useAuth()
  const [loading, setLoading] = useState(false)

  const onFinish = async (v: { email: string; password: string }) => {
    setLoading(true)
    try {
      const res = await apiLogin(v.email, v.password)
      login(res.token, res.user.must_change_pwd)
      nav('/', { replace: true })
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', background: '#f0f2f5' }}>
      <Card title="BSS 业务管理系统" style={{ width: 380 }}>
        <Form onFinish={onFinish} size="large">
          <Form.Item name="email" rules={[{ required: true, message: '请输入邮箱' }]}>
            <Input prefix={<MailOutlined />} placeholder="邮箱" autoComplete="username" />
          </Form.Item>
          <Form.Item name="password" rules={[{ required: true, message: '请输入密码' }]}>
            <Input.Password prefix={<LockOutlined />} placeholder="密码" autoComplete="current-password" />
          </Form.Item>
          <Button type="primary" htmlType="submit" block loading={loading}>
            登录
          </Button>
        </Form>
      </Card>
    </div>
  )
}
