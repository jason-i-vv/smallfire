<template>
  <div class="settings">
    <el-row :gutter="24">
      <el-col :span="12">
        <el-card>
          <template #header>
            <span>{{ t('settings.password') }}</span>
          </template>
          <el-form ref="pwdFormRef" :model="pwdForm" :rules="pwdRules" label-width="100px">
            <el-form-item :label="t('settings.currentPassword')" prop="oldPassword">
              <el-input v-model="pwdForm.oldPassword" type="password" show-password :placeholder="t('settings.enterCurrentPassword')" />
            </el-form-item>
            <el-form-item :label="t('settings.newPassword')" prop="newPassword">
              <el-input v-model="pwdForm.newPassword" type="password" show-password :placeholder="t('settings.passwordLength')" />
            </el-form-item>
            <el-form-item :label="t('settings.confirmPassword')" prop="confirmPassword">
              <el-input v-model="pwdForm.confirmPassword" type="password" show-password :placeholder="t('settings.confirmNewPasswordPlaceholder')" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" :loading="pwdLoading" @click="handleChangePassword">{{ t('settings.submit') }}</el-button>
            </el-form-item>
          </el-form>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { ElMessage } from 'element-plus'

const { t } = useI18n()
const authStore = useAuthStore()

const pwdFormRef = ref(null)
const pwdLoading = ref(false)
const pwdForm = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: ''
})

const validateConfirmPassword = (rule, value, callback) => {
  if (value !== pwdForm.newPassword) {
    callback(new Error(t('auth.register.passwordMismatch')))
  } else {
    callback()
  }
}

const pwdRules = {
  oldPassword: [{ required: true, message: t('settings.enterCurrentPassword') }],
  newPassword: [
    { required: true, message: t('settings.enterNewPassword') },
    { min: 6, max: 64, message: t('settings.passwordLength') }
  ],
  confirmPassword: [
    { required: true, message: t('settings.confirmNewPasswordPlaceholder') },
    { validator: validateConfirmPassword, trigger: 'blur' }
  ]
}

const handleChangePassword = async () => {
  try {
    await pwdFormRef.value.validate()
    pwdLoading.value = true
    await authStore.changePassword({
      oldPassword: pwdForm.oldPassword,
      newPassword: pwdForm.newPassword
    })
    ElMessage.success(t('settings.passwordSuccess'))
    authStore.logout()
  } catch (error) {
    // 错误已在拦截器中提示
  } finally {
    pwdLoading.value = false
  }
}
</script>

<style lang="scss" scoped>
@import '@/assets/styles/variables.scss';
</style>
