import { createContext, useContext, useState, ReactNode } from 'react'
import { getToken, setToken, clearToken } from '../api/client'

interface AuthState {
  token: string | null
  mustChangePwd: boolean
  login: (token: string, mustChangePwd: boolean) => void
  logout: () => void
  clearMustChange: () => void
}

const AuthCtx = createContext<AuthState>(null as unknown as AuthState)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setTok] = useState<string | null>(getToken())
  const [mustChangePwd, setMustChangePwd] = useState(false)

  const login = (t: string, must: boolean) => {
    setToken(t)
    setTok(t)
    setMustChangePwd(must)
  }
  const logout = () => {
    clearToken()
    setTok(null)
  }
  const clearMustChange = () => setMustChangePwd(false)

  return (
    <AuthCtx.Provider value={{ token, mustChangePwd, login, logout, clearMustChange }}>
      {children}
    </AuthCtx.Provider>
  )
}

export const useAuth = () => useContext(AuthCtx)
