import api from './index'

export const authApi = {
  register: (data) => api.post('/auth/register', data),
  login: (data) => api.post('/auth/login', data),
  getMe: () => api.get('/auth/me'),
  changePassword: (data) => api.put('/auth/password', data),
  listUsers: () => api.get('/users'),
  updateUserStatus: (id, data) => api.put(`/users/${id}/status`, data),
  resetPassword: (id, data) => api.put(`/users/${id}/password`, data)
}
