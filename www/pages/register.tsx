import { Providers } from '@/components/providers'
import { RegisterComponent } from '@/components/register'
import { useTranslation } from 'react-i18next'
import Link from 'next/link'
import { AuthShell } from '@/components/auth-shell'

export default function RegisterPage() {
  const { t } = useTranslation()

  return (
    <Providers>
      <AuthShell
        title={t('auth.registerTitle')}
        subtitle={t('auth.inputCredentials')}
        footer={<>{t('auth.haveAccount')}{' '}<Link className="font-medium text-primary hover:underline" href="/login">{t('auth.login')}</Link></>}
      >
        <RegisterComponent />
      </AuthShell>
    </Providers>
  )
}
