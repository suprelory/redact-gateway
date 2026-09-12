/**
 * API 客户端
 */

const API_BASE = '/api';

export interface ApiResponse<T = unknown> {
  success: boolean;
  data?: T;
  error?: {
    code: string;
    message: string;
  };
}

class ApiClient {
  private baseUrl: string;
  private token: string | null = null;

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl;
  }

  setToken(token: string) {
    this.token = token;
    if (typeof window !== 'undefined') {
      sessionStorage.setItem('admin_token', token);
    }
  }

  getToken(): string | null {
    if (this.token) return this.token;
    if (typeof window !== 'undefined') {
      return sessionStorage.getItem('admin_token');
    }
    return null;
  }

  clearToken() {
    this.token = null;
    if (typeof window !== 'undefined') {
      sessionStorage.removeItem('admin_token');
    }
  }

  private async request<T>(
    path: string,
    options: RequestInit = {}
  ): Promise<ApiResponse<T>> {
    const token = this.getToken();
    const headers: HeadersInit = {
      'Content-Type': 'application/json',
      ...(token && { Authorization: `Bearer ${token}` }),
      ...options.headers,
    };

    const response = await fetch(`${this.baseUrl}${path}`, {
      ...options,
      headers,
    });

    if (!response.ok) {
      if (response.status === 401) {
        this.clearToken();
        throw new Error('未授权，请重新登录');
      }
      const error = await response.json().catch(() => ({ message: '请求失败' }));
      throw new Error(error.message || `HTTP ${response.status}`);
    }

    return response.json();
  }

  // ============= 状态 API =============
  async getStatus() {
    return this.request('/status');
  }

  async getStats(period?: string) {
    const query = period ? `?period=${period}` : '';
    return this.request(`/stats${query}`);
  }

  // ============= 规则 API =============
  async getRules() {
    return this.request('/rules');
  }

  async createRule(rule: unknown) {
    return this.request('/rules', {
      method: 'POST',
      body: JSON.stringify(rule),
    });
  }

  async updateRule(id: string, rule: unknown) {
    return this.request(`/rules/${id}`, {
      method: 'PUT',
      body: JSON.stringify(rule),
    });
  }

  async deleteRule(id: string) {
    return this.request(`/rules/${id}`, {
      method: 'DELETE',
    });
  }

  async testRule(text: string, ruleIds?: string[]) {
    return this.request('/rules/test', {
      method: 'POST',
      body: JSON.stringify({ text, ruleIds }),
    });
  }

  // ============= 流量日志 API =============
  async getLogs(params?: { page?: number; pageSize?: number; filter?: string }) {
    const query = new URLSearchParams(params as Record<string, string>).toString();
    return this.request(`/logs${query ? `?${query}` : ''}`);
  }

  async getLogDetail(id: string) {
    return this.request(`/logs/${id}`);
  }

  // ============= 上游配置 API =============
  async getUpstreams() {
    return this.request('/upstreams');
  }

  async updateUpstream(id: string, config: unknown) {
    return this.request(`/upstreams/${id}`, {
      method: 'PUT',
      body: JSON.stringify(config),
    });
  }

  // ============= 系统配置 API =============
  async getConfig() {
    return this.request('/config');
  }

  async updateConfig(config: unknown) {
    return this.request('/config', {
      method: 'PUT',
      body: JSON.stringify(config),
    });
  }
}

export const api = new ApiClient(API_BASE);
