import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ConfigProvider, App as AntApp } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import { AuthProvider, useAuth } from './auth/AuthContext'
import MainLayout from './layouts/MainLayout'
import Login from './pages/Login'
import Dashboard from './pages/Dashboard'
import Employees from './pages/Employees'
import Customers from './pages/Customers'
import CustomerDetail from './pages/CustomerDetail'
import Deals from './pages/Deals'
import Contracts from './pages/Contracts'
import Payments from './pages/Payments'
import Approvals from './pages/Approvals'
import Invoices from './pages/Invoices'
import Reports from './pages/Reports'
import CustomerPool from './pages/CustomerPool'
import ImportCustomers from './pages/ImportCustomers'
import Projects from './pages/Projects'
import Audit from './pages/Audit'
import System from './pages/System'
import NotifyChannels from './pages/NotifyChannels'
import DuplicateCustomers from './pages/DuplicateCustomers'
import LostDealAnalysis from './pages/LostDealAnalysis'
import BankReconciliation from './pages/BankReconciliation'
import Recruitment from './pages/Recruitment'
import Hr from './pages/Hr'
import { ReactNode } from 'react'

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, refetchOnWindowFocus: false } },
})

function RequireAuth({ children }: { children: ReactNode }) {
  const { token } = useAuth()
  if (!token) return <Navigate to="/login" replace />
  return <>{children}</>
}

export default function App() {
  return (
    <ConfigProvider locale={zhCN}>
      <AntApp>
        <QueryClientProvider client={queryClient}>
          <AuthProvider>
            <BrowserRouter>
              <Routes>
                <Route path="/login" element={<Login />} />
                <Route
                  path="/"
                  element={
                    <RequireAuth>
                      <MainLayout />
                    </RequireAuth>
                  }
                >
                  <Route index element={<Dashboard />} />
                  <Route path="employees" element={<Employees />} />
                  <Route path="customers" element={<Customers />} />
                  <Route path="customers/:id" element={<CustomerDetail />} />
                  <Route path="deals" element={<Deals />} />
                  <Route path="contracts" element={<Contracts />} />
                  <Route path="payments" element={<Payments />} />
                  <Route path="approvals" element={<Approvals />} />
                  <Route path="invoices" element={<Invoices />} />
                  <Route path="reports" element={<Reports />} />
                  <Route path="customer-pool" element={<CustomerPool />} />
                  <Route path="import-customers" element={<ImportCustomers />} />
                  <Route path="projects" element={<Projects />} />
                  <Route path="audit" element={<Audit />} />
                  <Route path="system" element={<System />} />
                  <Route path="notify-channels" element={<NotifyChannels />} />
                  <Route path="duplicates" element={<DuplicateCustomers />} />
                  <Route path="lost-analysis" element={<LostDealAnalysis />} />
                  <Route path="reconciliation" element={<BankReconciliation />} />
                  <Route path="recruitment" element={<Recruitment />} />
                  <Route path="hr" element={<Hr />} />
                </Route>
                <Route path="*" element={<Navigate to="/" replace />} />
              </Routes>
            </BrowserRouter>
          </AuthProvider>
        </QueryClientProvider>
      </AntApp>
    </ConfigProvider>
  )
}
