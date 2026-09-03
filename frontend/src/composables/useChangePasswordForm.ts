import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuth } from '@/stores/auth'
import { ApiError } from '@/api/client'

// useChangePasswordForm is the change-password form's shared state/validation/
// submit logic — used by both the forced first-login screen
// (views/ChangePassword.vue) and the "any time" Account Security dialog
// reachable from the nav rail's avatar menu and Settings → Team.
// Both wrap the exact same three-field form and POST /auth/password call;
// only the surrounding chrome (a full page vs. a dialog) and what happens
// on success (navigate vs. close) differ, so those stay with each caller via onSuccess.
export function useChangePasswordForm(onSuccess: () => void) {
  const auth = useAuth()
  const { t } = useI18n()

  const currentPassword = ref('')
  const newPassword = ref('')
  const confirmPassword = ref('')
  const error = ref('')
  const busy = ref(false)

  function reset() {
    currentPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
    error.value = ''
    busy.value = false
  }

  async function submit() {
    error.value = ''
    // MaskedSecretInput's root is a wrapping <div> (for the show/hide
    // toggle), so native HTML `required` on it would be a no-op — validate
    // presence explicitly instead of relying on form validation.
    if (!currentPassword.value || !newPassword.value || !confirmPassword.value) {
      error.value = t('changePassword.errors.generic')
      return
    }
    if (newPassword.value !== confirmPassword.value) {
      error.value = t('changePassword.errors.mismatch')
      return
    }
    if (newPassword.value === currentPassword.value) {
      error.value = t('changePassword.errors.sameAsCurrent')
      return
    }
    busy.value = true
    try {
      await auth.changePassword(currentPassword.value, newPassword.value)
      onSuccess()
    } catch (e) {
      if (e instanceof ApiError && e.status === 429) {
        error.value = t('changePassword.errors.rateLimited')
      } else if (e instanceof ApiError && e.errcode === 'UNAUTHORIZED') {
        error.value = t('changePassword.errors.wrongCurrent')
      } else {
        error.value = t('changePassword.errors.generic')
      }
    } finally {
      busy.value = false
    }
  }

  return { currentPassword, newPassword, confirmPassword, error, busy, submit, reset }
}
