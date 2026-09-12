import { ReactNode } from 'react'
import { Link, useLocation } from 'react-router-dom'
import {
  LayoutDashboard,
  Shield,
  List,
  Cloud,
  Settings,
  Activity
} from 'lucide-react'
import { cn } from '@/lib/utils'

interface AppLayoutProps {
  children: ReactNode
}

const navigation = [
  { name: '仪表盘', href: '/', icon: LayoutDashboard },
  { name: '规则管理', href: '/rules', icon: Shield },
  { name: '实时流量', href: '/traffic', icon: Activity },
  { name: '上游配置', href: '/upstream', icon: Cloud },
  { name: '设置', href: '/settings', icon: Settings },
]

export function AppLayout({ children }: AppLayoutProps) {
  const location = useLocation()

  return (
    <div className="flex h-screen bg-background">
      {/* Sidebar */}
      <aside className="w-64 border-r border-border bg-card flex flex-col">
        <div className="p-6 border-b border-border">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-full bg-primary/10 flex items-center justify-center">
              <Shield className="w-6 h-6 text-primary" />
            </div>
            <div>
              <h1 className="text-lg font-semibold">Redact Gateway</h1>
              <p className="text-xs text-muted-foreground">脱敏网关</p>
            </div>
          </div>
        </div>

        <nav className="flex-1 p-4 space-y-1">
          {navigation.map((item) => {
            const isActive = location.pathname === item.href
            const Icon = item.icon
            return (
              <Link
                key={item.href}
                to={item.href}
                className={cn(
                  'flex items-center gap-3 px-4 py-3 rounded-lg text-sm font-medium transition-colors',
                  isActive
                    ? 'bg-primary text-primary-foreground'
                    : 'text-muted-foreground hover:bg-muted hover:text-foreground'
                )}
              >
                <Icon className="w-5 h-5" />
                {item.name}
              </Link>
            )
          })}
        </nav>

        <div className="p-4 border-t border-border">
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            <span className="status-dot running"></span>
            <span>运行中</span>
            <span className="ml-auto">v0.1.0</span>
          </div>
        </div>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto">
        <div className="container mx-auto p-8 max-w-7xl">
          {children}
        </div>
      </main>
    </div>
  )
}
