import { useState } from 'react'
import {
  Upload, Button, Table, Typography, Alert, Space, App, Tag, Progress,
} from 'antd'
import { InboxOutlined, DownloadOutlined, UploadOutlined } from '@ant-design/icons'
import type { UploadProps } from 'antd'
import { apiDownloadCustomerTemplate, apiImportCustomers, ImportResult } from '../api'

export default function ImportCustomers() {
  const { message } = App.useApp()
  const [file, setFile] = useState<File | null>(null)
  const [uploading, setUploading] = useState(false)
  const [result, setResult] = useState<ImportResult | null>(null)

  const beforeUpload: UploadProps['beforeUpload'] = (f) => {
    const ok = f.name.toLowerCase().endsWith('.xlsx')
    if (!ok) {
      message.error('仅支持 .xlsx 文件')
      return Upload.LIST_IGNORE
    }
    setFile(f as File)
    return false // 阻止自动上传，由按钮触发
  }

  const downloadTemplate = async () => {
    try {
      const blob = await apiDownloadCustomerTemplate()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = '客户导入模板.xlsx'
      a.click()
      URL.revokeObjectURL(url)
    } catch {
      message.error('模板下载失败')
    }
  }

  const startImport = async () => {
    if (!file) {
      message.warning('请先选择 Excel 文件')
      return
    }
    setUploading(true)
    setResult(null)
    try {
      const res = await apiImportCustomers(file)
      setResult(res)
      if (res.errors.length === 0) {
        message.success(`导入完成：新建客户 ${res.created_customers} 个、联系人 ${res.created_contacts} 个`)
      } else {
        message.warning(`导入完成，但有 ${res.errors.length} 行存在问题，请查看下方明细`)
      }
    } catch (e: unknown) {
      const msg = (e as { message?: string })?.message
      message.error(msg || '导入失败，请检查文件格式')
    } finally {
      setUploading(false)
    }
  }

  const pct =
    result && result.total > 0
      ? Math.round(((result.created_customers + result.skipped) / result.total) * 100)
      : 0

  return (
    <div>
      <Space style={{ marginBottom: 16, width: '100%', justifyContent: 'space-between' }} wrap>
        <Typography.Title level={4} style={{ margin: 0 }}>
          <InboxOutlined /> 客户 Excel 导入
        </Typography.Title>
        <Button icon={<DownloadOutlined />} onClick={downloadTemplate}>
          下载导入模板
        </Button>
      </Space>

      <Alert
        style={{ marginBottom: 16 }}
        type="info"
        showIcon
        message="批量录入存量客户与联系人"
        description="下载模板后按列填写，第一行表头不要修改。系统按客户名称去重，已存在的客户将跳过；负责人留空则归属导入人。"
      />

      <Upload.Dragger
        accept=".xlsx"
        beforeUpload={beforeUpload}
        maxCount={1}
        fileList={file ? [{ uid: '-1', name: file.name, status: 'done' }] : []}
        onRemove={() => setFile(null)}
        style={{ marginBottom: 16 }}
      >
        <p className="ant-upload-drag-icon"><InboxOutlined /></p>
        <p className="ant-upload-text">点击或拖拽 .xlsx 文件到此区域</p>
        <p className="ant-upload-hint">仅支持 .xlsx（不支持旧版 .xls），单文件上限 10MB</p>
      </Upload.Dragger>

      <Button
        type="primary"
        icon={<UploadOutlined />}
        loading={uploading}
        disabled={!file}
        onClick={startImport}
      >
        开始导入
      </Button>

      {result && (
        <div style={{ marginTop: 24 }}>
          <Space size="large" wrap style={{ marginBottom: 12 }}>
            <Stat label="有效行" value={result.total} />
            <Stat label="新建客户" value={result.created_customers} color="green" />
            <Stat label="新建联系人" value={result.created_contacts} color="blue" />
            <Stat label="跳过(重名)" value={result.skipped} color="orange" />
            <Stat label="失败行" value={result.errors.length} color={result.errors.length ? 'red' : 'default'} />
          </Space>

          {result.total > 0 && <Progress percent={pct} status={result.errors.length ? 'exception' : 'success'} />}

          {result.errors.length > 0 && (
            <Table<{ row: number; message: string }>
              size="small"
              style={{ marginTop: 12 }}
              rowKey="row"
              dataSource={result.errors}
              pagination={result.errors.length > 10 ? { pageSize: 10 } : false}
              columns={[
                { title: '行号', dataIndex: 'row', width: 80, render: (v) => <Tag>{v}</Tag> },
                { title: '原因', dataIndex: 'message' },
              ]}
            />
          )}

          {result.errors.length === 0 && (
            <Alert style={{ marginTop: 12 }} type="success" showIcon message="全部导入成功，没有错误行。" />
          )}
        </div>
      )}
    </div>
  )
}

function Stat({ label, value, color = 'default' }: { label: string; value: number; color?: string }) {
  const colorMap: Record<string, string> = {
    green: '#52c41a', blue: '#1677ff', orange: '#fa8c16', red: '#ff4d4f', default: '#333',
  }
  return (
    <div style={{ textAlign: 'center' }}>
      <div style={{ fontSize: 24, fontWeight: 600, color: colorMap[color] }}>{value}</div>
      <div style={{ color: '#888', fontSize: 12 }}>{label}</div>
    </div>
  )
}
