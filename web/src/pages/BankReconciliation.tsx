import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Alert, Button, Card, Form, Input, InputNumber, Modal, Select, Space, Table, Tag, message } from 'antd'
import {
  apiCreateBankStatements,
  apiListBankStatements,
  apiReconcile,
  apiUnreconcile,
  apiReconciliation,
  BankStatement,
  Reconciliation,
} from '../api'

const yuan = (c: number) => `¥${(c / 100).toFixed(2)}`

interface DraftRow {
  trans_date: string
  counterparty: string
  amount: number
  direction: string
  summary: string
}

export default function BankReconciliation() {
  const qc = useQueryClient()
  const { data: stmts } = useQuery({ queryKey: ['bank-statements'], queryFn: apiListBankStatements })
  const { data: recon } = useQuery({ queryKey: ['reconciliation'], queryFn: apiReconciliation })
  const [draft, setDraft] = useState<DraftRow[]>([])
  const [form] = Form.useForm()
  const [reconOpen, setReconOpen] = useState(false)
  const [reconStmt, setReconStmt] = useState('')
  const [reconPr, setReconPr] = useState('')

  const addDraft = () => {
    const v = form.getFieldsValue()
    if (!v.trans_date || !v.amount) {
      message.warning('日期与金额必填')
      return
    }
    setDraft((d) => [
      ...d,
      {
        trans_date: v.trans_date,
        counterparty: v.counterparty || '',
        amount: v.amount,
        direction: v.direction || 'income',
        summary: v.summary || '',
      },
    ])
    form.resetFields()
  }

  const submitDraft = async () => {
    if (draft.length === 0) {
      message.warning('请先添加流水')
      return
    }
    await apiCreateBankStatements(
      draft.map((d) => ({
        trans_date: d.trans_date,
        counterparty: d.counterparty,
        amount_cent: Math.round(d.amount * 100),
        direction: d.direction,
        summary: d.summary,
      })),
    )
    message.success(`已录入 ${draft.length} 条`)
    setDraft([])
    qc.invalidateQueries({ queryKey: ['bank-statements'] })
    qc.invalidateQueries({ queryKey: ['reconciliation'] })
  }

  const openRecon = (stmtId: string) => {
    setReconStmt(stmtId)
    setReconPr('')
    setReconOpen(true)
  }

  const confirmRecon = async () => {
    if (!reconPr) {
      message.warning('请选择要勾对的回款记录')
      return
    }
    await apiReconcile(reconStmt, reconPr)
    message.success('勾对成功')
    setReconOpen(false)
    qc.invalidateQueries({ queryKey: ['bank-statements'] })
    qc.invalidateQueries({ queryKey: ['reconciliation'] })
  }

  const unrecon = async (stmtId: string) => {
    await apiUnreconcile(stmtId)
    message.success('已取消勾对')
    qc.invalidateQueries({ queryKey: ['bank-statements'] })
    qc.invalidateQueries({ queryKey: ['reconciliation'] })
  }

  const columns = [
    { title: '日期', dataIndex: 'trans_date' },
    { title: '对方户名', dataIndex: 'counterparty' },
    { title: '金额', dataIndex: 'amount_cent', render: (v: number) => yuan(v) },
    {
      title: '方向',
      dataIndex: 'direction',
      render: (v: string) => <Tag color={v === 'income' ? 'green' : 'red'}>{v === 'income' ? '收' : '付'}</Tag>,
    },
    { title: '摘要', dataIndex: 'summary' },
    {
      title: '状态 / 操作',
      render: (_: unknown, r: BankStatement) =>
        r.payment_record_id ? (
          <Space>
            <Tag color="green">已勾对</Tag>
            <Button size="small" onClick={() => unrecon(r.id)}>
              取消
            </Button>
          </Space>
        ) : (
          <Button size="small" type="primary" onClick={() => openRecon(r.id)}>
            勾对
          </Button>
        ),
    },
  ]

  return (
    <div style={{ padding: 24 }}>
      <h2>银企对账</h2>
      <Space direction="vertical" style={{ width: '100%' }} size="small">
        <Alert type="warning" showIcon message={`银行已收企业未收：${recon?.bank_only.length ?? 0} 笔`} />
        <Alert type="info" showIcon message={`企业已收银行未收：${recon?.company_only.length ?? 0} 笔`} />
      </Space>

      <Card title="录入银行流水" style={{ marginTop: 16 }}>
        <Form form={form} layout="inline">
          <Form.Item name="trans_date" label="日期">
            <Input placeholder="2026-07-01" style={{ width: 130 }} />
          </Form.Item>
          <Form.Item name="counterparty" label="对方户名">
            <Input style={{ width: 130 }} />
          </Form.Item>
          <Form.Item name="amount" label="金额(元)">
            <InputNumber min={0} step={0.01} />
          </Form.Item>
          <Form.Item name="direction" label="方向" initialValue="income">
            <Select
              style={{ width: 90 }}
              options={[
                { value: 'income', label: '收' },
                { value: 'expend', label: '付' },
              ]}
            />
          </Form.Item>
          <Form.Item name="summary" label="摘要">
            <Input style={{ width: 130 }} />
          </Form.Item>
          <Button onClick={addDraft}>添加到草稿</Button>
        </Form>
        <Table
          style={{ marginTop: 12 }}
          rowKey={(r) => `${r.trans_date}-${r.counterparty}-${r.amount}`}
          pagination={false}
          dataSource={draft}
          columns={[
            { title: '日期', dataIndex: 'trans_date' },
            { title: '对方户名', dataIndex: 'counterparty' },
            { title: '金额', render: (_: unknown, r: DraftRow) => yuan(Math.round(r.amount * 100)) },
            { title: '方向', dataIndex: 'direction', render: (v: string) => (v === 'income' ? '收' : '付') },
            { title: '摘要', dataIndex: 'summary' },
            {
              title: '操作',
              render: (_: unknown, r: DraftRow) => (
                <Button size="small" danger onClick={() => setDraft((d) => d.filter((x) => x !== r))}>
                  删除
                </Button>
              ),
            },
          ]}
        />
        <Button type="primary" style={{ marginTop: 12 }} disabled={draft.length === 0} onClick={submitDraft}>
          提交录入（{draft.length}）
        </Button>
      </Card>

      <Card title="银行流水" style={{ marginTop: 16 }}>
        <Table rowKey="id" dataSource={stmts ?? []} columns={columns} pagination={{ pageSize: 20 }} />
      </Card>

      <Modal
        title="勾对回款记录"
        open={reconOpen}
        onOk={confirmRecon}
        onCancel={() => setReconOpen(false)}
        okText="勾对"
      >
        <p>选择一条尚未勾对的回款记录：</p>
        <Select
          style={{ width: '100%' }}
          value={reconPr || undefined}
          placeholder="回款记录（企业已收银行未收）"
          onChange={(v) => setReconPr(v)}
          options={(recon?.company_only ?? []).map((c) => ({
            value: c.id,
            label: `${c.trans_date} ${yuan(c.amount_cent)}`,
          }))}
        />
      </Modal>
    </div>
  )
}
