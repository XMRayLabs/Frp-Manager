import { Providers } from '@/components/providers'
import { LoginComponent } from '@/components/login'
import { useTranslation } from 'react-i18next'
import Link from 'next/link'
import { AuthShell } from '@/components/auth-shell'

export default function LoginPage() {
  const { t } = useTranslation()

  return (
    <Providers>
      <AuthShell
        title={t('auth.loginTitle')}
        subtitle={t('auth.inputCredentials')}
        footer={<>{t('auth.noAccount')}{' '}<Link className="font-medium text-primary hover:underline" href="/register">{t('auth.register')}</Link></>}
      >
        <LoginComponent />
      </AuthShell>
    </Providers>
  )
}
