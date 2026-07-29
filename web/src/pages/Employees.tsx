import { Table, Tag, Typography } from 'antd'
import { useQuery } from '@tanstack/react-query'
import { apiListEmployees, Employee } from '../api'

const roleMap: Record<string, { label: string; color: string }> = {
  admin: { label: '管理员', color: 'red' },
  sales: { label: '销售', color: 'blue' },
  sales_lead: { label: '销售主管', color: 'geekblue' },
  finance: { label: '财务', color: 'green' },
  hr: { label: 'HR', color: 'purple' },
}

export default function Employees() {
  const { data, isLoading } = useQuery({ queryKey: ['employees'], queryFn: apiListEmployees })

  return (
    <div>
      <Typography.Title level={4}>员工列表</Typography.Title>
      <Table<Employee>
        rowKey="id"
        loading={isLoading}
        dataSource={data?.list || []}
        pagination={false}
        columns={[
          { title: '姓名', dataIndex: 'name' },
          { title: '邮箱', dataIndex: 'email' },
          { title: '部门', dataIndex: 'dept', render: (v) => v || '-' },
          { title: '职位', dataIndex: 'position', render: (v) => v || '-' },
          {
            title: '角色',
            dataIndex: 'role',
            render: (v: string) => <Tag color={roleMap[v]?.color}>{roleMap[v]?.label || v}</Tag>,
          },
          {
            title: '状态',
            dataIndex: 'status',
            render: (v: string) => (v === 'active' ? <Tag color="success">在职</Tag> : <Tag>停用</Tag>),
          },
        ]}
      />
    </div>
  )
}
