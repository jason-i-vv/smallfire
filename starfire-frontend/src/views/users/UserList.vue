<template>
  <div class="user-list">
    <el-card>
      <template #header>
        <div class="card-header">
          <span>用户管理</span>
          <el-button type="primary" :icon="Refresh" @click="loadUsers">刷新</el-button>
        </div>
      </template>

      <el-table :data="users" v-loading="loading" stripe>
        <el-table-column prop="id" label="ID" width="60" />
        <el-table-column prop="username" label="用户名" width="140" />
        <el-table-column prop="nickname" label="昵称" width="120" />
        <el-table-column prop="role" label="角色" width="100">
          <template #default="{ row }">
            <el-tag :type="row.role === 'admin' ? 'danger' : 'info'" size="small">
              {{ row.role === 'admin' ? '管理员' : '普通用户' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="is_active" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.is_active ? 'success' : 'danger'" size="small">
              {{ row.is_active ? '正常' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="last_login_at" label="最后登录" width="180">
          <template #default="{ row }">
            {{ row.last_login_at || '--' }}
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="注册时间" width="180" />
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button
              :type="row.is_active ? 'warning' : 'success'"
              size="small"
              @click="handleToggleStatus(row)"
            >
              {{ row.is_active ? '禁用' : '启用' }}
            </el-button>
            <el-button
              type="primary"
              size="small"
              @click="handleResetPassword(row)"
            >
              重置密码
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="resetDialogVisible" title="重置密码" width="400px">
      <el-form :model="resetForm">
        <el-form-item label="新密码">
          <el-input v-model="resetForm.newPassword" type="password" show-password placeholder="请输入新密码" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="resetDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="resetLoading" @click="confirmResetPassword">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { authApi } from '@/api/auth'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'

const users = ref([])
const loading = ref(false)
const resetDialogVisible = ref(false)
const resetLoading = ref(false)
const resetTargetUser = ref(null)
const resetForm = reactive({
  newPassword: ''
})

const loadUsers = async () => {
  loading.value = true
  try {
    const res = await authApi.listUsers()
    users.value = res.data.users || []
  } catch (error) {
    // 错误已在拦截器中提示
  } finally {
    loading.value = false
  }
}

const handleToggleStatus = async (row) => {
  const action = row.is_active ? '禁用' : '启用'
  try {
    await ElMessageBox.confirm(`确定要${action}用户 "${row.username}" 吗？`, '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
    await authApi.updateUserStatus(row.id, { is_active: !row.is_active })
    ElMessage.success(`用户已${action}`)
    loadUsers()
  } catch (error) {
    // 用户取消或请求失败
  }
}

const handleResetPassword = (row) => {
  resetTargetUser.value = row
  resetForm.newPassword = ''
  resetDialogVisible.value = true
}

const confirmResetPassword = async () => {
  if (!resetForm.newPassword || resetForm.newPassword.length < 6) {
    ElMessage.warning('密码长度不能少于6位')
    return
  }
  resetLoading.value = true
  try {
    await authApi.resetPassword(resetTargetUser.value.id, {
      new_password: resetForm.newPassword
    })
    ElMessage.success('密码已重置')
    resetDialogVisible.value = false
  } catch (error) {
    // 错误已在拦截器中提示
  } finally {
    resetLoading.value = false
  }
}

onMounted(() => {
  loadUsers()
})
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';

.user-list {
  .card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;

    span {
      font-size: 16px;
      font-weight: 600;
      color: $text-primary;
    }
  }
}
</style>
