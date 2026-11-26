"use client";

import React, { useEffect, useState } from 'react';
import { Lock, Loader2, ShieldCheck } from 'lucide-react';

/**
 * 访问密码网关
 * - 后端未设置密码时直接放行
 * - 已设置密码时，校验通过前不渲染任何功能
 */

type GateState = 'checking' | 'locked' | 'unlocked';

// 拼接认证接口地址：生产环境同源，开发环境走 Next.js 代理
function authUrl(path: string): string {
  const backend = process.env.NEXT_PUBLIC_BACKEND_URL;
  if (backend) {
    return `${backend.replace(/\/$/, '')}/api${path}`;
  }
  if (typeof window !== 'undefined') {
    const { hostname, port, origin } = window.location;
    const isNextDev =
      hostname === 'localhost' && (port === '3000' || port === '3001');
    if (isNextDev) {
      // next.config.js 中的 rewrite: /api/proxy/:path* -> 后端 /api/:path*
      return `/api/proxy${path}`;
    }
    return `${origin}/api${path}`;
  }
  return `/api${path}`;
}

interface AuthGateProps {
  children: React.ReactNode;
}

export default function AuthGate({ children }: AuthGateProps) {
  const [state, setState] = useState<GateState>('checking');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);

  // 初始化：查询是否需要密码、是否已登录
  useEffect(() => {
    let cancelled = false;

    const check = async () => {
      try {
        const res = await fetch(authUrl('/auth/status'), {
          credentials: 'include',
          cache: 'no-store',
        });
        if (!res.ok) {
          throw new Error(`status ${res.status}`);
        }
        const data = await res.json();
        if (cancelled) return;

        if (!data.enabled || data.authed) {
          setState('unlocked');
        } else {
          setState('locked');
        }
      } catch {
        // 接口不可用时不阻塞使用（后端依然会拦截未授权的请求）
        if (!cancelled) setState('unlocked');
      }
    };

    check();
    return () => {
      cancelled = true;
    };
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (submitting) return;
    if (!password.trim()) {
      setError('请输入访问密码');
      return;
    }

    setSubmitting(true);
    setError('');
    try {
      const res = await fetch(authUrl('/auth/login'), {
        method: 'POST',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password }),
      });
      const data = await res.json().catch(() => ({}));

      if (res.ok && data.success) {
        setState('unlocked');
      } else {
        setError(data.message || '密码错误，请重试');
        setPassword('');
      }
    } catch {
      setError('无法连接服务器，请稍后重试');
    } finally {
      setSubmitting(false);
    }
  };

  if (state === 'checking') {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-slate-50 to-blue-50">
        <div className="flex items-center gap-2 text-slate-500">
          <Loader2 className="w-4 h-4 animate-spin" />
          <span className="text-sm">加载中...</span>
        </div>
      </div>
    );
  }

  if (state === 'locked') {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-slate-50 to-blue-50 px-4">
        <div className="w-full max-w-sm bg-white/90 backdrop-blur-sm rounded-2xl shadow-xl border border-slate-200 p-6 sm:p-8">
          <div className="text-center mb-6">
            <div className="w-12 h-12 mx-auto mb-4 rounded-full bg-blue-50 border border-blue-100 flex items-center justify-center">
              <Lock className="w-5 h-5 text-blue-600" />
            </div>
            <h1 className="text-xl font-bold bg-gradient-to-r from-blue-600 via-purple-600 to-indigo-600 bg-clip-text text-transparent">
              文件快传
            </h1>
            <p className="mt-1 text-xs text-slate-500">请输入访问密码后使用</p>
          </div>

          <form onSubmit={handleSubmit} className="space-y-3">
            <input
              type="password"
              value={password}
              onChange={(e) => {
                setPassword(e.target.value);
                setError('');
              }}
              autoFocus
              autoComplete="current-password"
              placeholder="访问密码"
              className="w-full px-3 py-2.5 text-sm rounded-lg border border-slate-300 bg-white text-slate-800 placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500/40 focus:border-blue-400 transition"
            />

            {error && <p className="text-xs text-red-500">{error}</p>}

            <button
              type="submit"
              disabled={submitting}
              className="w-full flex items-center justify-center gap-2 px-3 py-2.5 text-sm font-medium text-white rounded-lg bg-blue-500 hover:bg-blue-600 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
            >
              {submitting ? (
                <Loader2 className="w-4 h-4 animate-spin" />
              ) : (
                <ShieldCheck className="w-4 h-4" />
              )}
              {submitting ? '验证中...' : '进入'}
            </button>
          </form>
        </div>
      </div>
    );
  }

  return <>{children}</>;
}
